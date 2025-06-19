package copyover

import (
	"fmt"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// ModuleParticipant defines the interface for modules that participate in copyover
type ModuleParticipant interface {
	// ModuleName returns the unique name of this module
	ModuleName() string

	// GatherState is called before copyover to collect module state
	// The returned data will be passed to RestoreState after copyover
	GatherState() (interface{}, error)

	// RestoreState is called after copyover to restore module state
	// The data parameter contains what was returned from GatherState
	RestoreState(data interface{}) error

	// CanCopyover checks if the module is ready for copyover
	// Returns true if ready, or false with a veto reason
	CanCopyover() (bool, VetoInfo)

	// PrepareCopyover is called when copyover is imminent
	// Modules should pause operations and prepare for restart
	PrepareCopyover() error

	// CleanupCopyover is called if copyover fails or is cancelled
	// Modules should resume normal operations
	CleanupCopyover() error
}

// ModuleRegistry manages module participation in copyover
type ModuleRegistry struct {
	mu           sync.RWMutex
	participants map[string]ModuleParticipant
	states       map[string]interface{} // Stores gathered state during copyover
}

// Global module registry
var moduleRegistry = &ModuleRegistry{
	participants: make(map[string]ModuleParticipant),
	states:       make(map[string]interface{}),
}

// RegisterModule registers a module to participate in copyover
func RegisterModule(module ModuleParticipant) error {
	moduleRegistry.mu.Lock()
	defer moduleRegistry.mu.Unlock()

	name := module.ModuleName()
	if name == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	if _, exists := moduleRegistry.participants[name]; exists {
		return fmt.Errorf("module %s already registered", name)
	}

	moduleRegistry.participants[name] = module
	return nil
}

// UnregisterModule removes a module from copyover participation
func UnregisterModule(name string) {
	moduleRegistry.mu.Lock()
	defer moduleRegistry.mu.Unlock()

	delete(moduleRegistry.participants, name)
	delete(moduleRegistry.states, name)
}

// GetRegisteredModules returns a list of all registered module names
func GetRegisteredModules() []string {
	moduleRegistry.mu.RLock()
	defer moduleRegistry.mu.RUnlock()

	names := make([]string, 0, len(moduleRegistry.participants))
	for name := range moduleRegistry.participants {
		names = append(names, name)
	}
	return names
}

// CheckModuleVetoes checks all modules for copyover readiness
func CheckModuleVetoes() (bool, []VetoInfo) {
	moduleRegistry.mu.RLock()
	defer moduleRegistry.mu.RUnlock()

	vetoes := []VetoInfo{}
	canProceed := true

	for name, module := range moduleRegistry.participants {
		if ready, veto := module.CanCopyover(); !ready {
			// Add module name to veto if not already set
			if veto.Module == "" {
				veto.Module = name
			}
			veto.Timestamp = time.Now()
			vetoes = append(vetoes, veto)

			// Hard veto blocks copyover
			if veto.Type == "hard" {
				canProceed = false
			}
		}
	}

	return canProceed, vetoes
}

// GatherModuleStates collects state from all registered modules
func GatherModuleStates() error {
	moduleRegistry.mu.Lock()
	defer moduleRegistry.mu.Unlock()

	// Clear previous states
	moduleRegistry.states = make(map[string]interface{})

	for name, module := range moduleRegistry.participants {
		state, err := module.GatherState()
		if err != nil {
			// Log error but continue with other modules
			// Individual module failures shouldn't block copyover
			mudlog.Error("Copyover", "module", name, "error", "Failed to gather state", "err", err)
			continue
		}

		if state != nil {
			moduleRegistry.states[name] = state
		}
	}

	return nil
}

// RestoreModuleStates restores state to all registered modules
func RestoreModuleStates() error {
	moduleRegistry.mu.RLock()
	defer moduleRegistry.mu.RUnlock()

	var firstError error

	for name, module := range moduleRegistry.participants {
		state, exists := moduleRegistry.states[name]
		if !exists {
			// Module has no saved state, skip
			continue
		}

		if err := module.RestoreState(state); err != nil {
			mudlog.Error("Copyover", "module", name, "error", "Failed to restore state", "err", err)
			if firstError == nil {
				firstError = err
			}
		}
	}

	return firstError
}

// PrepareModulesForCopyover notifies all modules that copyover is imminent
func PrepareModulesForCopyover() error {
	moduleRegistry.mu.RLock()
	defer moduleRegistry.mu.RUnlock()

	var firstError error

	for name, module := range moduleRegistry.participants {
		if err := module.PrepareCopyover(); err != nil {
			mudlog.Error("Copyover", "module", name, "error", "Failed to prepare for copyover", "err", err)
			if firstError == nil {
				firstError = err
			}
		}
	}

	return firstError
}

// CleanupModulesAfterCopyover notifies modules that copyover was cancelled/failed
func CleanupModulesAfterCopyover() error {
	moduleRegistry.mu.RLock()
	defer moduleRegistry.mu.RUnlock()

	var firstError error

	for name, module := range moduleRegistry.participants {
		if err := module.CleanupCopyover(); err != nil {
			mudlog.Error("Copyover", "module", name, "error", "Failed to cleanup after copyover", "err", err)
			if firstError == nil {
				firstError = err
			}
		}
	}

	return firstError
}

// ModuleState represents the state of a module in the copyover data
type ModuleState struct {
	ModuleName string      `json:"module_name"`
	State      interface{} `json:"state"`
	SavedAt    time.Time   `json:"saved_at"`
}

// Integration with the main copyover system

func init() {
	// Register module state gatherer
	manager := GetManager()

	manager.RegisterStateGatherer(func() (interface{}, error) {
		// Gather all module states
		if err := GatherModuleStates(); err != nil {
			return nil, err
		}

		// Convert to serializable format
		states := make([]ModuleState, 0, len(moduleRegistry.states))
		for name, state := range moduleRegistry.states {
			states = append(states, ModuleState{
				ModuleName: name,
				State:      state,
				SavedAt:    time.Now(),
			})
		}

		return states, nil
	})

	manager.RegisterStateRestorer(func(state *CopyoverState) error {
		// Extract module states from copyover data
		// This would need to be implemented based on how state is stored
		// For now, we'll use the already gathered states
		return RestoreModuleStates()
	})
}

// Example implementation for reference
type ExampleModule struct {
	name      string
	data      map[string]interface{}
	isRunning bool
	mu        sync.RWMutex
}

func (m *ExampleModule) ModuleName() string {
	return m.name
}

func (m *ExampleModule) GatherState() (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy of the module's state
	stateCopy := make(map[string]interface{})
	for k, v := range m.data {
		stateCopy[k] = v
	}

	return stateCopy, nil
}
