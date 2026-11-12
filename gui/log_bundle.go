package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// The service's logs are in its own folder, which only the service may read (it
// holds the settings too), and the app's are in the user's. A person who has to
// attach them to a bug report needs one file with both, and no change to who may
// read what. So the app asks the service for its logs through its API, and
// packs them with its own into one zip.

const (
	// logBundleDays is how far back a bundle goes.
	logBundleDays = 14
	// logBundleMaxBytes caps the log files in a bundle (before compression), newest first.
	logBundleMaxBytes = 150 << 20
)

// bundleFile is one log file for a bundle.
type bundleFile struct {
	dir, name string
	size      int64
	modified  time.Time
}

// collectLogs lists the log files in dirs that were written in the last days,
// newest first, until maxBytes is reached; leftOut counts the ones that did
// not fit (older ones, so a bundle always has the newest logs).
func collectLogs(dirs []string, days int, maxBytes int64) (files []bundleFile, leftOut int) {
	cutoff := time.Now().AddDate(0, 0, -days)
	var all []bundleFile
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if _, _, _, ok := classifyLog(e.Name()); !ok {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}
			all = append(all, bundleFile{dir: dir, name: e.Name(), size: info.Size(), modified: info.ModTime()})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].modified.After(all[j].modified) })
	var total int64
	for _, f := range all {
		if total+f.size > maxBytes && len(files) > 0 {
			leftOut++
			continue
		}
		total += f.size
		files = append(files, f)
	}
	return files, leftOut
}

// addLogFiles copies the files into the zip under prefix, keeping their times.
// A file that cannot be read is noted in the returned list, not fatal.
func addLogFiles(zw *zip.Writer, prefix string, files []bundleFile) (skipped []string, err error) {
	for _, f := range files {
		src, oerr := os.Open(filepath.Join(f.dir, f.name))
		if oerr != nil {
			skipped = append(skipped, f.name+": "+oerr.Error())
			continue
		}
		method := zip.Deflate
		if strings.HasSuffix(f.name, ".gz") {
			method = zip.Store // already compressed
		}
		w, cerr := zw.CreateHeader(&zip.FileHeader{Name: prefix + f.name, Method: method, Modified: f.modified})
		if cerr != nil {
			_ = src.Close()
			return skipped, cerr
		}
		_, cerr = io.Copy(w, src)
		_ = src.Close()
		if cerr != nil {
			skipped = append(skipped, f.name+": "+cerr.Error())
		}
	}
	return skipped, nil
}

// bundleInfo is the readme at the top of a bundle: what it is and what is in it.
// It says nothing about the computer beyond the operating system and the
// version of the program.
func bundleInfo(part string, files []bundleFile, leftOut int, skipped []string, note string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s logs (%s)\n", product().Title, part)
	fmt.Fprintf(&b, "Version:  %s\n", appVersion)
	fmt.Fprintf(&b, "System:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "Saved:    %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&b, "Covers:   the last %d days\n", logBundleDays)
	if note != "" {
		fmt.Fprintf(&b, "Note:     %s\n", note)
	}
	if leftOut > 0 {
		fmt.Fprintf(&b, "Left out: %d older file(s) that did not fit the size limit\n", leftOut)
	}
	b.WriteString("\nFiles:\n")
	for _, f := range files {
		fmt.Fprintf(&b, "  %-52s %10d bytes  %s\n", f.name, f.size, f.modified.Format("2006-01-02 15:04:05"))
	}
	for _, s := range skipped {
		fmt.Fprintf(&b, "  could not read: %s\n", s)
	}
	b.WriteString("\nThe logs name folders and servers of this computer. Look through them before posting them in public.\n")
	return b.String()
}

// localLogBundle is a zip of this program's own log files (the service's, when
// the service builds it for the app: LogBundleForAPI).
func (a *App) localLogBundle(days int) ([]byte, error) {
	if days <= 0 {
		days = logBundleDays
	}
	files, leftOut := collectLogs(logFolders(), days, logBundleMaxBytes)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	skipped, err := addLogFiles(zw, "", files)
	if err != nil {
		return nil, err
	}
	w, err := zw.Create("info.txt")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, bundleInfo("service", files, leftOut, skipped, "")); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LogBundleForAPI is the service's logs as a zip, for an app that cannot read
// the service's folder.
func (a *App) LogBundleForAPI(days int) ([]byte, error) {
	return a.localLogBundle(days)
}

// buildLogBundle is the zip the app saves: its own logs under app/, the
// service's under service/ (asked of the service), and an info.txt on top.
func (a *App) buildLogBundle(days int) ([]byte, error) {
	if days <= 0 {
		days = logBundleDays
	}
	files, leftOut := collectLogs(a.logDirs(), days, logBundleMaxBytes)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	skipped, err := addLogFiles(zw, "app/", files)
	if err != nil {
		return nil, err
	}
	note := "the service's logs are not part of this file (the app does not use the service)"
	svcCount := 0
	if a.delegated() {
		note = ""
		data, derr := a.apiClient.LogBundle(days)
		if derr != nil {
			note = fmt.Sprintf("the service's logs could not be fetched: %v", derr)
		} else if n, merr := mergeLogZip(zw, "service/", data); merr != nil {
			note = fmt.Sprintf("the service's logs could not be read: %v", merr)
		} else {
			svcCount = n
		}
	}
	if svcCount > 0 {
		note = fmt.Sprintf("includes %d file(s) from the service", svcCount)
	}
	w, err := zw.Create("info.txt")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, bundleInfo("app and service", files, leftOut, skipped, note)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mergeLogZip copies the entries of another zip into zw under prefix, without
// unpacking them, and returns how many log files that was.
func mergeLogZip(zw *zip.Writer, prefix string, data []byte) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name) // never a path out of the folder
		if name == "info.txt" {
			name = "info-service.txt"
		} else {
			n++
		}
		fh := f.FileHeader
		fh.Name = prefix + name
		w, err := zw.CreateRaw(&fh)
		if err != nil {
			return n, err
		}
		raw, err := f.OpenRaw()
		if err != nil {
			return n, err
		}
		if _, err := io.Copy(w, raw); err != nil {
			return n, err
		}
	}
	return n, nil
}
