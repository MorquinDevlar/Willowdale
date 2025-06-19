package copyover

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Mock module for testing
type mockModule struct {
	name          string
	state         map[string]interface{}
	canCopyover   bool
	vetoReason    string
	vetoType      string
	gatherCalled  bool
	restoreCalled bool
	prepareCalled bool
	cleanupCalled bool
	gatherError   error
	restoreError  error
	mu            sync.Mutex
}

func newMockModule(name string) *mockModule {
	return &mockModule{
		name:        name,
		state:       make(map[string]interface{}),
		canCopyover: true,
	}
}

func (m *mockModule) ModuleName() string {
	return m.name
}

func (m *mockModule) GatherState() (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gatherCalled = true
	if m.gatherError != nil {
		return nil, m.gatherError
	}

	// Return a copy of state
	stateCopy := make(map[string]interface{})
	for k, v := range m.state {
		stateCopy[k] = v
	}
	return stateCopy, nil
}

func (m *mockModule) RestoreState(data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.restoreCalled = true
	if m.restoreError != nil {
		return m.restoreError
	}

	if restored, ok := data.(map[string]interface{}); ok {
		m.state = restored
	}
	return nil
}

func (m *mockModule) CanCopyover() (bool, VetoInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.canCopyover {
		return true, VetoInfo{}
	}

	return false, VetoInfo{
		Module: m.name,
		Reason: m.vetoReason,
		Type:   m.vetoType,
	}
}

func (m *mockModule) PrepareCopyover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prepareCalled = true
	return nil
}

func (m *mockModule) CleanupCopyover() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupCalled = true
	return nil
}

func TestModuleRegistration(t *testing.T) {
	// Reset registry for testing
	moduleRegistry = &ModuleRegistry{
		participants: make(map[string]ModuleParticipant),
		states:       make(map[string]interface{}),
	}

	t.Run("RegisterModule", func(t *testing.T) {
		module := newMockModule("test_module")

		// Test successful registration
		err := RegisterModule(module)
		if err != nil {
			t.Fatalf("Failed to register module: %v", err)
		}

		// Test duplicate registration
		err = RegisterModule(module)
		if err == nil {
			t.Error("Expected error for duplicate registration")
		}

		// Test empty name
		emptyModule := newMockModule("")
		err = RegisterModule(emptyModule)
		if err == nil {
			t.Error("Expected error for empty module name")
		}
	})

	t.Run("UnregisterModule", func(t *testing.T) {
		module := newMockModule("test_unregister")
		RegisterModule(module)

		// Verify registered
		modules := GetRegisteredModules()
		found := false
		for _, name := range modules {
			if name == "test_unregister" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Module not found after registration")
		}

		// Unregister
		UnregisterModule("test_unregister")

		// Verify unregistered
		modules = GetRegisteredModules()
		for _, name := range modules {
			if name == "test_unregister" {
				t.Error("Module still found after unregistration")
			}
		}
	})

	t.Run("GetRegisteredModules", func(t *testing.T) {
		// Clear registry
		moduleRegistry.participants = make(map[string]ModuleParticipant)

		// Register multiple modules
		for i := 0; i < 3; i++ {
			module := newMockModule(fmt.Sprintf("module_%d", i))
			RegisterModule(module)
		}

		modules := GetRegisteredModules()
		if len(modules) != 3 {
			t.Errorf("Expected 3 modules, got %d", len(modules))
		}
	})
}

func TestModuleVetoes(t *testing.T) {
	// Reset registry
	moduleRegistry = &ModuleRegistry{
		participants: make(map[string]ModuleParticipant),
		states:       make(map[string]interface{}),
	}

	t.Run("NoVetoes", func(t *testing.T) {
		module1 := newMockModule("module1")
		module2 := newMockModule("module2")

		RegisterModule(module1)
		RegisterModule(module2)

		canProceed, vetoes := CheckModuleVetoes()
		if !canProceed {
			t.Error("Expected to proceed with no vetoes")
		}
		if len(vetoes) != 0 {
			t.Errorf("Expected no vetoes, got %d", len(vetoes))
		}
	})

	t.Run("SoftVeto", func(t *testing.T) {
		module := newMockModule("soft_veto_module")
		module.canCopyover = false
		module.vetoReason = "Warning: High load"
		module.vetoType = "soft"

		RegisterModule(module)
		defer UnregisterModule(module.name)

		canProceed, vetoes := CheckModuleVetoes()
		if !canProceed {
			t.Error("Soft veto should not block copyover")
		}
		if len(vetoes) != 1 {
			t.Errorf("Expected 1 veto, got %d", len(vetoes))
		}
		if vetoes[0].Type != "soft" {
			t.Errorf("Expected soft veto, got %s", vetoes[0].Type)
		}
	})

	t.Run("HardVeto", func(t *testing.T) {
		module := newMockModule("hard_veto_module")
		module.canCopyover = false
		module.vetoReason = "Critical operation in progress"
		module.vetoType = "hard"

		RegisterModule(module)
		defer UnregisterModule(module.name)

		canProceed, vetoes := CheckModuleVetoes()
		if canProceed {
			t.Error("Hard veto should block copyover")
		}
		if len(vetoes) == 0 {
			t.Error("Expected veto info")
		}
	})

	t.Run("MixedVetoes", func(t *testing.T) {
		// Clear registry
		moduleRegistry.participants = make(map[string]ModuleParticipant)

		// Register modules with different veto types
		softModule := newMockModule("soft_module")
		softModule.canCopyover = false
		softModule.vetoType = "soft"

		hardModule := newMockModule("hard_module")
		hardModule.canCopyover = false
		hardModule.vetoType = "hard"

		okModule := newMockModule("ok_module")

		RegisterModule(softModule)
		RegisterModule(hardModule)
		RegisterModule(okModule)

		canProceed, vetoes := CheckModuleVetoes()
		if canProceed {
			t.Error("Should be blocked by hard veto")
		}
		if len(vetoes) != 2 {
			t.Errorf("Expected 2 vetoes, got %d", len(vetoes))
		}
	})
}

