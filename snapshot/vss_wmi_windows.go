//go:build windows

package snapshot

import (
	"fmt"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// shadowDeviceFromWMI returns the device of shadow copy id ("{GUID}") as
// Win32_ShadowCopy has it, for when VSS's own snapshot properties leave it
// empty (CreateVSSSnapshot, #131). COM must be initialised on the calling
// thread.
func shadowDeviceFromWMI(id string) (string, error) {
	id = strings.ToUpper(strings.Trim(id, "{}"))
	if id == "" || strings.ContainsAny(id, `'"\`) {
		return "", fmt.Errorf("bad shadow copy id %q", id)
	}
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return "", fmt.Errorf("WMI locator: %w", err)
	}
	defer unknown.Release()
	locator, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", fmt.Errorf("WMI locator: %w", err)
	}
	defer locator.Release()
	svcRaw, err := oleutil.CallMethod(locator, "ConnectServer", nil, `root\cimv2`)
	if err != nil {
		return "", fmt.Errorf("WMI connect: %w", err)
	}
	svc := svcRaw.ToIDispatch()
	defer svcRaw.Clear()
	resRaw, err := oleutil.CallMethod(svc, "ExecQuery", "SELECT DeviceObject FROM Win32_ShadowCopy WHERE ID='{"+id+"}'")
	if err != nil {
		return "", fmt.Errorf("WMI query: %w", err)
	}
	res := resRaw.ToIDispatch()
	defer resRaw.Clear()
	countRaw, err := oleutil.GetProperty(res, "Count")
	if err != nil {
		return "", fmt.Errorf("WMI query: %w", err)
	}
	count := int(countRaw.Val)
	countRaw.Clear()
	for i := 0; i < count; i++ {
		itemRaw, err := oleutil.CallMethod(res, "ItemIndex", i)
		if err != nil {
			return "", fmt.Errorf("WMI result: %w", err)
		}
		devRaw, err := oleutil.GetProperty(itemRaw.ToIDispatch(), "DeviceObject")
		if err == nil {
			dev := devRaw.ToString()
			devRaw.Clear()
			if dev != "" {
				itemRaw.Clear()
				return dev, nil
			}
		}
		itemRaw.Clear()
	}
	return "", fmt.Errorf("no Win32_ShadowCopy with id {%s}", id)
}
