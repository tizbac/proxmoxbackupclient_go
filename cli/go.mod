module salvanza-cli

go 1.25.0

require (
	clientcommon v0.0.0
	directorybackup v0.0.0
	machinebackup v0.0.0
	pbsnbd v0.0.0
)

require (
	github.com/alessio/shellescape v1.4.2 // indirect
	github.com/alphadose/haxmap v1.4.1 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/gdamore/tcell/v2 v2.8.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pilebones/go-udev v0.9.0 // indirect
	github.com/pojntfx/go-nbd v0.3.2 // indirect
	github.com/rivo/tview v0.42.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rodolfoag/gow32 v0.0.0-20230512144032-1e896a3c51aa // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	github.com/tawesoft/golib/v2 v2.16.0 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	machinebackuplib v0.0.0 // indirect
	pbscommon v0.0.0 // indirect
	retry v0.0.0 // indirect
	snapshot v0.0.0 // indirect
)

// The project's own modules, beside this one.
replace (
	clientcommon => ../clientcommon
	directorybackup => ../directorybackup
	github.com/st-matskevich/go-vss => ../third_party/go-vss
	machinebackup => ../machinebackup
	machinebackuplib => ../machinebackuplib
	pbscommon => ../pbscommon
	pbsnbd => ../nbd
	retry => ../pkg/retry
	security => ../pkg/security
	snapshot => ../snapshot
)
