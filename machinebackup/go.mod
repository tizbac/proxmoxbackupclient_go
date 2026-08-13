module machinebackup

go 1.25

require (
	clientcommon v0.0.0
	github.com/alphadose/haxmap v1.4.1
	github.com/google/uuid v1.6.0
	github.com/tawesoft/golib/v2 v2.16.0
	golang.org/x/sys v0.20.0
	pbscommon v0.0.0
	snapshot v0.0.0
)

require (
	github.com/alessio/shellescape v1.4.2 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/rodolfoag/gow32 v0.0.0-20230512144032-1e896a3c51aa // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	golang.org/x/exp v0.0.0-20240531132922-fd00a4e0eefc // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/text v0.15.0 // indirect
)

// Local package replacements
replace (
	clientcommon => ../clientcommon
	pbscommon => ../pbscommon
	snapshot => ../snapshot
	machinebackuplib => ../machinebackuplib
)
