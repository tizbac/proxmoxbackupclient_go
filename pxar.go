package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"fmt"
//	"io/ioutil"
	"path/filepath"
	"github.com/dchest/siphash"
)



const (
	PXAR_ENTRY                 uint64 = 0xd5956474e588acef
	PXAR_ENTRY_V1              uint64 = 0x11da850a1c1cceff
	PXAR_FILENAME              uint64 = 0x16701121063917b3
	PXAR_SYMLINK               uint64 = 0x27f971e7dbf5dc5f
	PXAR_DEVICE                uint64 = 0x9fc9e906586d5ce9
	PXAR_XATTR                 uint64 = 0x0dab0229b57dcd03
	PXAR_ACL_USER              uint64 = 0x2ce8540a457d55b8
	PXAR_ACL_GROUP             uint64 = 0x136e3eceb04c03ab
	PXAR_ACL_GROUP_OBJ         uint64 = 0x10868031e9582876
	PXAR_ACL_DEFAULT           uint64 = 0xbbbb13415a6896f5
	PXAR_ACL_DEFAULT_USER      uint64 = 0xc89357b40532cd1f
	PXAR_ACL_DEFAULT_GROUP     uint64 = 0xf90a8a5816038ffe
	PXAR_FCAPS                 uint64 = 0x2da9dd9db5f7fb67
	PXAR_QUOTA_PROJID          uint64 = 0xe07540e82f7d1cbb
	PXAR_HARDLINK              uint64 = 0x51269c8422bd7275
	PXAR_PAYLOAD               uint64 = 0x28147a1b0b7c1a25
	PXAR_GOODBYE               uint64 = 0x2fec4fa642d5731d
	PXAR_GOODBYE_TAIL_MARKER   uint64 = 0xef5eed5b753e1555
)

var catalog_magic = []byte {145, 253, 96, 249, 196, 103, 88, 213}

const (
    IFMT  uint64 = 0o0170000
    IFSOCK uint64 = 0o0140000
    IFLNK  uint64 = 0o0120000
    IFREG  uint64 = 0o0100000
    IFBLK  uint64 = 0o0060000
    IFDIR  uint64 = 0o0040000
    IFCHR  uint64 = 0o0020000
    IFIFO  uint64 = 0o0010000

    ISUID  uint64 = 0o0004000
    ISGID  uint64 = 0o0002000
    ISVTX  uint64 = 0o0001000
)
type MTime struct {
	secs uint64
	nanos uint32 
	padding uint32
	
}
type PXARFileEntry struct {
	hdr uint64
	len uint64
	mode uint64
	flags uint64 
	uid uint32 
	gid uint32 
	mtime MTime
}

type PXARFilenameEntry struct {
	hdr uint64
	len uint64

}

type GoodByeItem struct {
	hash uint64 
	offset uint64 
	len uint64
}

type PXAROutCB func([]byte)

type PXARArchive struct {
	//Create(filename string, writeCB PXAROutCB)
	//AddFile(filename string)
	//AddDirectory(dirname string)
	writeCB PXAROutCB
	catalogWriteCB PXAROutCB
	buffer bytes.Buffer
	pos uint64

	catalog_pos uint64

}

func (a *PXARArchive) Flush() {
	b := make([]byte, 64*1024);
	count, _ := a.buffer.Read(b)
	a.writeCB(b[:count])
	a.pos = a.pos + uint64(count)
	//fmt.Printf("Flush %d bytes\n", count)
}

func (a *PXARArchive) Create() {
	a.pos = 0
	a.catalog_pos = 8
}

type CatalogDir struct {
	Pos uint64 //Punta alla tabella successiva, quindi il padre deve essere sempre scritto DOPO il figlio
	Name string
}

type CatalogFile struct {
	Name string
	MTime uint64 
	Size uint64
}


func append_u64_7bit( a []byte, v uint64) []byte {
	x := a 
	for {
		if v < 128 {
			x = append(x, byte(v & 0x7f) ) 
			break
		}
		x = append(x, byte(v & 0x7f) | byte(0x80) ) 
		v = v >> 7
	}
	return x
}