func TestModuleStateManagement(t *testing.T) {
	// Reset registry
	moduleRegistry = &ModuleRegistry{
		participants: make(map[string]ModuleParticipant),
		states:       make(map[string]interface{}),
	}

	t.Run("GatherStates", func(t *testing.T) {
		module1 := newMockModule("gather_module1")
		module1.state["key"] = "value1"

		module2 := newMockModule("gather_module2")
		module2.state["key"] = "value2"

		RegisterModule(module1)
		RegisterModule(module2)

		err := GatherModuleStates()
		if err != nil {
			t.Fatalf("Failed to gather states: %v", err)
		}

		// Verify gather was called
		if !module1.gatherCalled || !module2.gatherCalled {
			t.Error("GatherState not called on all modules")
		}

		// Verify states were stored
		if len(moduleRegistry.states) != 2 {
			t.Errorf("Expected 2 states, got %d", len(moduleRegistry.states))
		}
	})

	t.Run("GatherStateError", func(t *testing.T) {
		module := newMockModule("error_module")
		module.gatherError = fmt.Errorf("gather failed")

		RegisterModule(module)
		defer UnregisterModule(module.name)

		// Should not fail even if module fails
		err := GatherModuleStates()
		if err != nil {
			t.Error("GatherModuleStates should not fail on module error")
		}

		// State should not be stored for failed module
		if _, exists := moduleRegistry.states[module.name]; exists {
			t.Error("State should not be stored for failed module")
		}
	})

	t.Run("RestoreStates", func(t *testing.T) {
		// Clear and setup
		moduleRegistry.participants = make(map[string]ModuleParticipant)
		moduleRegistry.states = make(map[string]interface{})

		module := newMockModule("restore_module")
		RegisterModule(module)

		// Set saved state
		savedState := map[string]interface{}{"restored": true}
		moduleRegistry.states[module.name] = savedState

		err := RestoreModuleStates()
		if err != nil {
			t.Fatalf("Failed to restore states: %v", err)
		}

		if !module.restoreCalled {
			t.Error("RestoreState not called")
		}

		if !module.state["restored"].(bool) {
			t.Error("State not properly restored")
		}
	})

	t.Run("RestoreStateError", func(t *testing.T) {
		module := newMockModule("restore_error_module")
		module.restoreError = fmt.Errorf("restore failed")

		RegisterModule(module)
		defer UnregisterModule(module.name)

		moduleRegistry.states[module.name] = map[string]interface{}{}

		err := RestoreModuleStates()
		if err == nil {
			t.Error("Expected error from failed restore")
		}
	})
}

func TestModuleLifecycle(t *testing.T) {
	// Reset registry
	moduleRegistry = &ModuleRegistry{
		participants: make(map[string]ModuleParticipant),
		states:       make(map[string]interface{}),
	}

	t.Run("PrepareModules", func(t *testing.T) {
		module1 := newMockModule("prepare_module1")
		module2 := newMockModule("prepare_module2")

		RegisterModule(module1)
		RegisterModule(module2)

		err := PrepareModulesForCopyover()
		if err != nil {
			t.Fatalf("Failed to prepare modules: %v", err)
		}

		if !module1.prepareCalled || !module2.prepareCalled {
			t.Error("PrepareCopyover not called on all modules")
		}
	})

	t.Run("CleanupModules", func(t *testing.T) {
		module1 := newMockModule("cleanup_module1")
		module2 := newMockModule("cleanup_module2")

		RegisterModule(module1)
		RegisterModule(module2)

		err := CleanupModulesAfterCopyover()
		if err != nil {
			t.Fatalf("Failed to cleanup modules: %v", err)
		}

		if !module1.cleanupCalled || !module2.cleanupCalled {
			t.Error("CleanupCopyover not called on all modules")
		}
	})
}

func TestModuleConcurrency(t *testing.T) {
	// Reset registry
	moduleRegistry = &ModuleRegistry{
		participants: make(map[string]ModuleParticipant),
		states:       make(map[string]interface{}),
	}

	// Register multiple modules
	for i := 0; i < 10; i++ {
		module := newMockModule(fmt.Sprintf("concurrent_%d", i))
		RegisterModule(module)
	}

	// Run concurrent operations
	done := make(chan bool, 4)

	// Concurrent veto checks
	go func() {
		for i := 0; i < 100; i++ {
			CheckModuleVetoes()
		}
		done <- true
	}()

	// Concurrent state gathering
	go func() {
		for i := 0; i < 50; i++ {
			GatherModuleStates()
		}
		done <- true
	}()

	// Concurrent registration
	go func() {
		for i := 0; i < 20; i++ {
			module := newMockModule(fmt.Sprintf("dynamic_%d", i))
			RegisterModule(module)
			time.Sleep(time.Millisecond)
			UnregisterModule(module.name)
		}
		done <- true
	}()

	// Concurrent module listing
	go func() {
		for i := 0; i < 100; i++ {
			GetRegisteredModules()
		}
		done <- true
	}()

	// Wait for completion
	for i := 0; i < 4; i++ {
		<-done
	}

	// If we get here without deadlock or panic, concurrency is handled
}
