module clientcommon

go 1.25

require github.com/rodolfoag/gow32 v0.0.0-20230512144032-1e896a3c51aa

require (
	golang.org/x/term v0.36.0
	pbscommon v0.0.0
)

require (
	github.com/alphadose/haxmap v1.4.1 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	golang.org/x/exp v0.0.0-20221031165847-c99f073a8326 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace pbscommon => ../pbscommon