func (a *PXARArchive) WriteDir(path string, dirname string, toplevel bool) CatalogDir {
	//fmt.Printf("Write dir %s at %d\n", path, a.pos)
	files, err := os.ReadDir(path)
	if err != nil {
		return CatalogDir{}
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Failed to stat %s\n", path)
		return CatalogDir{}
	}

	if (!toplevel) {
		fname_entry := &PXARFilenameEntry{
			hdr: PXAR_FILENAME,
			len: uint64(16)+uint64(len(dirname))+1,
		}
	
		binary.Write(&a.buffer, binary.LittleEndian, fname_entry)
	
		a.buffer.WriteString(dirname)
		a.buffer.WriteByte(0x00)
	} else {
		if a.catalogWriteCB != nil {
			a.catalogWriteCB(catalog_magic)
			a.catalog_pos = 8
		}
	}

	entry := &PXARFileEntry{
		hdr: PXAR_ENTRY,
		len: 56,
		mode: IFDIR | 0o777,
		flags: 0,
		uid: 1000,
		gid: 1000,
		mtime: MTime{
			secs: uint64(fileInfo.ModTime().Unix()),
			nanos: 0,
			padding: 0,
		},
	}
	binary.Write(&a.buffer, binary.LittleEndian, entry)

	posmap := make(map[string]uint64)
	lenmap := make(map[string]uint64)

	catalog_files := make([]CatalogFile,0)
	catalog_dirs := make([]CatalogDir,0 )

	for _ , file := range files {
		if file.IsDir() {
			posmap[file.Name()] = a.pos
			D := a.WriteDir(filepath.Join(path, file.Name()), file.Name(), false)
			catalog_dirs = append(catalog_dirs, D)
			lenmap[file.Name()] = a.pos-posmap[file.Name()]
		}else{
			posmap[file.Name()] = a.pos
			F := a.WriteFile(filepath.Join(path, file.Name()), file.Name())
			catalog_files = append(catalog_files, F)
			lenmap[file.Name()] = a.pos-posmap[file.Name()]
		}
	}

	//Here we can write AFTER the recursion so leaves get written first 
	oldpos := a.catalog_pos
	tabledata := make([]byte, 0)
	tabledata = append_u64_7bit(tabledata, uint64(len(catalog_files)+len(catalog_dirs)))
	for _, d := range catalog_dirs {
		tabledata = append(tabledata, 'd')
		tabledata = append_u64_7bit(tabledata, uint64(len(d.Name)))
		tabledata = append(tabledata, []byte(d.Name)...)
		tabledata = append_u64_7bit(tabledata, d.Pos)
	}

	for _, f := range catalog_files {
		tabledata = append(tabledata, 'f')
		tabledata = append_u64_7bit(tabledata, uint64(len(f.Name)))
		tabledata = append(tabledata, []byte(f.Name)...)
		tabledata = append_u64_7bit(tabledata, f.Size)
		tabledata = append_u64_7bit(tabledata, f.MTime)
	}
	
	catalog_outdata := make([]byte, 0)
	catalog_outdata = append_u64_7bit(catalog_outdata, uint64(len(tabledata)))
	catalog_outdata = append(catalog_outdata, tabledata...)

	if a.catalogWriteCB != nil {
		a.catalogWriteCB(catalog_outdata)
		
	}

	a.catalog_pos += uint64(len(catalog_outdata))


	binary.Write(&a.buffer, binary.LittleEndian, PXAR_GOODBYE)
	goodbyelen := uint64(16 + 24*(len(posmap)+1))
	binary.Write(&a.buffer, binary.LittleEndian, goodbyelen)

	for filename, pos := range posmap {
		gi := &GoodByeItem{
			offset: a.pos-pos,
			len: lenmap[filename],
			hash: siphash.Hash(0x83ac3f1cfbb450db, 0xaa4f1b6879369fbd, []byte(filename)),
		}
		binary.Write(&a.buffer, binary.LittleEndian, gi)
	}

	gi := &GoodByeItem{
		offset: a.pos,
		len: goodbyelen,
		hash: 0xef5eed5b753e1555,
	}

	binary.Write(&a.buffer, binary.LittleEndian, gi)

	a.Flush()

	if ( toplevel ) {
		//We write special pointer to root dir here 
		ptr := make([]byte,0)
		ptr = binary.LittleEndian.AppendUint64(ptr, oldpos)
		if a.catalogWriteCB != nil {
			a.catalogWriteCB(ptr)
		}
	}

	return CatalogDir{
		Name: dirname,
		Pos: oldpos,
	}
}


//Prima deve essere scritta una directory!!
func (a *PXARArchive) WriteFile(path string, basename string) CatalogFile {
	//fmt.Printf("Write file %s at %d\n", path, a.pos)
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Failed to stat %s\n", path)
		return CatalogFile{}
	}

	file, err := os.Open(path)

	if err != nil {
		fmt.Printf("Failed to open %s\n", path)
		return CatalogFile{}
	}

	defer file.Close()


	fname_entry := &PXARFilenameEntry{
		hdr: PXAR_FILENAME,
		len: uint64(16)+uint64(len(basename))+1,
	}

	binary.Write(&a.buffer, binary.LittleEndian, fname_entry)

	a.buffer.WriteString(basename)
	a.buffer.WriteByte(0x00)

	entry := &PXARFileEntry{
		hdr: PXAR_ENTRY,
		len: 56,
		mode: IFREG | 0o777,
		flags: 0,
		uid: 1000,
		gid: 1000,
		mtime: MTime{
			secs: uint64(fileInfo.ModTime().Unix()),
			nanos: 0,
			padding: 0,
		},
	}
	binary.Write(&a.buffer, binary.LittleEndian, entry)

	binary.Write(&a.buffer, binary.LittleEndian, PXAR_PAYLOAD)
	filesize := uint64(fileInfo.Size())+16 //Dimensione del file + Header

	binary.Write(&a.buffer, binary.LittleEndian, filesize)

	a.Flush()


	readbuffer := make([]byte, 1024*64)
	
	for {
		nread, err := file.Read(readbuffer)
		if nread <= 0 {
			break
		}
		if err != nil {
			panic(err.Error())
		}
		a.buffer.Write(readbuffer[:nread])
		a.Flush()
	}
	




	a.Flush()

	return CatalogFile{
		Name: basename,
		MTime: uint64(fileInfo.ModTime().Unix()),
		Size: uint64(fileInfo.Size()),
	}
}