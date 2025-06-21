package copyover

import (
	"testing"
	"time"
)

func TestCopyoverPhaseProperties(t *testing.T) {
	t.Run("IsActive", func(t *testing.T) {
		if PhaseIdle.IsActive() {
			t.Error("PhaseIdle should not be active")
		}
		if !PhaseScheduled.IsActive() {
			t.Error("PhaseScheduled should be active")
		}
		if !PhaseBuilding.IsActive() {
			t.Error("PhaseBuilding should be active")
		}
		if !PhaseRecovering.IsActive() {
			t.Error("PhaseRecovering should be active")
		}
	})
}

func TestCopyoverStatus(t *testing.T) {
	t.Run("CanCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State:       PhaseIdle,
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
		status.State = PhaseBuilding
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover while building")
		}
		if len(reasons) == 0 {
			t.Error("Should have reason for preventing copyover")
		}

		// Test with veto
		status.State = PhaseIdle
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
			State:        PhaseScheduled,
			ScheduledFor: time.Now().Add(30 * time.Second),
		}

		duration := status.GetTimeUntilCopyover()
		if duration <= 29*time.Second || duration > 31*time.Second {
			t.Errorf("Expected duration around 30s, got %v", duration)
		}

		// Test with non-scheduled state
		status.State = PhaseIdle
		duration = status.GetTimeUntilCopyover()
		if duration != 0 {
			t.Error("Should return 0 for non-scheduled state")
		}
	})

	t.Run("GetProgress", func(t *testing.T) {
		status := &CopyoverStatus{State: PhaseIdle}
		// GetProgress now returns 0 (simplified)
		if status.GetProgress() != 0 {
			t.Error("GetProgress should return 0")
		}

		status.State = PhaseBuilding
		status.BuildProgress = 50
		if status.GetProgress() != 0 {
			t.Errorf("GetProgress should return 0, got %d", status.GetProgress())
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
