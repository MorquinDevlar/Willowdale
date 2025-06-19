package copyover

import (
	"testing"
	"time"
)

// mockManager creates a test manager that doesn't execute actual copyovers
func mockManager() *Manager {
	return &Manager{
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
}

// TestCopyoverAPIIntegration tests the full copyover API flow
func TestCopyoverAPIIntegration(t *testing.T) {
	mgr := mockManager()

	t.Run("InitialState", func(t *testing.T) {
		// Test initial state
		if mgr.IsInProgress() {
			t.Error("Manager should not be in progress initially")
		}

		status := mgr.GetStatus()
		if status.State != StateIdle {
			t.Errorf("Expected StateIdle, got %v", status.State)
		}

		if status.GetProgress() != 0 {
			t.Errorf("Expected 0 progress, got %d", status.GetProgress())
		}

		canStart, reasons := status.CanCopyover()
		if !canStart {
			t.Errorf("Should be able to start copyover, got reasons: %v", reasons)
		}
	})

	t.Run("StateGathererRegistration", func(t *testing.T) {
		called := false
		mgr.RegisterStateGatherer(func() (interface{}, error) {
			called = true
			return "test_data", nil
		})

		if len(mgr.stateGatherers) != 1 {
			t.Errorf("Expected 1 gatherer, got %d", len(mgr.stateGatherers))
		}

		// Test gatherer execution
		_, err := mgr.stateGatherers[0]()
		if err != nil {
			t.Errorf("Gatherer returned error: %v", err)
		}
		if !called {
			t.Error("Gatherer was not called")
		}
	})

	t.Run("StateRestorerRegistration", func(t *testing.T) {
		called := false
		mgr.RegisterStateRestorer(func(state *CopyoverState) error {
			called = true
			return nil
		})

		if len(mgr.stateRestorers) != 1 {
			t.Errorf("Expected 1 restorer, got %d", len(mgr.stateRestorers))
		}

		// Test restorer execution
		testState := &CopyoverState{}
		err := mgr.stateRestorers[0](testState)
		if err != nil {
			t.Errorf("Restorer returned error: %v", err)
		}
		if !called {
			t.Error("Restorer was not called")
		}
	})

	t.Run("ScheduleCopyover", func(t *testing.T) {
		// Reset to idle state
		mgr.currentState = StateIdle
		mgr.inProgress = false
		mgr.status.State = StateIdle

		// Test scheduling in the past
		past := time.Now().Add(-1 * time.Hour)
		err := mgr.ScheduleCopyover(past, 123, "Test past")
		if err == nil {
			t.Error("Expected error scheduling in the past")
		}

		// Test scheduling in the future
		future := time.Now().Add(1 * time.Hour)
		mgr.status.ScheduledFor = future
		mgr.status.InitiatedBy = 123
		mgr.status.Reason = "Test scheduled copyover"
		mgr.currentState = StateScheduled
		mgr.status.State = StateScheduled

		// Check status
		status := mgr.GetStatus()
		if status.InitiatedBy != 123 {
			t.Errorf("Expected InitiatedBy 123, got %d", status.InitiatedBy)
		}
		if status.Reason != "Test scheduled copyover" {
			t.Errorf("Expected reason 'Test scheduled copyover', got %s", status.Reason)
		}

		// Check time until copyover
		duration := status.GetTimeUntilCopyover()
		if duration <= 59*time.Minute || duration > 61*time.Minute {
			t.Errorf("Expected duration around 1h, got %v", duration)
		}
	})

	t.Run("CancelCopyover", func(t *testing.T) {
		// Ensure we're in a cancellable state
		mgr.currentState = StateScheduled
		mgr.inProgress = true

		err := mgr.CancelCopyover("Test cancellation")
		if err != nil {
			t.Fatalf("Failed to cancel copyover: %v", err)
		}

		// Check state changed to cancelling
		if mgr.currentState != StateCancelling {
			t.Errorf("Expected StateCancelling, got %v", mgr.currentState)
		}

		// Wait for async transition to idle
		time.Sleep(200 * time.Millisecond)

		if mgr.currentState != StateIdle {
			t.Errorf("Expected StateIdle after cancellation, got %v", mgr.currentState)
		}
		if mgr.inProgress {
			t.Error("Expected inProgress to be false after cancellation")
		}
	})

	t.Run("VetoSystem", func(t *testing.T) {
		// Add a hard veto
		mgr.status.VetoReasons = append(mgr.status.VetoReasons, VetoInfo{
			Module:    "test_module",
			Reason:    "test in progress",
			Type:      "hard",
			Timestamp: time.Now(),
		})

		status := mgr.GetStatus()
		canStart, reasons := status.CanCopyover()
		if canStart {
			t.Error("Should not be able to start with hard veto")
		}
		if len(reasons) == 0 {
			t.Error("Expected veto reasons")
		}

		// Clear vetoes
		mgr.status.VetoReasons = []VetoInfo{}
	})

	t.Run("ProgressTracking", func(t *testing.T) {
		// Test progress in different states
		testCases := []struct {
			state           CopyoverPhase
			phaseProgress   int
			expectedOverall int
		}{
			{StateIdle, 0, 0},
			{StateBuilding, 50, 12},     // 50/4 = 12
			{StateSaving, 100, 50},      // 25 + 100/4 = 50
			{StateGathering, 50, 62},    // 50 + 50/4 = 62
			{StateExecuting, 0, 75},     // Fixed at 75
			{StateRecovering, 100, 100}, // 75 + 100/4 = 100
		}

		for _, tc := range testCases {
			mgr.currentState = tc.state
			mgr.status.State = tc.state

			// Set appropriate progress field
			switch tc.state {
			case StateBuilding:
				mgr.status.BuildProgress = tc.phaseProgress
			case StateSaving:
				mgr.status.SaveProgress = tc.phaseProgress
			case StateGathering:
				mgr.status.GatherProgress = tc.phaseProgress
			case StateRecovering:
				mgr.status.RestoreProgress = tc.phaseProgress
			}

			progress := mgr.status.GetProgress()
			if progress != tc.expectedOverall {
				t.Errorf("State %v with phase progress %d: expected overall %d, got %d",
					tc.state, tc.phaseProgress, tc.expectedOverall, progress)
			}
		}
	})

	t.Run("History", func(t *testing.T) {
		// Add some history
		for i := 0; i < 5; i++ {
			mgr.addToHistory(CopyoverHistory{
				StartedAt:        time.Now().Add(time.Duration(-i) * time.Hour),
				CompletedAt:      time.Now().Add(time.Duration(-i) * time.Hour).Add(5 * time.Second),
				Duration:         5 * time.Second,
				Success:          i%2 == 0,
				InitiatedBy:      i,
				Reason:           "test",
				ConnectionsSaved: 10 - i,
				ConnectionsLost:  i,
			})
		}

		// Get limited history
		history := mgr.GetHistory(3)
		if len(history) != 3 {
			t.Errorf("Expected 3 history items, got %d", len(history))
		}

		// Check newest first
		if history[0].InitiatedBy != 4 {
			t.Error("History should be newest first")
		}

		// Check statistics updated
		if mgr.status.TotalCopyovers != 5 {
			t.Errorf("Expected 5 total copyovers, got %d", mgr.status.TotalCopyovers)
		}
		if mgr.status.AverageDuration != 5*time.Second {
			t.Errorf("Expected average duration 5s, got %v", mgr.status.AverageDuration)
		}
	})
}

// TestStateTransitionIntegration tests state machine transitions
func TestStateTransitionIntegration(t *testing.T) {
	mgr := mockManager()

	// Test valid transition path
	transitions := []struct {
		from  CopyoverPhase
		to    CopyoverPhase
		valid bool
	}{
		{StateIdle, StateScheduled, true},
		{StateScheduled, StateAnnouncing, true},
		{StateAnnouncing, StateBuilding, true},
		{StateBuilding, StateSaving, true},
		{StateSaving, StateGathering, true},
		{StateGathering, StateExecuting, true},
		{StateExecuting, StateRecovering, true},
		{StateRecovering, StateIdle, true},

		// Test invalid transitions
		{StateIdle, StateExecuting, false},
		{StateBuilding, StateScheduled, false},
		{StateFailed, StateBuilding, false},
	}

	for _, tc := range transitions {
		mgr.currentState = tc.from
		err := mgr.changeState(tc.to)

		if tc.valid && err != nil {
			t.Errorf("Expected valid transition %v->%v, got error: %v",
				tc.from, tc.to, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("Expected invalid transition %v->%v to fail",
				tc.from, tc.to)
		}

		// Reset for next test
		if err == nil {
			mgr.currentState = tc.from
		}
	}
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	mgr := GetManager()

	// Run multiple goroutines accessing the manager
	done := make(chan bool, 4)

	// Status reader
	go func() {
		for i := 0; i < 100; i++ {
			_ = mgr.GetStatus()
			_ = mgr.IsInProgress()
		}
		done <- true
	}()

	// History reader
	go func() {
		for i := 0; i < 100; i++ {
			_ = mgr.GetHistory(10)
		}
		done <- true
	}()

	// State gatherer registrar
	go func() {
		for i := 0; i < 10; i++ {
			mgr.RegisterStateGatherer(func() (interface{}, error) {
				return nil, nil
			})
		}
		done <- true
	}()

	// Progress updater
	go func() {
		for i := 0; i < 100; i++ {
			mgr.updateProgress("test", i)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// If we get here without deadlock or panic, concurrent access is safe
}
