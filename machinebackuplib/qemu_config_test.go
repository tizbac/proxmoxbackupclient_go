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
		"#qmdump#map:sata0:drive-sata0::raw:",
		"#qmdump#map:sata1:drive-sata1::raw:",
		"vmgenid: 11111111-1111-1111-1111-111111111111",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q\n%s", want, cfg)
		}
	}
}

func TestRenderQemuConfigUEFI(t *testing.T) {
	base := qemuConfigData{VMGenId: "g", VMID: 107, VMName: "h", OS: "win11", SMBIOS: "s", Disks: []BackupDisk{{Index: 0, Size: 1 << 30}}}
	out, err := renderQemuConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "bios:") {
		t.Errorf("BIOS disk must not get a bios line\n%s", out)
	}
	base.UEFI = true
	out, err = renderQemuConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "bios: ovmf\n") || strings.Contains(string(out), "efidisk") {
		t.Errorf("UEFI config should start with bios: ovmf and have no efidisk\n%s", out)
	}
}

func TestGPTDetection(t *testing.T) {
	mk := func(off int) []byte {
		b := make([]byte, 8192)
		copy(b[off:], "EFI PART")
		return b
	}
	if !gptSignatureIn(mk(512)) || !gptSignatureIn(mk(4096)) {
		t.Error("GPT header not found")
	}
	if gptSignatureIn(make([]byte, 8192)) {
		t.Error("MBR/blank disk reported as GPT")
	}
	if !bootDiskIsGPT([]BackupDisk{{Index: 1}, {Index: 0, GPT: true}}) || bootDiskIsGPT([]BackupDisk{{Index: 0}, {Index: 1, GPT: true}}) {
		t.Error("bootDiskIsGPT must follow the lowest-index disk")
	}
}
