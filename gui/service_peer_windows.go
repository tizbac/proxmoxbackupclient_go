//go:build windows && !service

package main

// Who answers on the service's port (gui/api/peer.go). Before the app sends
// anything on a new connection to 127.0.0.1:18765, the process holding the
// other end of that connection (the system's TCP table) must be the running
// background service's, as the Service Control Manager reports it. Anything
// else, a program squatting on the port while the service is stopped
// included, is refused before it sees the token or a PBS secret.
//
// There is no fallback when the Service Control Manager cannot name the
// service's process (not installed, not running): the connection is refused,
// as for a stopped service. A fallback on the owning process's account
// (LocalSystem) would have to open that process, which Windows does not let
// a standard user do for a SYSTEM process, so it could only refuse as well.

import (
	"fmt"
	"net"
	"path/filepath"
	"slices"
	"unsafe"

	"github.com/salvanza/salvanza_client/gui/api"
	"golang.org/x/sys/windows"
)

var procGetExtendedTcpTable = windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

// tcpTableOwnerPIDAll is TCP_TABLE_OWNER_PID_ALL: every socket, with the
// process that owns it.
const tcpTableOwnerPIDAll = 5

func init() {
	api.SetServicePeerCheck(checkServicePeer)
}

// checkServicePeer is the check api.SetServicePeerCheck runs on each new
// connection to the service's address.
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
		return &api.ServicePeerError{Addr: addr, Owner: processLabel(pid)}
	}
	return nil
}

// runningServicePIDs returns the process IDs of this app's background
// service (serviceNames: the brand's name and the older one) while it runs.
// None when it is not installed or not running.
func runningServicePIDs() ([]uint32, error) {
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseServiceHandle(scm) }()
	_, names, _ := serviceNames()
	var pids []uint32
	for _, n := range names {
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

// tcpTableRows reads the system's IPv4 and IPv6 TCP tables. The IPv6 one is
// left out if Windows will not give it (IPv6 turned off): a connection to
// 127.0.0.1 is in the IPv4 one, and a socket not found is refused anyway.
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

// extendedTCPTable returns GetExtendedTcpTable's buffer for the address
// family af, growing it while the table does not fit.
func extendedTCPTable(af uint32) ([]byte, error) {
	size := uint32(16 << 10)
	for range 5 {
		buf := make([]byte, size)
		r, _, _ := procGetExtendedTcpTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(af), tcpTableOwnerPIDAll, 0)
		switch windows.Errno(r) {
		case windows.ERROR_SUCCESS:
			return buf, nil
		case windows.ERROR_INSUFFICIENT_BUFFER:
			size += 4 << 10 // room for sockets opened since
		default:
			return nil, fmt.Errorf("GetExtendedTcpTable: %w", windows.Errno(r))
		}
	}
	return nil, fmt.Errorf("GetExtendedTcpTable: the table kept growing")
}

// processLabel names process pid for the message and the log: "process
// 1234 (python.exe)", or just "process 1234" when it cannot be opened.
func processLabel(pid uint32) string {
	label := fmt.Sprintf("process %d", pid)
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return label
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(windows.MAX_LONG_PATH)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, filepath.Base(windows.UTF16ToString(buf[:n])))
}
