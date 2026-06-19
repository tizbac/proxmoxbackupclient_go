package api

import (
	"fmt"
)

// ExecutionMode represents how the application is running
type ExecutionMode int

const (
	// ModeService - GUI connects to local service (HTTP)
	ModeService ExecutionMode = iota
	// ModeStandalone - GUI runs backup directly (needs admin for VSS)
	ModeStandalone
)

// ModeDetector handles execution mode detection
type ModeDetector struct {
	client *Client
}

// NewModeDetector creates a new mode detector. tokenPath is the shared local-API
// token file so the detector's /status probe is authenticated (H-01).
func NewModeDetector(tokenPath string) *ModeDetector {
	return &ModeDetector{
		client: NewClient(tokenPath),
	}
}

// DetectMode checks which mode the application should run in
func (d *ModeDetector) DetectMode() ExecutionMode {
	if d.client.IsServiceAvailable() {
		return ModeService
	}
	return ModeStandalone
}

// GetModeName returns a human-readable mode name
func (m ExecutionMode) String() string {
	switch m {
	case ModeService:
		return "Service Mode"
	case ModeStandalone:
		return "Standalone Mode"
	default:
		return "Unknown Mode"
	}
}

// ShouldWarnVSS determines if VSS warning should be shown
// Warning is shown ONLY if:
// - VSS is requested
// - Service is NOT available
// - Application is NOT running as admin
func ShouldWarnVSS(useVSS bool, mode ExecutionMode, isAdmin bool) (bool, string) {
	if !useVSS {
		return false, ""
	}

	if mode == ModeService {
		// Service handles VSS with admin rights
		return false, ""
	}

	if mode == ModeStandalone && !isAdmin {
		return true, "VSS (Shadow Copy) nécessite les privilèges administrateur - veuillez redémarrer l'application en tant qu'administrateur ou désactiver VSS"
	}

	return false, ""
}

// GetModeDescription returns a description of the current mode
func GetModeDescription(mode ExecutionMode) string {
	switch mode {
	case ModeService:
		return fmt.Sprintf(
			"✅ Mode Service\n"+
				"Le service Windows gère les backups avec privilèges admin.\n"+
				"VSS (Shadow Copy) fonctionne automatiquement.",
		)
	case ModeStandalone:
		return fmt.Sprintf(
			"⚠️ Mode Standalone\n"+
				"Backup direct sans service Windows.\n"+
				"VSS nécessite de lancer l'application en tant qu'administrateur.",
		)
	default:
		return "Mode inconnu"
	}
}
