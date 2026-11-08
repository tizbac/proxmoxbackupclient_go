package main

// restorePlan weighs a multi-folder restore's progress by each folder's size,
// so the bar, the bytes and the time left are for the whole restore: a 1.6 GB
// folder and a 32.7 GB one used to get a third of the bar each.
//
// The sizes are fixed before the first folder starts (a whole folder's from
// the snapshot's manifest, part of one from the browse tree's sizes), so the
// total never changes under the bar. When any folder's size is not known, the
// plan falls back to the old equal shares, and the bytes are each folder's own.
type restorePlan struct {
	sizes []int64
	known bool
}

func newRestorePlan(sizes []int64) *restorePlan {
	p := &restorePlan{sizes: make([]int64, len(sizes)), known: len(sizes) > 0}
	for i, n := range sizes {
		if n > 0 {
			p.sizes[i] = n
		} else {
			p.known = false
		}
	}
	return p
}

// weighted reports whether every folder's size is known, so the plan weighs
// by size and reports bytes for the whole restore.
func (p *restorePlan) weighted() bool { return p.known }

// size is folder i's planned size.
func (p *restorePlan) size(i int) int64 {
	if i < 0 || i >= len(p.sizes) {
		return 0
	}
	return p.sizes[i]
}

// total is every folder's planned size together.
func (p *restorePlan) total() int64 {
	var t int64
	for _, n := range p.sizes {
		t += n
	}
	return t
}

// before is the planned size of the folders before folder i: what is done
// once folder i starts.
func (p *restorePlan) before(i int) int64 {
	var t int64
	for j := 0; j < i && j < len(p.sizes); j++ {
		t += p.sizes[j]
	}
	return t
}

// fraction is how far through the whole restore it is when folder i is frac
// (0 to 1) of the way through, from 0 to 1.
func (p *restorePlan) fraction(i int, frac float64) float64 {
	n := len(p.sizes)
	if n == 0 {
		return 0
	}
	frac = clampUnit(frac)
	if !p.known {
		return clampUnit((float64(i) + frac) / float64(n))
	}
	return clampUnit((float64(p.before(i)) + frac*float64(p.size(i))) / float64(p.total()))
}

// bytesAt is the whole restore's bytes done when folder i is frac of the way
// through (only meaningful when weighted).
func (p *restorePlan) bytesAt(i int, frac float64) int64 {
	return p.before(i) + int64(clampUnit(frac)*float64(p.size(i)))
}

func clampUnit(f float64) float64 {
	if f < 0 || f != f {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// ratio is done/total as a fraction, 0 when total is not positive.
func ratio(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(done) / float64(total)
}
