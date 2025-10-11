//go:build windows
// +build windows

package snapshot

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	vss "github.com/jeromehadorn/vss"
)

func SymlinkSnapshot(symlinkPath string, id string, deviceObjectPath string) (string, error) {

	snapshotSymLinkFolder := symlinkPath + "\\" + id + "\\"

	snapshotSymLinkFolder = filepath.Clean(snapshotSymLinkFolder)
	os.RemoveAll(snapshotSymLinkFolder)
	if err := os.MkdirAll(snapshotSymLinkFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create snapshot symlink folder for snapshot: %s, err: %s", id, err)
	}

	os.Remove(snapshotSymLinkFolder)

	fmt.Println("Symlink from: ", deviceObjectPath, " to: ", snapshotSymLinkFolder)

	if err := os.Symlink(deviceObjectPath, snapshotSymLinkFolder); err != nil {
		return "", fmt.Errorf("failed to create symlink from: %s to: %s, error: %s", deviceObjectPath, snapshotSymLinkFolder, err)
	}

	return snapshotSymLinkFolder, nil
}

func getAppDataFolder() (string, error) {
	// Get information about the current user
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	// Construct the path to the application data folder
	appDataFolder := filepath.Join(currentUser.HomeDir, "AppData", "Roaming", "PBSBackupGO")

	// Create the folder if it doesn't exist
	err = os.MkdirAll(appDataFolder, os.ModePerm)
	if err != nil {
		return "", err
	}

	return appDataFolder, nil
}

func CreateVSSSnapshot(path string) SnapShot {

	path, _ = filepath.Abs(path)
	volName := filepath.VolumeName(path)
	volName += "\\"
	subPath := path[len(volName):] //Strp C:\, 3 chars or whatever it is

	appDataFolder, err := getAppDataFolder()
	if err != nil {
		fmt.Println("Error:", err)
		return SnapShot{FullPath: path, Valid: false}
	}

	sn := vss.Snapshotter{}
	snapid, err := os.ReadFile(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
	if err == nil {
		snapid_str := string(snapid)

		fmt.Printf("Found leftover snapshot, deleting it...\n")

		sn.DeleteSnapshot(snapid_str)

		os.Remove(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
	}

	fmt.Printf("Creating VSS Snapshot...")
	snapshot, err := sn.CreateSnapshot(volName, 180, true)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Snapshot created: %s\n", snapshot.Id)

	f, err := os.Create(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
	if err != nil {
		sn.DeleteSnapshot(snapshot.Id)
		panic(err)
	}

	f.WriteString(snapshot.Id)
	f.Close()

	_, err = SymlinkSnapshot(filepath.Join(appDataFolder, "VSS"), snapshot.Id, snapshot.DeviceObjectPath)

	if err != nil {
		sn.DeleteSnapshot(snapshot.Id)
		os.Remove(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
		panic(err)
	}

	return SnapShot{FullPath: filepath.Join(appDataFolder, "VSS", snapshot.Id, subPath), Id: snapshot.Id, ObjectPath: snapshot.DeviceObjectPath, Valid: true}

}

func VSSCleanup() {
	appDataFolder, err := getAppDataFolder()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	sn := vss.Snapshotter{}
	snapid, err := os.ReadFile(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
	if err == nil {
		snapid_str := string(snapid)

		fmt.Printf("Found leftover snapshot, deleting it...\n")

		sn.DeleteSnapshot(snapid_str)

		os.Remove(filepath.Join(appDataFolder, "temp_snapshot_id.txt"))
	}
}
