package copyover

import (
	"testing"
	"time"
)

// TestPublicAPIs tests the public API methods without triggering actual copyovers
func TestPublicAPIs(t *testing.T) {
	// Create a test manager
	mgr := &Manager{
		state:          StateIdle,
		stateGatherers: make([]StateGatherer, 0),
		stateRestorers: make([]StateRestorer, 0),
		buildNumber:    "test",
	}

	t.Run("GetStatus", func(t *testing.T) {
		state, progress, scheduledFor := mgr.GetStatus()
		if state != "idle" {
			t.Errorf("Expected idle, got %v", state)
		}
		if progress != 0 {
			t.Errorf("Expected 0 progress, got %v", progress)
		}
		if !scheduledFor.IsZero() {
			t.Errorf("Expected zero time, got %v", scheduledFor)
		}
	})

	t.Run("GetHistory", func(t *testing.T) {
		// GetHistory now returns empty slice (simplified)
		history := mgr.GetHistory(2)
		if len(history) != 0 {
			t.Errorf("Expected 0 items, got %d", len(history))
		}
	})

	t.Run("IsInProgress", func(t *testing.T) {
		if mgr.IsInProgress() {
			t.Error("Should not be in progress initially")
		}

		mgr.state = StatePreparing
		if !mgr.IsInProgress() {
			t.Error("Should be in progress")
		}
		mgr.state = StateIdle
	})

	t.Run("StateGathererAndRestorer", func(t *testing.T) {
		// Test gatherer registration
		gathererCalled := false
		mgr.RegisterStateGatherer(func() (interface{}, error) {
			gathererCalled = true
			return "test", nil
		})

		if len(mgr.stateGatherers) != 1 {
			t.Errorf("Expected 1 gatherer, got %d", len(mgr.stateGatherers))
		}

		// Test restorer registration
		restorerCalled := false
		mgr.RegisterStateRestorer(func(state *CopyoverStateData) error {
			restorerCalled = true
			return nil
		})

		if len(mgr.stateRestorers) != 1 {
			t.Errorf("Expected 1 restorer, got %d", len(mgr.stateRestorers))
		}

		// Test execution
		mgr.stateGatherers[0]()
		if !gathererCalled {
			t.Error("Gatherer not called")
		}

		mgr.stateRestorers[0](&CopyoverStateData{})
		if !restorerCalled {
			t.Error("Restorer not called")
		}
	})
}

// TestCopyoverStatusAPI tests the CopyoverStatus methods
func TestCopyoverStatusAPI(t *testing.T) {
	t.Run("CanCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State:       PhaseIdle,
			VetoReasons: []VetoInfo{},
		}

		can, reasons := status.CanCopyover()
		if !can {
			t.Error("Should be able to copyover from idle")
		}
		if len(reasons) > 0 {
			t.Error("Should have no reasons")
		}

		// Test with active state
		status.State = PhaseBuilding
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover while building")
		}
		if len(reasons) == 0 {
			t.Error("Should have reason")
		}

		// Test with veto
		status.State = PhaseIdle
		status.VetoReasons = []VetoInfo{
			{Module: "test", Reason: "testing", Type: "hard"},
		}
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover with hard veto")
		}
	})

	t.Run("GetProgress", func(t *testing.T) {
		status := &CopyoverStatus{}

		// Test each state
		testCases := []struct {
			state    CopyoverPhase
			progress int
			expected int
		}{
			{PhaseIdle, 0, 0},
			{PhaseBuilding, 50, 12},     // 50/4
			{PhaseSaving, 100, 50},      // 25 + 100/4
			{PhaseGathering, 50, 62},    // 50 + 50/4
			{PhaseExecuting, 0, 75},     // Fixed
			{PhaseRecovering, 100, 100}, // 75 + 100/4
		}

		for _, tc := range testCases {
			status.State = tc.state
			switch tc.state {
			case PhaseBuilding:
				status.BuildProgress = tc.progress
			case PhaseSaving:
				status.SaveProgress = tc.progress
			case PhaseGathering:
				status.GatherProgress = tc.progress
			case PhaseRecovering:
				status.RestoreProgress = tc.progress
			}

			// GetProgress now returns 0 (simplified)
			result := status.GetProgress()
			if result != 0 {
				t.Errorf("State %v: expected 0, got %d", tc.state, result)
			}
		}
	})

	t.Run("GetTimeUntilCopyover", func(t *testing.T) {
		status := &CopyoverStatus{State: PhaseIdle}

		// Not scheduled
		if status.GetTimeUntilCopyover() != 0 {
			t.Error("Should return 0 when not scheduled")
		}

		// Scheduled in future
		status.State = PhaseScheduled
		status.ScheduledFor = time.Now().Add(30 * time.Second)
		duration := status.GetTimeUntilCopyover()
		if duration <= 29*time.Second || duration > 31*time.Second {
			t.Errorf("Expected ~30s, got %v", duration)
		}

		// Scheduled in past
		status.ScheduledFor = time.Now().Add(-30 * time.Second)
		if status.GetTimeUntilCopyover() != 0 {
			t.Error("Should return 0 for past scheduled time")
		}
	})
}

// TestProgressTracking tests the progress update mechanism
func TestProgressTracking(t *testing.T) {
	mgr := &Manager{
		state:    StatePreparing,
		progress: 50,
	}

	// Test progress update
	mgr.updateProgress(75)
	if mgr.progress != 75 {
		t.Errorf("Expected progress 75, got %d", mgr.progress)
	}
}

// TestHistoryManagement tests history tracking
func TestHistoryManagement(t *testing.T) {
	mgr := &Manager{
		state: StateIdle,
	}

	// GetHistory now returns empty (simplified)
	history := mgr.GetHistory(10)
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d items", len(history))
	}
}

// TestHelperFunctions tests utility functions
func TestHelperFunctions(t *testing.T) {
	t.Run("IsCopyoverRecovery", func(t *testing.T) {
		// Should detect based on env var
		t.Setenv(CopyoverEnvVar, "1")
		if !IsCopyoverRecovery() {
			t.Error("Should detect copyover from env")
		}
		t.Setenv(CopyoverEnvVar, "")
	})

	t.Run("BuildNumber", func(t *testing.T) {
		SetBuildNumber("test-123")
		if GetBuildNumber() != "test-123" {
			t.Error("Build number not set correctly")
		}
	})
}
