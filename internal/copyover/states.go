package copyover

import (
	"fmt"
	"time"
)

// CopyoverPhase represents the current phase of the copyover system
type CopyoverPhase int

const (
	// StateIdle - No copyover in progress
	StateIdle CopyoverPhase = iota
	// StateScheduled - Copyover has been scheduled
	StateScheduled
	// StateAnnouncing - Sending countdown announcements
	StateAnnouncing
	// StateBuilding - Building new executable
	StateBuilding
	// StateSaving - Saving player and world state
	StateSaving
	// StateGathering - Gathering state from all systems
	StateGathering
	// StateExecuting - Executing new process
	StateExecuting
	// StateRecovering - Recovering state in new process
	StateRecovering
	// StateCancelling - Cancellation in progress
	StateCancelling
	// StateFailed - Copyover failed
	StateFailed
)

// String returns the string representation of the state
func (s CopyoverPhase) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateScheduled:
		return "scheduled"
	case StateAnnouncing:
		return "announcing"
	case StateBuilding:
		return "building"
	case StateSaving:
		return "saving"
	case StateGathering:
		return "gathering"
	case StateExecuting:
		return "executing"
	case StateRecovering:
		return "recovering"
	case StateCancelling:
		return "cancelling"
	case StateFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// CanTransitionTo checks if a transition to the target state is valid
func (s CopyoverPhase) CanTransitionTo(target CopyoverPhase) bool {
	validTransitions := map[CopyoverPhase][]CopyoverPhase{
		StateIdle: {
			StateScheduled,
			StateBuilding,   // For immediate copyover
			StateRecovering, // For recovery on startup
		},
		StateScheduled: {
			StateAnnouncing,
			StateCancelling,
		},
		StateAnnouncing: {
			StateBuilding,
			StateCancelling,
		},
		StateBuilding: {
			StateSaving,
			StateFailed,
			StateCancelling,
		},
		StateSaving: {
			StateGathering,
			StateFailed,
		},
		StateGathering: {
			StateExecuting,
			StateFailed,
		},
		StateExecuting: {
			StateRecovering, // In new process
			StateFailed,
		},
		StateRecovering: {
			StateIdle, // Success
			StateFailed,
		},
		StateCancelling: {
			StateIdle,
		},
		StateFailed: {
			StateIdle, // Reset after failure
		},
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, validTarget := range allowed {
		if validTarget == target {
			return true
		}
	}
	return false
}

// IsTerminal returns true if this is a terminal state
func (s CopyoverPhase) IsTerminal() bool {
	return s == StateIdle || s == StateFailed
}

// IsActive returns true if copyover is in progress
func (s CopyoverPhase) IsActive() bool {
	return s != StateIdle && s != StateFailed
}

// CopyoverStatus represents the current status of the copyover system
type CopyoverStatus struct {
	// Current state
	State          CopyoverPhase `json:"state"`
	StateChangedAt time.Time     `json:"state_changed_at"`

	// Scheduling info
	ScheduledAt  time.Time `json:"scheduled_at,omitempty"`
	ScheduledFor time.Time `json:"scheduled_for,omitempty"`
	InitiatedBy  int       `json:"initiated_by,omitempty"`
	Reason       string    `json:"reason,omitempty"`

	// Progress tracking
	BuildProgress   int `json:"build_progress,omitempty"`   // 0-100
	SaveProgress    int `json:"save_progress,omitempty"`    // 0-100
	GatherProgress  int `json:"gather_progress,omitempty"`  // 0-100
	RestoreProgress int `json:"restore_progress,omitempty"` // 0-100

	// Timing information
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	LastErrorAt time.Time `json:"last_error_at,omitempty"`

	// Veto information
	VetoReasons []VetoInfo `json:"veto_reasons,omitempty"`

	// Statistics
	TotalCopyovers  int           `json:"total_copyovers"`
	LastCopyoverAt  time.Time     `json:"last_copyover_at,omitempty"`
	AverageDuration time.Duration `json:"average_duration,omitempty"`
}

// VetoInfo contains information about a copyover veto
type VetoInfo struct {
	Module    string    `json:"module"`
	Reason    string    `json:"reason"`
	Type      string    `json:"type"` // "hard" or "soft"
	Timestamp time.Time `json:"timestamp"`
}

// CanCopyover returns whether a copyover can be initiated
func (s *CopyoverStatus) CanCopyover() (bool, []string) {
	reasons := []string{}

	// Check if already in progress
	if s.State.IsActive() {
		reasons = append(reasons, fmt.Sprintf("Copyover already in progress (state: %s)", s.State))
	}

	// Check for hard vetoes
	for _, veto := range s.VetoReasons {
		if veto.Type == "hard" {
			reasons = append(reasons, fmt.Sprintf("%s: %s", veto.Module, veto.Reason))
		}
	}

	return len(reasons) == 0, reasons
}

// GetTimeUntilCopyover returns the duration until the scheduled copyover
func (s *CopyoverStatus) GetTimeUntilCopyover() time.Duration {
	if s.State != StateScheduled || s.ScheduledFor.IsZero() {
		return 0
	}

	duration := time.Until(s.ScheduledFor)
	if duration < 0 {
		return 0
	}
	return duration
}

// GetProgress returns the overall progress percentage (0-100)
func (s *CopyoverStatus) GetProgress() int {
	switch s.State {
	case StateIdle, StateScheduled, StateAnnouncing:
		return 0
	case StateBuilding:
		return s.BuildProgress / 4 // 0-25%
	case StateSaving:
		return 25 + s.SaveProgress/4 // 25-50%
	case StateGathering:
		return 50 + s.GatherProgress/4 // 50-75%
	case StateExecuting:
		return 75 // Fixed at 75% during execution
	case StateRecovering:
		return 75 + s.RestoreProgress/4 // 75-100%
	case StateFailed, StateCancelling:
		return 0
	default:
		return 0
	}
}

// CopyoverHistory represents a historical copyover record
type CopyoverHistory struct {
	ID               int           `json:"id"`
	StartedAt        time.Time     `json:"started_at"`
	CompletedAt      time.Time     `json:"completed_at"`
	Duration         time.Duration `json:"duration"`
	Success          bool          `json:"success"`
	InitiatedBy      int           `json:"initiated_by"`
	Reason           string        `json:"reason"`
	BuildNumber      string        `json:"build_number"`
	OldBuildNumber   string        `json:"old_build_number"`
	ConnectionsSaved int           `json:"connections_saved"`
	ConnectionsLost  int           `json:"connections_lost"`
	ErrorMessage     string        `json:"error_message,omitempty"`
}
