package copyover

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestCopyoverPhaseSerialization(t *testing.T) {
	// Create a sample state
	state := &CopyoverState{
		Version:   "1.0",
		Timestamp: time.Now(),
		Environment: map[string]string{
			"CONFIG_PATH": "/path/to/config.yaml",
			"LOG_LEVEL":   "HIGH",
		},
		Listeners: map[string]ListenerState{
			"telnet": {
				Type:    "telnet",
				Address: ":1111",
				FD:      3,
			},
			"websocket": {
				Type:    "websocket",
				Address: ":80",
				FD:      4,
			},
		},
		Connections: []ConnectionState{
			{
				ConnectionID: 12345,
				Type:         "telnet",
				FD:           5,
				RemoteAddr:   "192.168.1.100:54321",
				ConnectedAt:  time.Now().Add(-5 * time.Minute),
				UserID:       1,
				RoomID:       100,
			},
		},
		GameState: GameSnapshot{
			CurrentRound: 12345,
			GameTime:     time.Now(),
			ActiveCombats: []CombatState{
				{
					RoomID:     100,
					Combatants: []int{1, 2},
					RoundCount: 3,
				},
			},
		},
	}

	// Test serialization
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	t.Logf("Serialized state (%d bytes):\n%s", len(data), string(data))

	// Test deserialization
	var loaded CopyoverState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify key fields
	if loaded.Version != state.Version {
		t.Errorf("Version mismatch: got %s, want %s", loaded.Version, state.Version)
	}

	if len(loaded.Listeners) != len(state.Listeners) {
		t.Errorf("Listener count mismatch: got %d, want %d", len(loaded.Listeners), len(state.Listeners))
	}

	if len(loaded.Connections) != len(state.Connections) {
		t.Errorf("Connection count mismatch: got %d, want %d", len(loaded.Connections), len(state.Connections))
	}

	// Test file operations
	tempFile := "test_copyover.dat"
	defer os.Remove(tempFile)

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	readData, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(readData) != len(data) {
		t.Errorf("File size mismatch: got %d, want %d", len(readData), len(data))
	}
}

func TestManagerBasics(t *testing.T) {
	mgr := &Manager{
		fdMap:          make(map[string]int),
		stateGatherers: make([]StateGatherer, 0),
		stateRestorers: make([]StateRestorer, 0),
	}

	// Test initial state
	if mgr.IsInProgress() {
		t.Error("Manager should not be in progress initially")
	}

	// Test gatherer registration
	mgr.RegisterStateGatherer(func() (interface{}, error) {
		return nil, nil
	})

	if len(mgr.stateGatherers) != 1 {
		t.Errorf("Expected 1 gatherer, got %d", len(mgr.stateGatherers))
	}

	// Test restorer registration
	mgr.RegisterStateRestorer(func(state *CopyoverState) error {
		return nil
	})

	if len(mgr.stateRestorers) != 1 {
		t.Errorf("Expected 1 restorer, got %d", len(mgr.stateRestorers))
	}
}

func TestCopyoverDetection(t *testing.T) {
	// Test environment variable detection
	os.Setenv(CopyoverEnvVar, "1")
	defer os.Unsetenv(CopyoverEnvVar)

	if !IsCopyoverRecovery() {
		t.Error("Should detect copyover via environment variable")
	}

	os.Unsetenv(CopyoverEnvVar)

	// Test file detection
	tempFile := CopyoverDataFile
	defer os.Remove(tempFile)

	if IsCopyoverRecovery() {
		t.Error("Should not detect copyover without file")
	}

	// Create file
	os.WriteFile(tempFile, []byte("{}"), 0600)

	if !IsCopyoverRecovery() {
		t.Error("Should detect copyover via file")
	}
}
