module directorybackup

go 1.25

require (
	clientcommon v0.0.0
	github.com/cornelk/hashmap v1.0.8
	pbscommon v0.0.0
	snapshot v0.0.0
)

require (
	github.com/alessio/shellescape v1.4.2 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/rodolfoag/gow32 v0.0.0-20230512144032-1e896a3c51aa // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	golang.org/x/exp v0.0.0-20240531132922-fd00a4e0eefc // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/text v0.15.0 // indirect
)

require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/tawesoft/golib/v2 v2.16.0
	golang.org/x/sys v0.30.0 // indirect
)

// Local package replacements
replace (
	clientcommon => ../clientcommon
	pbscommon => ../pbscommon
	snapshot => ../snapshot
)
