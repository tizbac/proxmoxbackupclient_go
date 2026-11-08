//go:build !service

package main

import "testing"

func TestPlanOwnCopy(t *testing.T) {
	svc := &Config{PBSServers: map[string]*PBSServer{
		"pbs1":   {ID: "pbs1", Name: "Office", BaseURL: "https://office:8007", Datastore: "main", AuthID: "backup@pbs!a"},
		"shared": {ID: "shared", Name: "Shared", BaseURL: "https://shared:8007/", Datastore: "ds", AuthID: "u@pbs!t"},
	}, DefaultPBSID: "pbs1"}
	svcJobs := []ScheduledJob{{ID: "job1", Name: "Documents"}}

	own := &Config{PBSServers: map[string]*PBSServer{
		// Same ID as the computer's pbs1, another server, same name.
		"pbs1": {ID: "pbs1", Name: "office", BaseURL: "https://home:8007", Datastore: "home", AuthID: "me@pbs!t", EncryptionKeyFile: "C:/Users/me/key.json"},
		// The computer's "shared", written differently.
		"mine": {ID: "mine", Name: "Mine", BaseURL: "HTTPS://shared:8007", Datastore: "ds", AuthID: "u@pbs!t"},
	}, DefaultPBSID: "pbs1"}
	ownJobs := []ScheduledJob{
		{ID: "job1", Name: "documents"},                   // default server, ID and name taken
		{ID: "job2", Name: "Photos", PBSServerID: "mine"}, // the computer's server
	}

	servers, jobs := planOwnCopy(own, ownJobs, svc, svcJobs, "user")

	if len(servers) != 1 {
		t.Fatalf("servers added = %d, want 1 (the other is the computer's)", len(servers))
	}
	s := servers["pbs1-user"]
	if s == nil || s.ID != "pbs1-user" || s.Name != "office (user)" || s.EncryptionKeyFile == "" {
		t.Fatalf("renamed server = %+v", s)
	}
	if svc.PBSServers["pbs1"].Name != "Office" {
		t.Fatal("the computer's server was changed")
	}

	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	if j := jobs[0]; j.ID != "job1-user" || j.Name != "documents (user)" || j.PBSServerID != "pbs1-user" {
		t.Errorf("clashing set = %+v, want job1-user, documents (user), on pbs1-user (not the computer's default)", j)
	}
	if j := jobs[1]; j.ID != "job2" || j.Name != "Photos" || j.PBSServerID != "shared" {
		t.Errorf("set on a server the computer has = %+v, want job2 on shared", j)
	}
	if ownJobs[0].ID != "job1" || own.PBSServers["pbs1"].ID != "pbs1" {
		t.Error("the user's own settings were changed")
	}
}

func TestPlanOwnCopyRepeatedClash(t *testing.T) {
	svc := &Config{PBSServers: map[string]*PBSServer{
		"a":      {ID: "a", BaseURL: "https://x"},
		"a-user": {ID: "a-user", BaseURL: "https://y"},
	}}
	own := &Config{PBSServers: map[string]*PBSServer{"a": {ID: "a", BaseURL: "https://z"}}}
	servers, _ := planOwnCopy(own, nil, svc, nil, "user")
	if servers["a-user-2"] == nil {
		t.Fatalf("servers = %v, want a-user-2", servers)
	}
}
