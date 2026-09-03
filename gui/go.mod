module github.com/tizbac/proxmoxbackupclient_go/gui

go 1.25.0

require (
	github.com/alphadose/haxmap v1.4.1
	github.com/getlantern/systray v1.2.2
	github.com/kardianos/service v1.3.0
	github.com/tizbac/proxmoxbackupclient_go/gui/api v0.0.0
	github.com/wailsapp/wails/v2 v2.13.0
	golang.org/x/sys v0.44.0
	machinebackuplib v0.0.0-00010101000000-000000000000
	pbscommon v0.0.0
	retry v0.0.0
	security v0.0.0
	snapshot v0.0.0
)

require (
	git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3 // indirect
	github.com/alessio/shellescape v1.4.2 // indirect
	github.com/bep/debounce v1.2.1 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/getlantern/context v0.0.0-20190109183933-c447772a6520 // indirect
	github.com/getlantern/errors v0.0.0-20190325191628-abdb3e3e36f7 // indirect
	github.com/getlantern/golog v0.0.0-20190830074920-4ef2e798c2d7 // indirect
	github.com/getlantern/hex v0.0.0-20190417191902-c6586a6fe0b7 // indirect
	github.com/getlantern/hidden v0.0.0-20190325191715-f02dbb02be55 // indirect
	github.com/getlantern/ops v0.0.0-20190325191751-d70cb0d6f85f // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/labstack/echo/v4 v4.13.3 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leaanthony/go-ansi-parser v1.6.1 // indirect
	github.com/leaanthony/gosod v1.0.4 // indirect
	github.com/leaanthony/slicer v1.6.0 // indirect
	github.com/leaanthony/u v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/st-matskevich/go-vss v0.3.3 // indirect
	github.com/tawesoft/golib/v2 v2.16.0 // indirect
	github.com/tkrajina/go-reflector v0.5.8 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/wailsapp/go-webview2 v1.0.22 // indirect
	github.com/wailsapp/mimetype v1.4.1 // indirect
	golang.org/x/crypto v0.51.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

// Local package replacements to use sibling directories
replace (
	clientcommon => ../clientcommon
	github.com/tizbac/proxmoxbackupclient_go/gui/api => ./api
	machinebackuplib => ../machinebackuplib
	pbscommon => ../pbscommon
	retry => ../pkg/retry
	security => ../pkg/security
	snapshot => ../snapshot
)
