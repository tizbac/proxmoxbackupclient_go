module snapshot

go 1.25

require (
	github.com/st-matskevich/go-vss v0.3.3
	golang.org/x/sys v0.1.0
)

require github.com/go-ole/go-ole v1.2.6

// go-vss with its COM reference counts fixed (third_party/go-vss).
replace github.com/st-matskevich/go-vss => ../third_party/go-vss
