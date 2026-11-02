package machinebackuplib

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// A Stop must stay recognisable two ways: by errors.Is through the wrapping
// Backup adds ("backup disk X: ..."), and by its text, which the GUI's
// history matches ("cancelled by user").
func TestErrCancelledSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("backup device %s: %w", "/dev/sda", ErrCancelled)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("errors.Is(%v, ErrCancelled) = false", err)
	}
	if !strings.Contains(err.Error(), "cancelled by user") {
		t.Fatalf("%q does not say \"cancelled by user\"", err.Error())
	}
}
