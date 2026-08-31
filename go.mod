module proxmoxbackupclient_go

go 1.22

replace (
	"github.com/tawesoft/golib/v2" => github.com/tawesoft/golib/v2 v2.0.0
	"github.com/tawesoft/golib" => github.com/tawesoft/golib v2.0.0
)

require (
	github.com/alphadose/haxmap v1.4.1
	github.com/google/uuid v1.6.0
	golang.org/x/sys v0.23.0
)