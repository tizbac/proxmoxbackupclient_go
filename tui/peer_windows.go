//go:build windows

package tui

// Who answers on the service's port (gui/api/peer.go). Before anything is sent
// on a new connection to 127.0.0.1:18765, the process holding the other end
// must be the running background service's, as the Service Control Manager
// reports it: a program squatting on the port while the service is stopped
// would otherwise be handed the token. This is the same check the desktop app
// installs (gui/service_peer_windows.go), with the service names of every brand.

import (
	"fmt"
	"net"
	"slices"
	"unsafe"

	"github.com/salvanza/salvanza_client/gui/api"
	"golang.org/x/sys/windows"
)

// serviceNames are the Windows service names of Salvanza's brands, and the
// names the service had before the rename.
var serviceNames = []string{
	"salvanza-svc", "nimbusbackup-svc", "etitechbackup-svc",
	"ProxmoxBackupClientSVC", "ProxmoxBackupClient", "NimbusBackupSVC", "EtitechBackupSVC", "AcmeBackupSVC",
}

var procGetExtendedTcpTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

// tcpTableOwnerPIDAll is TCP_TABLE_OWNER_PID_ALL: every socket, with its process.
const tcpTableOwnerPIDAll = 5

func init() {
	api.SetServicePeerCheck(checkServicePeer)
}

func checkServicePeer(conn net.Conn) error {
	addr := conn.RemoteAddr().String()
	pid, err := api.ServiceEndOwner(conn, tcpTableRows)
	if err != nil {
		return &api.ServicePeerError{Addr: addr, Err: err}
	}
	want, err := runningServicePIDs()
	if err != nil {
		return &api.ServicePeerError{Addr: addr, Err: fmt.Errorf("the background service's process: %w", err)}
	}
	if !slices.Contains(want, pid) {
		return &api.ServicePeerError{Addr: addr, Owner: fmt.Sprintf("process %d", pid)}
	}
	return nil
}

// runningServicePIDs returns the process IDs of the background service while
// it runs, none when it is not installed or not running.
func runningServicePIDs() ([]uint32, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseServiceHandle(scm) }()
	var pids []uint32
	for _, n := range serviceNames {
		p, err := windows.UTF16PtrFromString(n)
		if err != nil {
			continue
		}
		h, err := windows.OpenService(scm, p, windows.SERVICE_QUERY_STATUS)
		if err != nil {
			continue // not installed under this name
		}
		var st windows.SERVICE_STATUS_PROCESS
		var need uint32
		err = windows.QueryServiceStatusEx(h, windows.SC_STATUS_PROCESS_INFO, (*byte)(unsafe.Pointer(&st)), uint32(unsafe.Sizeof(st)), &need)
		_ = windows.CloseServiceHandle(h)
		if err == nil && st.ProcessId != 0 {
			pids = append(pids, st.ProcessId)
		}
	}
	return pids, nil
}

// tcpTableRows reads the system's IPv4 and IPv6 TCP tables.
func tcpTableRows() ([]api.TCPRow, error) {
	buf, err := extendedTCPTable(windows.AF_INET)
	if err != nil {
		return nil, err
	}
	rows, err := api.ParseTCPTableOwnerPID(buf, false)
	if err != nil {
		return nil, err
	}
	if buf6, err := extendedTCPTable(windows.AF_INET6); err == nil {
		if rows6, err := api.ParseTCPTableOwnerPID(buf6, true); err == nil {
			rows = append(rows, rows6...)
		}
	}
	return rows, nil
}

func extendedTCPTable(af uint32) ([]byte, error) {
	size := uint32(16 << 10)
	for range 5 {
		buf := make([]byte, size)
		r, _, _ := procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(af), tcpTableOwnerPIDAll, 0)
		switch windows.Errno(r) {
		case windows.ERROR_SUCCESS:
			return buf, nil
		case windows.ERROR_INSUFFICIENT_BUFFER:
			size += 4 << 10
		default:
			return nil, fmt.Errorf("GetExtendedTcpTable: %w", windows.Errno(r))
		}
	}
	return nil, fmt.Errorf("GetExtendedTcpTable: the table kept growing")
}
