package clientcommon

import "testing"

func TestGenerateBackupID(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"SERVER01", `D:\DATA\Users`, "SERVER01_D_DATA_Users"},
		{"host", "/srv/my data/photos/", "host_srv_my-data_photos"},
		{"host", `C:\Program Files\App (x86)`, "host_C_Program-Files_App-x86"},
		{"host", "/", "host"},
	}
	for _, c := range cases {
		if got := GenerateBackupID(c.base, c.path); got != c.want {
			t.Errorf("GenerateBackupID(%q, %q) = %q, want %q", c.base, c.path, got, c.want)
		}
	}
}
