module directorybackup

go 1.25.0

require (
	clientcommon v0.0.0
	github.com/alphadose/haxmap v1.4.1
	pbscommon v0.0.0
	snapshot v0.0.0
)

require (
	github.com/alessio/shellescape v1.4.2 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/rodolfoag/gow32 v0.0.0-20230512144032-1e896a3c51aa // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/tawesoft/golib/v2 v2.16.0
	golang.org/x/sys v0.44.0 // indirect
)

// Local package replacements
replace (
	clientcommon => ../clientcommon
	pbscommon => ../pbscommon
	snapshot => ../snapshot
)
