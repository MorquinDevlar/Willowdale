package events

import (
	"testing"
	"time"
)

func TestCopyoverEventTypes(t *testing.T) {
	// Test CopyoverScheduled
	t.Run("CopyoverScheduled", func(t *testing.T) {
		evt := CopyoverScheduled{
			ScheduledAt: time.Now(),
			Countdown:   300,
			Reason:      "System update",
			InitiatedBy: 1,
		}
		if evt.Type() != "CopyoverScheduled" {
			t.Errorf("Expected Type() to return 'CopyoverScheduled', got '%s'", evt.Type())
		}
	})

	// Test CopyoverStateChange
	t.Run("CopyoverStateChange", func(t *testing.T) {
		evt := CopyoverStateChange{
			OldState: "idle",
			NewState: "building",
			Progress: 50,
		}
		if evt.Type() != "CopyoverStateChange" {
			t.Errorf("Expected Type() to return 'CopyoverStateChange', got '%s'", evt.Type())
		}
	})

	// Test CopyoverGatherState
	t.Run("CopyoverGatherState", func(t *testing.T) {
		evt := CopyoverGatherState{
			Phase:        "connections",
			TotalPhases:  4,
			CurrentPhase: 2,
		}
		if evt.Type() != "CopyoverGatherState" {
			t.Errorf("Expected Type() to return 'CopyoverGatherState', got '%s'", evt.Type())
		}
	})

	// Test CopyoverRestoreState
	t.Run("CopyoverRestoreState", func(t *testing.T) {
		evt := CopyoverRestoreState{
			Phase:       "connections",
			TotalSteps:  10,
			CurrentStep: 5,
			Success:     true,
			Error:       "",
		}
		if evt.Type() != "CopyoverRestoreState" {
			t.Errorf("Expected Type() to return 'CopyoverRestoreState', got '%s'", evt.Type())
		}
	})

	// Test CopyoverCancelled
	t.Run("CopyoverCancelled", func(t *testing.T) {
		evt := CopyoverCancelled{
			Reason:      "Admin request",
			CancelledBy: 1,
		}
		if evt.Type() != "CopyoverCancelled" {
			t.Errorf("Expected Type() to return 'CopyoverCancelled', got '%s'", evt.Type())
		}
	})

	// Test CopyoverCompleted
	t.Run("CopyoverCompleted", func(t *testing.T) {
		now := time.Now()
		evt := CopyoverCompleted{
			Duration:         5 * time.Second,
			BuildNumber:      "002",
			OldBuildNumber:   "001",
			ConnectionsSaved: 10,
			ConnectionsLost:  0,
			StartTime:        now,
			EndTime:          now.Add(5 * time.Second),
		}
		if evt.Type() != "CopyoverCompleted" {
			t.Errorf("Expected Type() to return 'CopyoverCompleted', got '%s'", evt.Type())
		}
	})

	// Test CopyoverVeto
	t.Run("CopyoverVeto", func(t *testing.T) {
		evt := CopyoverVeto{
			ModuleName: "combat",
			Reason:     "Active battles in progress",
			VetoType:   "hard",
			Timestamp:  time.Now(),
		}
		if evt.Type() != "CopyoverVeto" {
			t.Errorf("Expected Type() to return 'CopyoverVeto', got '%s'", evt.Type())
		}
	})
}

// Test that events can be added to queue
func TestCopyoverEventsInQueue(t *testing.T) {
	// This tests that our events are compatible with the event queue
	evt := CopyoverScheduled{
		ScheduledAt: time.Now(),
		Countdown:   60,
		Reason:      "Test",
		InitiatedBy: 1,
	}

	// Should be able to add to queue without error
	AddToQueue(evt)
}
