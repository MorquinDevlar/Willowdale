package copyover

import (
	"testing"
	"time"
)

func TestCopyoverPhaseTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     CopyoverPhase
		to       CopyoverPhase
		expected bool
	}{
		// Valid transitions from Idle
		{"Idle->Scheduled", StateIdle, StateScheduled, true},
		{"Idle->Building", StateIdle, StateBuilding, true},
		{"Idle->Invalid", StateIdle, StateGathering, false},

		// Valid transitions from Scheduled
		{"Scheduled->Announcing", StateScheduled, StateAnnouncing, true},
		{"Scheduled->Cancelling", StateScheduled, StateCancelling, true},
		{"Scheduled->Invalid", StateScheduled, StateBuilding, false},

		// Valid transitions from Building
		{"Building->Saving", StateBuilding, StateSaving, true},
		{"Building->Failed", StateBuilding, StateFailed, true},
		{"Building->Cancelling", StateBuilding, StateCancelling, true},
		{"Building->Invalid", StateBuilding, StateIdle, false},

		// Valid transitions from Failed
		{"Failed->Idle", StateFailed, StateIdle, true},
		{"Failed->Invalid", StateFailed, StateBuilding, false},

		// Terminal states
		{"Recovering->Idle", StateRecovering, StateIdle, true},
		{"Recovering->Failed", StateRecovering, StateFailed, true},
		{"Cancelling->Idle", StateCancelling, StateIdle, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.from.CanTransitionTo(tt.to)
			if result != tt.expected {
				t.Errorf("CanTransitionTo(%s->%s) = %v, want %v",
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestCopyoverPhaseString(t *testing.T) {
	tests := []struct {
		state    CopyoverPhase
		expected string
	}{
		{StateIdle, "idle"},
		{StateScheduled, "scheduled"},
		{StateAnnouncing, "announcing"},
		{StateBuilding, "building"},
		{StateSaving, "saving"},
		{StateGathering, "gathering"},
		{StateExecuting, "executing"},
		{StateRecovering, "recovering"},
		{StateCancelling, "cancelling"},
		{StateFailed, "failed"},
		{CopyoverPhase(999), "unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCopyoverPhaseProperties(t *testing.T) {
	t.Run("IsTerminal", func(t *testing.T) {
		if !StateIdle.IsTerminal() {
			t.Error("StateIdle should be terminal")
		}
		if !StateFailed.IsTerminal() {
			t.Error("StateFailed should be terminal")
		}
		if StateBuilding.IsTerminal() {
			t.Error("StateBuilding should not be terminal")
		}
	})

	t.Run("IsActive", func(t *testing.T) {
		if StateIdle.IsActive() {
			t.Error("StateIdle should not be active")
		}
		if StateFailed.IsActive() {
			t.Error("StateFailed should not be active")
		}
		if !StateBuilding.IsActive() {
			t.Error("StateBuilding should be active")
		}
		if !StateRecovering.IsActive() {
			t.Error("StateRecovering should be active")
		}
	})
}

func TestCopyoverStatus(t *testing.T) {
	t.Run("CanCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State:       StateIdle,
			VetoReasons: []VetoInfo{},
		}

		can, reasons := status.CanCopyover()
		if !can {
			t.Error("Should be able to copyover from idle state")
		}
		if len(reasons) > 0 {
			t.Error("Should have no reasons preventing copyover")
		}

		// Test with active state
		status.State = StateBuilding
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover while building")
		}
		if len(reasons) == 0 {
			t.Error("Should have reason for preventing copyover")
		}

		// Test with veto
		status.State = StateIdle
		status.VetoReasons = []VetoInfo{
			{Module: "combat", Reason: "battle in progress", Type: "hard"},
		}
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover with hard veto")
		}
	})

	t.Run("GetTimeUntilCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State:        StateScheduled,
			ScheduledFor: time.Now().Add(30 * time.Second),
		}

		duration := status.GetTimeUntilCopyover()
		if duration <= 29*time.Second || duration > 31*time.Second {
			t.Errorf("Expected duration around 30s, got %v", duration)
		}

		// Test with non-scheduled state
		status.State = StateIdle
		duration = status.GetTimeUntilCopyover()
		if duration != 0 {
			t.Error("Should return 0 for non-scheduled state")
		}
	})

	t.Run("GetProgress", func(t *testing.T) {
		status := &CopyoverStatus{State: StateIdle}
		if status.GetProgress() != 0 {
			t.Error("Idle should have 0 progress")
		}

		status.State = StateBuilding
		status.BuildProgress = 50
		if status.GetProgress() != 12 { // 50/4 = 12.5 -> 12
			t.Errorf("Building at 50%% should be 12%% overall, got %d", status.GetProgress())
		}

		status.State = StateSaving
		status.SaveProgress = 100
		if status.GetProgress() != 50 { // 25 + 100/4 = 50
			t.Errorf("Saving at 100%% should be 50%% overall, got %d", status.GetProgress())
		}
	})
}

func TestCopyoverHistory(t *testing.T) {
	history := CopyoverHistory{
		StartedAt:        time.Now(),
		CompletedAt:      time.Now().Add(5 * time.Second),
		Duration:         5 * time.Second,
		Success:          true,
		InitiatedBy:      1,
		Reason:           "test",
		BuildNumber:      "123",
		OldBuildNumber:   "122",
		ConnectionsSaved: 10,
		ConnectionsLost:  0,
	}

	if history.Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", history.Duration)
	}

	if !history.Success {
		t.Error("Expected success to be true")
	}
}
