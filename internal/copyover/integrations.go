package copyover

import (
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/economy"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scripting"
)

// SystemIntegration defines a subsystem that participates in copyover
type SystemIntegration struct {
	Name    string
	Gather  func() error
	Restore func() error
}

// registeredSystems holds all systems that participate in copyover
// NOTE: These systems were previously integrated but their implementations
// were removed when we consolidated the integration files. We need to
// re-implement them in their respective packages if copyover support is needed.
var registeredSystems = []SystemIntegration{
	// Combat system has actual copyover functions
	{
		Name:    "Combat",
		Gather:  func() error { return combat.SaveCombatStateForCopyover() },
		Restore: func() error { return combat.LoadCombatStateFromCopyover() },
	},
	// Rooms system has actual copyover functions
	{
		Name:   "Rooms",
		Gather: func() error { return rooms.SaveRoomStateForCopyover() },
		Restore: func() error {
			// RestoreRoomState requires a state parameter - this is handled elsewhere
			// The actual restoration happens via the rooms package's own event listener
			return nil
		},
	},
	// Event queue preservation
	{
		Name:    "EventQueue",
		Gather:  func() error { return events.SaveEventStateForCopyover() },
		Restore: func() error { return events.LoadEventStateFromCopyover() },
	},
	// Script system preservation
	{
		Name:    "Scripts",
		Gather:  func() error { return scripting.SaveScriptStateForCopyover() },
		Restore: func() error { return scripting.LoadScriptStateFromCopyover() },
	},
	// Economy system has copyover functions
	{
		Name:    "Economy",
		Gather:  func() error { return economy.SaveEconomyStateForCopyover() },
		Restore: func() error { return economy.LoadEconomyStateFromCopyover() },
	},
	// Parties system has copyover functions
	{
		Name:    "Parties",
		Gather:  func() error { return parties.SavePartyStateForCopyover() },
		Restore: func() error { return parties.LoadPartyStateFromCopyover() },
	},
	// Pet/Charm relationships
	{
		Name:    "Pets",
		Gather:  func() error { return SavePetStateForCopyover() },
		Restore: func() error { return LoadPetStateFromCopyover() },
	},
	// Quest timers
	{
		Name:    "Quests",
		Gather:  func() error { return SaveQuestStateForCopyover() },
		Restore: func() error { return LoadQuestStateFromCopyover() },
	},
	// Spell cooldowns
	{
		Name:    "SpellBuff",
		Gather:  func() error { return SaveSpellBuffStateForCopyover() },
		Restore: func() error { return LoadSpellBuffStateFromCopyover() },
	},
}

// initializeIntegrations sets up all system integrations with copyover
func init() {
	// Register handlers for all systems
	events.RegisterListener(events.CopyoverGatherState{}, handleSystemsGatherState)
	events.RegisterListener(events.CopyoverRestoreState{}, handleSystemsRestoreState)
}

// handleSystemsGatherState saves state for all registered systems
func handleSystemsGatherState(e events.Event) events.ListenerReturn {
	for _, system := range registeredSystems {
		mudlog.Info("Copyover", "phase", "GatheringState", "system", system.Name, "status", "Starting")

		if err := system.Gather(); err != nil {
			mudlog.Error("Copyover", "phase", "GatheringState", "system", system.Name, "error", err.Error())
			// Don't block copyover for individual system failures
		}

		mudlog.Info("Copyover", "phase", "GatheringState", "system", system.Name, "status", "Complete")
	}
	return events.Continue
}

// handleSystemsRestoreState restores state for all registered systems
func handleSystemsRestoreState(e events.Event) events.ListenerReturn {
	for _, system := range registeredSystems {
		mudlog.Info("Copyover", "phase", "RestoringState", "system", system.Name, "status", "Starting")

		if err := system.Restore(); err != nil {
			mudlog.Error("Copyover", "phase", "RestoringState", "system", system.Name, "error", err.Error())
			// Don't fail the entire copyover for individual system issues
		}

		mudlog.Info("Copyover", "phase", "RestoringState", "system", system.Name, "status", "Complete")
	}
	return events.Continue
}
