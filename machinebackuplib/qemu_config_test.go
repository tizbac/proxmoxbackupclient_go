package machinebackuplib

import (
	"strings"
	"testing"
)

func TestRenderQemuConfigUsesVMIDForEveryDisk(t *testing.T) {
	out, err := renderQemuConfig(qemuConfigData{
		VMGenId: "11111111-1111-1111-1111-111111111111",
		VMID:    107,
		VMName:  "testhost",
		OS:      "l26",
		SMBIOS:  "22222222-2222-2222-2222-222222222222",
		Disks:   []BackupDisk{{Index: 0, Size: 1 << 30}, {Index: 1, Size: 2 << 30}},
	})
	if err != nil {
		t.Fatalf("renderQemuConfig: %v", err)
	}
	cfg := string(out)
	for _, want := range []string{
		"name: testhost",
		"sata0: local:107/vm-107-disk-0.raw,cache=writeback,discard=on,size=1073741824",
		"sata1: local:107/vm-107-disk-1.raw,cache=writeback,discard=on,size=2147483648",
		"vmgenid: 11111111-1111-1111-1111-111111111111",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q\n%s", want, cfg)
		}
	}
}
