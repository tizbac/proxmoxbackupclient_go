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
	buffer bytes.Buffer
	pos uint64
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
	
}

func (a *PXARArchive) WriteDir(path string, dirname string, toplevel bool){
	//fmt.Printf("Write dir %s at %d\n", path, a.pos)
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Failed to stat %s\n", path)
		return
	}

	if (!toplevel) {
		fname_entry := &PXARFilenameEntry{
			hdr: PXAR_FILENAME,
			len: uint64(16)+uint64(len(dirname))+1,
		}
	
		binary.Write(&a.buffer, binary.LittleEndian, fname_entry)
	
		a.buffer.WriteString(dirname)
		a.buffer.WriteByte(0x00)
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
	for _ , file := range files {
		if file.IsDir() {
			posmap[file.Name()] = a.pos
			a.WriteDir(filepath.Join(path, file.Name()), file.Name(), false)
			lenmap[file.Name()] = a.pos-posmap[file.Name()]
		}else{
			posmap[file.Name()] = a.pos
			a.WriteFile(filepath.Join(path, file.Name()), file.Name())
			lenmap[file.Name()] = a.pos-posmap[file.Name()]
		}
	}

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

}


//Prima deve essere scritta una directory!!
func (a *PXARArchive) WriteFile(path string, basename string) {
	//fmt.Printf("Write file %s at %d\n", path, a.pos)
	fileInfo, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Failed to stat %s\n", path)
		return
	}

	file, err := os.Open(path)

	if err != nil {
		fmt.Printf("Failed to open %s\n", path)
		return
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
}

func main() {
	client := &PBSClient{
		baseurl: os.Args[1],
		certfingerprint: os.Args[2],//"ea:7d:06:f9:87:73:a4:72:d0:e8:05:a4:b3:3d:95:d7:0a:26:dd:6d:5c:ca:e6:99:83:e4:11:3b:5f:10:f4:4b",
		authid: os.Args[3],
		secret: os.Args[4],
		datastore: os.Args[5],
	}

	client.Connect()
	client.CreateDynamicIndex("backup.pxar.didx")
	client.CloseDynamicIndex()
	client.Finish()
	return

    A := &PXARArchive{}
	C := &Chunker{}
	C.New(1024*1024*4)
	f, _ := os.Create("test.pxar")
	defer f.Close()


	current_chunk := make([]byte,0)

	A.writeCB = func(b []byte) {
		chunkpos := C.Scan(b)
		

		if ( chunkpos > 0 ) {
			current_chunk = append(current_chunk, b[:chunkpos]...)
			fmt.Printf("New chunk %d bytes\n",len(current_chunk))



			current_chunk = b[chunkpos:]
		}else{
			current_chunk = append(current_chunk, b...)
		}
		f.Write(b)
	}
	A.WriteDir("/home/tiziano/TA","",true)

}