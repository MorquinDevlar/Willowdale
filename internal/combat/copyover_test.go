package combat

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func TestCombatCopyoverState(t *testing.T) {
	// Clean up any existing state file
	defer os.Remove(combatStateFile)

	t.Run("NoActiveComabat", func(t *testing.T) {
		// With no active combat, should return empty state
		state, err := GatherCombatState()
		if err != nil {
			t.Fatalf("GatherCombatState failed: %v", err)
		}

		if len(state.PlayerCombat) != 0 {
			t.Errorf("Expected no player combat, got %d", len(state.PlayerCombat))
		}
		if len(state.MobCombat) != 0 {
			t.Errorf("Expected no mob combat, got %d", len(state.MobCombat))
		}
	})

	t.Run("SaveAndLoadState", func(t *testing.T) {
		// Save empty state
		if err := SaveCombatStateForCopyover(); err != nil {
			t.Fatalf("SaveCombatStateForCopyover failed: %v", err)
		}

		// Check file exists
		if _, err := os.Stat(combatStateFile); os.IsNotExist(err) {
			t.Error("Combat state file was not created")
		}

		// Load state
		if err := LoadCombatStateFromCopyover(); err != nil {
			t.Fatalf("LoadCombatStateFromCopyover failed: %v", err)
		}

		// Check file was cleaned up
		if _, err := os.Stat(combatStateFile); !os.IsNotExist(err) {
			t.Error("Combat state file was not cleaned up")
		}
	})

	t.Run("PreserveMobInstanceCounter", func(t *testing.T) {
		// Set a specific counter value
		originalCounter := 12345
		mobs.SetInstanceCounter(originalCounter)

		// Gather state
		state, err := GatherCombatState()
		if err != nil {
			t.Fatalf("GatherCombatState failed: %v", err)
		}

		if state.MobInstanceCounter != originalCounter {
			t.Errorf("Expected counter %d, got %d", originalCounter, state.MobInstanceCounter)
		}

		// Change the counter
		mobs.SetInstanceCounter(99999)

		// Restore state
		if err := RestoreCombatState(state); err != nil {
			t.Fatalf("RestoreCombatState failed: %v", err)
		}

		// Check counter was restored
		if mobs.GetInstanceCounter() != originalCounter {
			t.Errorf("Counter not restored: expected %d, got %d", originalCounter, mobs.GetInstanceCounter())
		}
	})
}

// MockUser creates a test user for combat testing
func mockUser(userId int, roomId int) *users.UserRecord {
	user := &users.UserRecord{
		UserId: userId,
		Character: &characters.Character{
			RoomId: roomId,
			Name:   "TestUser",
		},
	}
	return user
}

// MockMob creates a test mob for combat testing
func mockMob(mobId int, instanceId int, roomId int) *mobs.Mob {
	mob := &mobs.Mob{
		MobId:      mobs.MobId(mobId),
		InstanceId: instanceId,
		Character: characters.Character{
			RoomId: roomId,
			Name:   "TestMob",
		},
	}
	return mob
}

// MockRoom creates a test room
func mockRoom(roomId int) *rooms.Room {
	return &rooms.Room{
		RoomId: roomId,
	}
}
