package main

import (
	"math"
	"testing"
)

func planNear(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The case seen on a real restore: .claude 1.6 GB, Downloads 32.7 GB,
// Documents 2.7 GB. With 25.7 of Downloads' 32.7 GB written the whole restore
// is 74% done, not the 59% the equal thirds gave.
func TestRestorePlanWeighsBySize(t *testing.T) {
	const tenth = int64(1e8) // tenths of a GB, so the sums stay whole
	p := newRestorePlan([]int64{16 * tenth, 327 * tenth, 27 * tenth})
	if !p.weighted() || p.total() != 370*tenth {
		t.Fatalf("weighted %v total %d", p.weighted(), p.total())
	}
	if got := p.fraction(1, 257.0/327); !planNear(got, float64(16+257)/370) {
		t.Fatalf("fraction %v", got)
	}
	if got := p.fraction(0, 0); got != 0 {
		t.Fatalf("start %v", got)
	}
	if got := p.fraction(2, 1); !planNear(got, 1) {
		t.Fatalf("end %v", got)
	}
	if p.before(2) != 343*tenth {
		t.Fatalf("before %d", p.before(2))
	}
	if got := p.bytesAt(1, 0.5); got != 16*tenth+int64(0.5*float64(327*tenth)) {
		t.Fatalf("bytesAt %d", got)
	}
}

// Moving from one folder to the next never moves the bar back or forward.
func TestRestorePlanContinuousAcrossFolders(t *testing.T) {
	p := newRestorePlan([]int64{100, 1000, 10})
	for i := 0; i < 2; i++ {
		if !planNear(p.fraction(i, 1), p.fraction(i+1, 0)) {
			t.Fatalf("jump between folder %d and %d", i, i+1)
		}
		if p.bytesAt(i, 1) != p.bytesAt(i+1, 0) {
			t.Fatalf("bytes jump between folder %d and %d", i, i+1)
		}
	}
}

// A folder of unknown size: equal shares, as before the plan.
func TestRestorePlanFallsBackToEqualShares(t *testing.T) {
	p := newRestorePlan([]int64{100, 0, 50})
	if p.weighted() {
		t.Fatal("weighted with an unknown size")
	}
	if got := p.fraction(1, 0.5); !planNear(got, 1.5/3) {
		t.Fatalf("equal shares: %v", got)
	}
	if got := newRestorePlan(nil).fraction(0, 0.5); got != 0 {
		t.Fatalf("empty plan: %v", got)
	}
	if newRestorePlan(nil).weighted() {
		t.Fatal("empty plan weighted")
	}
}

func TestArchiveSelectionAndWholeArchive(t *testing.T) {
	sel := []string{"Docs/a.txt", "Downloads", "Other/x"}
	own, ref := archiveSelection(RestoreOptions{}, "Docs", "docs.pxar.didx", sel)
	if !ref || len(own) != 1 || own[0] != "a.txt" {
		t.Fatalf("Docs: %v %v", own, ref)
	}
	if wholeArchiveSelected(own, sel, false) {
		t.Fatal("Docs is only part selected")
	}
	own, ref = archiveSelection(RestoreOptions{}, "Downloads", "downloads.pxar.didx", sel)
	if !ref || len(own) != 1 || own[0] != "" || !wholeArchiveSelected(own, sel, false) {
		t.Fatalf("Downloads whole: %v %v", own, ref)
	}
	if _, ref = archiveSelection(RestoreOptions{}, "Music", "music.pxar.didx", sel); ref {
		t.Fatal("Music is not in the selection")
	}
	// Nothing selected at all means everything, in every archive.
	own, ref = archiveSelection(RestoreOptions{}, "Music", "music.pxar.didx", nil)
	if !ref || !wholeArchiveSelected(own, nil, false) {
		t.Fatal("empty selection is everything")
	}
	// Per-archive raw paths: an empty list is not everything.
	raw := RestoreOptions{ArchiveIncludes: map[string][]string{"docs.pxar.didx": {"a.txt"}}}
	own, ref = archiveSelection(raw, "Docs", "docs.pxar.didx", sel)
	if !ref || own[0] != "a.txt" || wholeArchiveSelected(own, nil, true) {
		t.Fatalf("raw: %v %v", own, ref)
	}
	if _, ref = archiveSelection(raw, "Music", "music.pxar.didx", sel); ref {
		t.Fatal("raw: Music has no paths")
	}
	got := wrappedIncludesFor([]string{`Docs\a.txt`, "Docs", "Docsx/b", "Other"}, "Docs")
	if len(got) != 2 || got[0] != `Docs\a.txt` || got[1] != "Docs" {
		t.Fatalf("wrapped: %v", got)
	}
}

func TestRestorePlanClamps(t *testing.T) {
	p := newRestorePlan([]int64{100, 100})
	if got := p.fraction(0, 5); !planNear(got, 0.5) {
		t.Fatalf("over 1: %v", got)
	}
	if got := p.fraction(0, -1); got != 0 {
		t.Fatalf("negative: %v", got)
	}
	if got := p.fraction(0, math.NaN()); got != 0 {
		t.Fatalf("NaN: %v", got)
	}
	if ratio(5, 0) != 0 || !planNear(ratio(1, 4), 0.25) {
		t.Fatal("ratio")
	}
}
