package copyover

import (
	"testing"
	"time"
)

// TestPublicAPIs tests the public API methods without triggering actual copyovers
func TestPublicAPIs(t *testing.T) {
	// Create a test manager
	mgr := &Manager{
		fdMap:          make(map[string]int),
		stateGatherers: make([]StateGatherer, 0),
		stateRestorers: make([]StateRestorer, 0),
		currentState:   StateIdle,
		stateChangedAt: time.Now(),
		status: &CopyoverStatus{
			State:          StateIdle,
			StateChangedAt: time.Now(),
			VetoReasons:    make([]VetoInfo, 0),
		},
		progress: make(map[string]int),
		history:  make([]CopyoverHistory, 0),
	}

	t.Run("GetStatus", func(t *testing.T) {
		status := mgr.GetStatus()
		if status == nil {
			t.Fatal("GetStatus returned nil")
		}
		if status.State != StateIdle {
			t.Errorf("Expected StateIdle, got %v", status.State)
		}

		// Modify returned status shouldn't affect manager
		status.State = StateBuilding
		if mgr.status.State != StateIdle {
			t.Error("GetStatus should return a copy")
		}
	})

	t.Run("GetHistory", func(t *testing.T) {
		// Add test history
		mgr.history = []CopyoverHistory{
			{ID: 1, Success: true},
			{ID: 2, Success: false},
			{ID: 3, Success: true},
		}

		// Test with limit
		history := mgr.GetHistory(2)
		if len(history) != 2 {
			t.Errorf("Expected 2 items, got %d", len(history))
		}

		// Should return newest first
		if history[0].ID != 3 {
			t.Error("History should be newest first")
		}

		// Test with 0 limit (all)
		history = mgr.GetHistory(0)
		if len(history) != 3 {
			t.Errorf("Expected all 3 items, got %d", len(history))
		}
	})

	t.Run("IsInProgress", func(t *testing.T) {
		if mgr.IsInProgress() {
			t.Error("Should not be in progress initially")
		}

		mgr.inProgress = true
		if !mgr.IsInProgress() {
			t.Error("Should be in progress")
		}
		mgr.inProgress = false
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
		mgr.RegisterStateRestorer(func(state *CopyoverState) error {
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

		mgr.stateRestorers[0](&CopyoverState{})
		if !restorerCalled {
			t.Error("Restorer not called")
		}
	})
}

// TestCopyoverStatusAPI tests the CopyoverStatus methods
func TestCopyoverStatusAPI(t *testing.T) {
	t.Run("CanCopyover", func(t *testing.T) {
		status := &CopyoverStatus{
			State:       StateIdle,
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
		status.State = StateBuilding
		can, reasons = status.CanCopyover()
		if can {
			t.Error("Should not be able to copyover while building")
		}
		if len(reasons) == 0 {
			t.Error("Should have reason")
		}

		// Test with veto
		status.State = StateIdle
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
			{StateIdle, 0, 0},
			{StateBuilding, 50, 12},     // 50/4
			{StateSaving, 100, 50},      // 25 + 100/4
			{StateGathering, 50, 62},    // 50 + 50/4
			{StateExecuting, 0, 75},     // Fixed
			{StateRecovering, 100, 100}, // 75 + 100/4
		}

		for _, tc := range testCases {
			status.State = tc.state
			switch tc.state {
			case StateBuilding:
				status.BuildProgress = tc.progress
			case StateSaving:
				status.SaveProgress = tc.progress
			case StateGathering:
				status.GatherProgress = tc.progress
			case StateRecovering:
				status.RestoreProgress = tc.progress
			}

			result := status.GetProgress()
			if result != tc.expected {
				t.Errorf("State %v: expected %d, got %d", tc.state, tc.expected, result)
			}
		}
	})

	t.Run("GetTimeUntilCopyover", func(t *testing.T) {
		status := &CopyoverStatus{State: StateIdle}

		// Not scheduled
		if status.GetTimeUntilCopyover() != 0 {
			t.Error("Should return 0 when not scheduled")
		}

		// Scheduled in future
		status.State = StateScheduled
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
		currentState: StateBuilding,
		status: &CopyoverStatus{
			State: StateBuilding,
		},
		progress: make(map[string]int),
	}

	// Test progress update
	mgr.updateProgress("build", 50)
	if mgr.progress["build"] != 50 {
		t.Errorf("Expected progress 50, got %d", mgr.progress["build"])
	}
	if mgr.status.BuildProgress != 50 {
		t.Errorf("Expected BuildProgress 50, got %d", mgr.status.BuildProgress)
	}

	// Test clamping
	mgr.updateProgress("build", 150)
	if mgr.progress["build"] != 100 {
		t.Error("Progress should be clamped to 100")
	}

	mgr.updateProgress("build", -50)
	if mgr.progress["build"] != 0 {
		t.Error("Progress should be clamped to 0")
	}
}

// TestHistoryManagement tests history tracking
func TestHistoryManagement(t *testing.T) {
	mgr := &Manager{
		history:        make([]CopyoverHistory, 0),
		historyCounter: 0,
		status: &CopyoverStatus{
			TotalCopyovers: 0,
		},
	}

	// Add history records
	for i := 0; i < 5; i++ {
		mgr.addToHistory(CopyoverHistory{
			Success:     i%2 == 0,
			Duration:    5 * time.Second,
			CompletedAt: time.Now(),
		})
	}

	// Check counter
	if mgr.historyCounter != 5 {
		t.Errorf("Expected counter 5, got %d", mgr.historyCounter)
	}

	// Check IDs assigned
	for i, h := range mgr.history {
		if h.ID != i+1 {
			t.Errorf("Expected ID %d, got %d", i+1, h.ID)
		}
	}

	// Check statistics
	if mgr.status.TotalCopyovers != 5 {
		t.Errorf("Expected 5 total copyovers, got %d", mgr.status.TotalCopyovers)
	}

	// Average should be 5s (all have same duration)
	if mgr.status.AverageDuration != 5*time.Second {
		t.Errorf("Expected average 5s, got %v", mgr.status.AverageDuration)
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
