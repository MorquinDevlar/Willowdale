package copyover

import (
	"testing"
	"time"
)

// mockManager creates a test manager that doesn't execute actual copyovers
func mockManager() *Manager {
	return &Manager{
		state:          StateIdle,
		stateGatherers: make([]StateGatherer, 0),
		stateRestorers: make([]StateRestorer, 0),
		buildNumber:    "test",
	}
}

// TestCopyoverAPIIntegration tests the full copyover API flow
func TestCopyoverAPIIntegration(t *testing.T) {
	mgr := mockManager()

	t.Run("InitialState", func(t *testing.T) {
		// Test initial state
		if mgr.IsInProgress() {
			t.Error("Manager should not be in progress initially")
		}

		state, progress, scheduledFor := mgr.GetStatus()
		if state != "idle" {
			t.Errorf("Expected idle state, got %s", state)
		}
		if progress != 0 {
			t.Errorf("Expected 0 progress, got %d", progress)
		}
		if !scheduledFor.IsZero() {
			t.Error("Expected zero scheduled time")
		}
	})

	t.Run("StateGathererFlow", func(t *testing.T) {
		gatherCalled := false
		mgr.RegisterStateGatherer(func() (interface{}, error) {
			gatherCalled = true
			return map[string]string{"test": "data"}, nil
		})

		// Call the gatherer
		data, err := mgr.stateGatherers[0]()
		if err != nil {
			t.Errorf("Gatherer returned error: %v", err)
		}
		if !gatherCalled {
			t.Error("Gatherer was not called")
		}
		if data.(map[string]string)["test"] != "data" {
			t.Error("Gatherer returned unexpected data")
		}
	})

	t.Run("StateRestorerFlow", func(t *testing.T) {
		restoreCalled := false
		mgr.RegisterStateRestorer(func(state *CopyoverStateData) error {
			restoreCalled = true
			return nil
		})

		// Call the restorer
		err := mgr.stateRestorers[0](&CopyoverStateData{})
		if err != nil {
			t.Errorf("Restorer returned error: %v", err)
		}
		if !restoreCalled {
			t.Error("Restorer was not called")
		}
	})

	t.Run("CopyoverOptions", func(t *testing.T) {
		// Test creating options
		opts := CopyoverOptions{
			Countdown:    30,
			IncludeBuild: true,
			Reason:       "Test copyover",
			InitiatedBy:  123,
		}

		if opts.Countdown != 30 {
			t.Error("Options not set correctly")
		}
	})

	t.Run("BuildNumber", func(t *testing.T) {
		SetBuildNumber("v1.2.3")
		if GetBuildNumber() != "v1.2.3" {
			t.Error("Build number not set correctly")
		}
	})

	t.Run("RecoveryDetection", func(t *testing.T) {
		// Test recovery detection
		t.Setenv(CopyoverEnvVar, "1")
		if !IsCopyoverRecovery() {
			t.Error("Should detect copyover recovery from env")
		}
		t.Setenv(CopyoverEnvVar, "")

		// Test isRecovering flag
		if IsRecovering() {
			t.Error("Should not be recovering initially")
		}
	})
}

// TestCopyoverStatusMethods tests the CopyoverStatus type methods
func TestCopyoverStatusMethods(t *testing.T) {
	t.Run("IsActive", func(t *testing.T) {
		tests := []struct {
			phase    CopyoverPhase
			expected bool
		}{
			{PhaseIdle, false},
			{PhaseScheduled, true},
			{PhaseBuilding, true},
			{PhaseExecuting, true},
		}

		for _, test := range tests {
			if test.phase.IsActive() != test.expected {
				t.Errorf("Phase %v: expected IsActive=%v", test.phase, test.expected)
			}
		}
	})

	t.Run("TimeUntilCopyover", func(t *testing.T) {
		status := &CopyoverStatus{}

		// Not scheduled
		if status.GetTimeUntilCopyover() != 0 {
			t.Error("Should return 0 when not scheduled")
		}

		// Scheduled in future
		status.ScheduledFor = time.Now().Add(1 * time.Hour)
		duration := status.GetTimeUntilCopyover()
		if duration < 59*time.Minute || duration > 61*time.Minute {
			t.Errorf("Expected ~1 hour, got %v", duration)
		}

		// Scheduled in past
		status.ScheduledFor = time.Now().Add(-1 * time.Hour)
		if status.GetTimeUntilCopyover() != 0 {
			t.Error("Should return 0 for past times")
		}
	})

	t.Run("CanCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State: PhaseIdle,
		}

		// Can copyover from idle
		can, reasons := status.CanCopyover()
		if !can || len(reasons) > 0 {
			t.Error("Should be able to copyover from idle")
		}

		// Cannot copyover while active
		status.State = PhaseBuilding
		can, reasons = status.CanCopyover()
		if can || len(reasons) == 0 {
			t.Error("Should not be able to copyover while active")
		}

		// Cannot copyover with hard veto
		status.State = PhaseIdle
		status.VetoReasons = []VetoInfo{
			{Module: "test", Reason: "Test veto", Type: "hard"},
		}
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover with hard veto")
		}
		if len(reasons) != 1 || reasons[0] != "Test veto" {
			t.Error("Should include veto reason")
		}
	})
}
