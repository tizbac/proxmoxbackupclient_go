module pbsnbd

go 1.25

require (
	github.com/gdamore/tcell/v2 v2.8.1
	github.com/pojntfx/go-nbd v0.3.2
	github.com/rivo/tview v0.42.0
	pbscommon v0.0.0
)

require (
	github.com/alphadose/haxmap v1.4.1 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/pilebones/go-udev v0.9.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/exp v0.0.0-20221031165847-c99f073a8326 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/term v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

// Local package replacements
replace pbscommon => ../pbscommon
