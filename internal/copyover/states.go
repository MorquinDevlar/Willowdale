package copyover

import (
	"time"
)

// CopyoverPhase for backwards compatibility
type CopyoverPhase int

// Map new states to old for compatibility
const (
	PhaseIdle       CopyoverPhase = 0
	PhaseScheduled  CopyoverPhase = 1
	PhaseAnnouncing CopyoverPhase = 2 // Deprecated - now part of StateScheduled
	PhaseBuilding   CopyoverPhase = 3 // Deprecated - now part of StatePreparing
	PhaseSaving     CopyoverPhase = 4 // Deprecated - now part of StatePreparing
	PhaseGathering  CopyoverPhase = 5 // Deprecated - now part of StatePreparing
	PhaseExecuting  CopyoverPhase = 6
	PhaseRecovering CopyoverPhase = 7
	PhaseCancelling CopyoverPhase = 8 // Deprecated - cancellation is immediate
	PhaseFailed     CopyoverPhase = 9 // Deprecated - failures return to idle
)

// IsActive returns true if copyover is in progress
func (p CopyoverPhase) IsActive() bool {
	return p != 0 // Not idle
}

// CopyoverStatus represents the current status (simplified)
type CopyoverStatus struct {
	State          CopyoverPhase `json:"state"`
	StateChangedAt time.Time     `json:"state_changed_at"`
	ScheduledFor   time.Time     `json:"scheduled_for,omitempty"`
	InitiatedBy    int           `json:"initiated_by,omitempty"`
	Reason         string        `json:"reason,omitempty"`
	StartedAt      time.Time     `json:"started_at,omitempty"`

	// Deprecated fields kept for compatibility
	BuildProgress   int           `json:"build_progress,omitempty"`
	SaveProgress    int           `json:"save_progress,omitempty"`
	GatherProgress  int           `json:"gather_progress,omitempty"`
	RestoreProgress int           `json:"restore_progress,omitempty"`
	VetoReasons     []VetoInfo    `json:"veto_reasons,omitempty"`
	TotalCopyovers  int           `json:"total_copyovers"`
	LastCopyoverAt  time.Time     `json:"last_copyover_at,omitempty"`
	AverageDuration time.Duration `json:"average_duration,omitempty"`
	LastError       string        `json:"last_error,omitempty"`
	LastErrorAt     time.Time     `json:"last_error_at,omitempty"`
}

// VetoInfo for module vetoes
type VetoInfo struct {
	Module    string    `json:"module"`
	Reason    string    `json:"reason"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

// CopyoverHistory for tracking past copyovers (simplified)
type CopyoverHistory struct {
	ID               int           `json:"id"`
	StartedAt        time.Time     `json:"started_at"`
	CompletedAt      time.Time     `json:"completed_at"`
	Duration         time.Duration `json:"duration"`
	Success          bool          `json:"success"`
	InitiatedBy      int           `json:"initiated_by"`
	Reason           string        `json:"reason"`
	BuildNumber      string        `json:"build_number"`
	ConnectionsSaved int           `json:"connections_saved"`
	ConnectionsLost  int           `json:"connections_lost"`
	ErrorMessage     string        `json:"error_message,omitempty"`
}

// GetProgress returns overall progress (simplified)
func (s *CopyoverStatus) GetProgress() int {
	// Progress is now tracked in the manager
	return 0
}

// GetTimeUntilCopyover returns time until scheduled copyover
func (s *CopyoverStatus) GetTimeUntilCopyover() time.Duration {
	if s.ScheduledFor.IsZero() {
		return 0
	}

	duration := time.Until(s.ScheduledFor)
	if duration < 0 {
		return 0
	}
	return duration
}

// CanCopyover checks if copyover can be initiated
func (s *CopyoverStatus) CanCopyover() (bool, []string) {
	reasons := []string{}

	if s.State.IsActive() {
		reasons = append(reasons, "Copyover already in progress")
	}

	// Check for hard vetoes
	for _, veto := range s.VetoReasons {
		if veto.Type == "hard" {
			reasons = append(reasons, veto.Reason)
		}
	}

	return len(reasons) == 0, reasons
}
