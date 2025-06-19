package combat

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// CombatCopyoverState represents all combat-related state that needs to be preserved
type CombatCopyoverState struct {
	// Player combat states
	PlayerCombat []PlayerCombatState `json:"player_combat"`

	// Mob combat states
	MobCombat []MobCombatState `json:"mob_combat"`

	// Global mob instance counter
	MobInstanceCounter int `json:"mob_instance_counter"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

// PlayerCombatState represents a player's combat state
type PlayerCombatState struct {
	UserId     int               `json:"user_id"`
	RoomId     int               `json:"room_id"`
	Aggro      *characters.Aggro `json:"aggro,omitempty"`
	Damage     map[int]int       `json:"player_damage,omitempty"`
	LastDamage uint64            `json:"last_damage,omitempty"`
}

// MobCombatState represents a mob's combat state
type MobCombatState struct {
	MobId      int               `json:"mob_id"`
	InstanceId int               `json:"instance_id"`
	RoomId     int               `json:"room_id"`
	Aggro      *characters.Aggro `json:"aggro,omitempty"`
	Damage     map[int]int       `json:"player_damage,omitempty"`
	LastDamage uint64            `json:"last_damage,omitempty"`
	CharmedBy  int               `json:"charmed_by,omitempty"`
}

// GatherCombatState collects all combat-related state for copyover
func GatherCombatState() (*CombatCopyoverState, error) {
	state := &CombatCopyoverState{
		PlayerCombat: []PlayerCombatState{},
		MobCombat:    []MobCombatState{},
		SavedAt:      time.Now(),
	}

	// Gather player combat states
	for _, userId := range users.GetOnlineUserIds() {
		user := users.GetByUserId(userId)
		if user == nil || user.Character == nil {
			continue
		}

		// Only save if player is in combat
		if user.Character.Aggro != nil {
			playerState := PlayerCombatState{
				UserId:     userId,
				RoomId:     user.Character.RoomId,
				Aggro:      user.Character.Aggro,
				Damage:     user.Character.PlayerDamage,
				LastDamage: user.Character.LastPlayerDamage,
			}
			state.PlayerCombat = append(state.PlayerCombat, playerState)
		}
	}

	// Gather mob combat states
	for _, room := range rooms.GetAllRooms() {
		for _, mobId := range room.GetMobs() {
			mob := mobs.GetInstance(mobId)
			if mob == nil {
				continue
			}
			// Only save if mob is in combat or has damage tracking
			if mob.Character.Aggro != nil || len(mob.Character.PlayerDamage) > 0 {
				mobState := MobCombatState{
					MobId:      int(mob.MobId),
					InstanceId: mob.InstanceId,
					RoomId:     room.RoomId,
					Aggro:      mob.Character.Aggro,
					Damage:     mob.Character.PlayerDamage,
					LastDamage: mob.Character.LastPlayerDamage,
				}

				// Check if mob is charmed
				if mob.Character.IsCharmed() {
					mobState.CharmedBy = mob.Character.Charmed.UserId
				}

				state.MobCombat = append(state.MobCombat, mobState)
			}
		}
	}

	// Get the current mob instance counter
	state.MobInstanceCounter = mobs.GetInstanceCounter()

	return state, nil
}

// RestoreCombatState restores combat state after copyover
func RestoreCombatState(state *CombatCopyoverState) error {
	if state == nil {
		return nil
	}

	// Restore mob instance counter first
	mobs.SetInstanceCounter(state.MobInstanceCounter)

	// Restore player combat states
	for _, playerState := range state.PlayerCombat {
		user := users.GetByUserId(playerState.UserId)
		if user == nil || user.Character == nil {
			continue
		}

		// Restore combat state
		user.Character.Aggro = playerState.Aggro
		user.Character.PlayerDamage = playerState.Damage
		user.Character.LastPlayerDamage = playerState.LastDamage

		// Validate aggro target still exists
		if err := validateAggroTarget(user.Character); err != nil {
			// Clear invalid aggro
			user.Character.Aggro = nil
		}
	}

	// Restore mob combat states
	for _, mobState := range state.MobCombat {
		// Find the mob by instance ID
		room := rooms.LoadRoom(mobState.RoomId)
		if room == nil {
			continue
		}

		var targetMob *mobs.Mob
		for _, mobId := range room.GetMobs() {
			mob := mobs.GetInstance(mobId)
			if mob != nil && mob.InstanceId == mobState.InstanceId {
				targetMob = mob
				break
			}
		}

		if targetMob == nil {
			// Mob instance not found, skip
			continue
		}

		// Restore combat state
		targetMob.Character.Aggro = mobState.Aggro
		targetMob.Character.PlayerDamage = mobState.Damage
		targetMob.Character.LastPlayerDamage = mobState.LastDamage

		// Restore charmed relationship
		if mobState.CharmedBy > 0 {
			if user := users.GetByUserId(mobState.CharmedBy); user != nil {
				targetMob.Character.Charmed = &characters.CharmInfo{
					UserId:          mobState.CharmedBy,
					RoundsRemaining: -1, // Preserve permanent charm
				}
			}
		}

		// Validate aggro target
		if err := validateAggroTarget(&targetMob.Character); err != nil {
			// Clear invalid aggro
			targetMob.Character.Aggro = nil
		}
	}

	return nil
}

// validateAggroTarget checks if an aggro target still exists
func validateAggroTarget(char *characters.Character) error {
	if char.Aggro == nil {
		return nil
	}

	// Check based on aggro type
	if char.Aggro.UserId > 0 {
		// Target is a player
		if user := users.GetByUserId(char.Aggro.UserId); user == nil {
			return fmt.Errorf("aggro target player %d not found", char.Aggro.UserId)
		}
	} else if char.Aggro.MobInstanceId > 0 {
		// Target is a mob - need to verify it exists
		// This is more complex as we'd need to search all rooms
		// For now, the validation happens during combat processing
	}

	return nil
}

// PauseCombat temporarily halts all combat processing
func PauseCombat() {
	// This would be called before copyover begins
	// Currently combat is processed automatically in rounds,
	// so pausing would require a global flag
}

// ResumeCombat resumes normal combat processing
func ResumeCombat() {
	// This would be called after copyover completes
	// Would clear any pause flags set by PauseCombat
}

const combatStateFile = "combat_copyover.dat"

// SaveCombatStateForCopyover saves combat state to a file for copyover
func SaveCombatStateForCopyover() error {
	state, err := GatherCombatState()
	if err != nil {
		return fmt.Errorf("failed to gather combat state: %w", err)
	}

	if state == nil {
		// No combat state to save
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal combat state: %w", err)
	}

	if err := os.WriteFile(combatStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write combat state file: %w", err)
	}

	return nil
}

// LoadCombatStateFromCopyover loads and restores combat state after copyover
func LoadCombatStateFromCopyover() error {
	data, err := os.ReadFile(combatStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No combat state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read combat state file: %w", err)
	}

	var state CombatCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal combat state: %w", err)
	}

	if err := RestoreCombatState(&state); err != nil {
		return fmt.Errorf("failed to restore combat state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(combatStateFile)

	return nil
}
