package copyover

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Pet/Charm system data structures

type PetCopyoverState struct {
	CharmedRelationships []CharmedRelationship `json:"charmed_relationships"`
	SavedAt              time.Time             `json:"saved_at"`
}

type CharmedRelationship struct {
	UserId        int                  `json:"user_id"`
	MobInstanceId int                  `json:"mob_instance_id"`
	MobId         int                  `json:"mob_id"`
	RoomId        int                  `json:"room_id"`
	CharmInfo     characters.CharmInfo `json:"charm_info"`
}

const petStateFile = "pet_copyover.dat"

// Pet/Charm system handlers

// SavePetStateForCopyover preserves pet/charm relationships during copyover
func SavePetStateForCopyover() error {
	state := &PetCopyoverState{
		CharmedRelationships: []CharmedRelationship{},
		SavedAt:              time.Now(),
	}

	// Get all online users
	activeUsers := users.GetAllActiveUsers()

	for _, user := range activeUsers {
		if user.Character == nil {
			continue
		}

		// Check for charmed mobs
		for _, mobInstanceId := range user.Character.CharmedMobs {
			// Find the mob instance
			mob := mobs.GetInstance(mobInstanceId)
			if mob == nil || mob.Character.Charmed == nil {
				continue
			}

			relationship := CharmedRelationship{
				UserId:        user.UserId,
				MobInstanceId: mob.InstanceId,
				MobId:         int(mob.MobId),
				RoomId:        mob.Character.RoomId,
				CharmInfo:     *mob.Character.Charmed,
			}
			state.CharmedRelationships = append(state.CharmedRelationships, relationship)
			mudlog.Info("Copyover", "subsystem", "Pets", "action", "SaveCharm",
				"userId", user.UserId, "mobInstanceId", mob.InstanceId)
		}
	}

	if len(state.CharmedRelationships) == 0 {
		// No charmed relationships to save
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal pet state: %w", err)
	}

	if err := os.WriteFile(petStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write pet state file: %w", err)
	}

	mudlog.Info("Copyover", "subsystem", "Pets", "action", "StateSaved",
		"relationships", len(state.CharmedRelationships))
	return nil
}

// LoadPetStateFromCopyover restores pet/charm relationships after copyover
func LoadPetStateFromCopyover() error {
	data, err := os.ReadFile(petStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No pet state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read pet state file: %w", err)
	}

	var state PetCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal pet state: %w", err)
	}

	restored := 0
	for _, relationship := range state.CharmedRelationships {
		// Find the user
		user := users.GetByUserId(relationship.UserId)
		if user == nil || user.Character == nil {
			mudlog.Warn("Copyover", "subsystem", "Pets", "action", "RestoreCharm",
				"error", "User not found", "userId", relationship.UserId)
			continue
		}

		// Find the mob by instance ID
		mob := mobs.GetInstance(relationship.MobInstanceId)
		if mob == nil {
			mudlog.Warn("Copyover", "subsystem", "Pets", "action", "RestoreCharm",
				"error", "Mob instance not found", "mobInstanceId", relationship.MobInstanceId)
			continue
		}

		// Verify mob is in the expected room
		if mob.Character.RoomId != relationship.RoomId {
			mudlog.Warn("Copyover", "subsystem", "Pets", "action", "RestoreCharm",
				"error", "Mob not in expected room", "mobInstanceId", relationship.MobInstanceId,
				"expectedRoom", relationship.RoomId, "actualRoom", mob.Character.RoomId)
			continue
		}

		// Restore the charm relationship
		charmInfo := relationship.CharmInfo
		mob.Character.Charmed = &charmInfo

		// Ensure the user's CharmedMobs list includes this mob
		found := false
		for _, id := range user.Character.CharmedMobs {
			if id == mob.InstanceId {
				found = true
				break
			}
		}
		if !found {
			user.Character.CharmedMobs = append(user.Character.CharmedMobs, mob.InstanceId)
		}

		restored++
		mudlog.Info("Copyover", "subsystem", "Pets", "action", "CharmRestored",
			"userId", relationship.UserId, "mobInstanceId", mob.InstanceId)
	}

	// Clean up the file after successful restoration
	os.Remove(petStateFile)

	mudlog.Info("Copyover", "subsystem", "Pets", "action", "StateRestored",
		"restored", restored, "total", len(state.CharmedRelationships))
	return nil
}

// Quest system data structures

type QuestCopyoverState struct {
	CharacterTimers []CharacterQuestTimers `json:"character_timers"`
	SavedAt         time.Time              `json:"saved_at"`
}

type CharacterQuestTimers struct {
	UserId int                            `json:"user_id"`
	Timers map[string]gametime.RoundTimer `json:"timers"`
}

const questStateFile = "quest_copyover.dat"

// Quest system handlers

// SaveQuestStateForCopyover preserves quest timers during copyover
func SaveQuestStateForCopyover() error {
	state := &QuestCopyoverState{
		CharacterTimers: []CharacterQuestTimers{},
		SavedAt:         time.Now(),
	}

	// Get all online users
	activeUsers := users.GetAllActiveUsers()

	for _, user := range activeUsers {
		if user.Character == nil || len(user.Character.Timers) == 0 {
			continue
		}

		// Extract quest-related timers (those with "quest" prefix)
		questTimers := make(map[string]gametime.RoundTimer)
		for name, timer := range user.Character.Timers {
			if strings.HasPrefix(name, "quest") {
				questTimers[name] = timer
			}
		}

		if len(questTimers) > 0 {
			charTimers := CharacterQuestTimers{
				UserId: user.UserId,
				Timers: questTimers,
			}
			state.CharacterTimers = append(state.CharacterTimers, charTimers)
			mudlog.Info("Copyover", "subsystem", "Quests", "action", "SaveTimers",
				"userId", user.UserId, "timerCount", len(questTimers))
		}
	}

	if len(state.CharacterTimers) == 0 {
		// No quest timers to save
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal quest state: %w", err)
	}

	if err := os.WriteFile(questStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write quest state file: %w", err)
	}

	mudlog.Info("Copyover", "subsystem", "Quests", "action", "StateSaved",
		"users", len(state.CharacterTimers))
	return nil
}

// LoadQuestStateFromCopyover restores quest timers after copyover
func LoadQuestStateFromCopyover() error {
	data, err := os.ReadFile(questStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No quest state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read quest state file: %w", err)
	}

	var state QuestCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal quest state: %w", err)
	}

	restored := 0
	for _, charTimers := range state.CharacterTimers {
		// Find the user
		user := users.GetByUserId(charTimers.UserId)
		if user == nil || user.Character == nil {
			mudlog.Warn("Copyover", "subsystem", "Quests", "action", "RestoreTimers",
				"error", "User not found", "userId", charTimers.UserId)
			continue
		}

		// Restore quest timers
		if user.Character.Timers == nil {
			user.Character.Timers = make(map[string]gametime.RoundTimer)
		}

		for name, timer := range charTimers.Timers {
			user.Character.Timers[name] = timer
			restored++
		}

		mudlog.Info("Copyover", "subsystem", "Quests", "action", "TimersRestored",
			"userId", charTimers.UserId, "timerCount", len(charTimers.Timers))
	}

	// Clean up the file after successful restoration
	os.Remove(questStateFile)

	mudlog.Info("Copyover", "subsystem", "Quests", "action", "StateRestored",
		"timersRestored", restored, "users", len(state.CharacterTimers))
	return nil
}

// SpellBuff system data structures

type SpellBuffCopyoverState struct {
	ActiveSpells []ActiveSpellCast `json:"active_spells"`
	SavedAt      time.Time         `json:"saved_at"`
}

type ActiveSpellCast struct {
	CasterType    string                    `json:"caster_type"` // "user" or "mob"
	CasterId      int                       `json:"caster_id"`
	RoomId        int                       `json:"room_id"`
	SpellId       string                    `json:"spell_id"`
	RoundsWaiting int                       `json:"rounds_waiting"`
	SpellInfo     characters.SpellAggroInfo `json:"spell_info"`
}

const spellBuffStateFile = "spellbuff_copyover.dat"

// SpellBuff system handlers

// SaveSpellBuffStateForCopyover preserves active spell casts during copyover
func SaveSpellBuffStateForCopyover() error {
	state := &SpellBuffCopyoverState{
		ActiveSpells: []ActiveSpellCast{},
		SavedAt:      time.Now(),
	}

	// Get all online users with active spell casts
	activeUsers := users.GetAllActiveUsers()

	for _, user := range activeUsers {
		if user.Character == nil || user.Character.Aggro == nil {
			continue
		}

		// Check if they're casting a spell
		if user.Character.Aggro.Type == characters.SpellCast {
			activeCast := ActiveSpellCast{
				CasterType:    "user",
				CasterId:      user.UserId,
				RoomId:        user.Character.RoomId,
				SpellId:       user.Character.Aggro.SpellInfo.SpellId,
				RoundsWaiting: user.Character.Aggro.RoundsWaiting,
				SpellInfo:     user.Character.Aggro.SpellInfo,
			}
			state.ActiveSpells = append(state.ActiveSpells, activeCast)
			mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "SaveSpellCast",
				"userId", user.UserId, "spellId", user.Character.Aggro.SpellInfo.SpellId)
		}
	}

	// Check all mobs for active spell casts
	for _, room := range rooms.GetAllRooms() {
		mobIds := room.GetMobs()
		for _, mobId := range mobIds {
			mob := mobs.GetInstance(mobId)
			if mob == nil || mob.Character.Aggro == nil {
				continue
			}

			if mob.Character.Aggro.Type == characters.SpellCast {
				activeCast := ActiveSpellCast{
					CasterType:    "mob",
					CasterId:      mob.InstanceId,
					RoomId:        room.RoomId,
					SpellId:       mob.Character.Aggro.SpellInfo.SpellId,
					RoundsWaiting: mob.Character.Aggro.RoundsWaiting,
					SpellInfo:     mob.Character.Aggro.SpellInfo,
				}
				state.ActiveSpells = append(state.ActiveSpells, activeCast)
				mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "SaveSpellCast",
					"mobInstanceId", mob.InstanceId, "spellId", mob.Character.Aggro.SpellInfo.SpellId)
			}
		}
	}

	if len(state.ActiveSpells) == 0 {
		// No active spells to save
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal spellbuff state: %w", err)
	}

	if err := os.WriteFile(spellBuffStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write spellbuff state file: %w", err)
	}

	mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "StateSaved",
		"activeSpells", len(state.ActiveSpells))
	return nil
}

// LoadSpellBuffStateFromCopyover restores active spell casts after copyover
func LoadSpellBuffStateFromCopyover() error {
	data, err := os.ReadFile(spellBuffStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No spellbuff state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read spellbuff state file: %w", err)
	}

	var state SpellBuffCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal spellbuff state: %w", err)
	}

	restored := 0
	for _, activeCast := range state.ActiveSpells {
		if activeCast.CasterType == "user" {
			// Restore user spell cast
			user := users.GetByUserId(activeCast.CasterId)
			if user == nil || user.Character == nil {
				mudlog.Warn("Copyover", "subsystem", "SpellBuff", "action", "RestoreSpellCast",
					"error", "User not found", "userId", activeCast.CasterId)
				continue
			}

			// Validate targets still exist
			validUserTargets := []int{}
			for _, targetId := range activeCast.SpellInfo.TargetUserIds {
				if users.GetByUserId(targetId) != nil {
					validUserTargets = append(validUserTargets, targetId)
				}
			}
			activeCast.SpellInfo.TargetUserIds = validUserTargets

			// Recreate the aggro structure
			user.Character.Aggro = &characters.Aggro{
				Type:          characters.SpellCast,
				RoundsWaiting: activeCast.RoundsWaiting,
				SpellInfo:     activeCast.SpellInfo,
			}

			restored++
			mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "SpellCastRestored",
				"userId", activeCast.CasterId, "spellId", activeCast.SpellId)

		} else if activeCast.CasterType == "mob" {
			// Restore mob spell cast
			mob := mobs.GetInstance(activeCast.CasterId)
			if mob == nil {
				mudlog.Warn("Copyover", "subsystem", "SpellBuff", "action", "RestoreSpellCast",
					"error", "Mob instance not found", "mobInstanceId", activeCast.CasterId)
				continue
			}

			// Verify mob is in the expected room
			if mob.Character.RoomId != activeCast.RoomId {
				mudlog.Warn("Copyover", "subsystem", "SpellBuff", "action", "RestoreSpellCast",
					"error", "Mob not in expected room", "mobInstanceId", activeCast.CasterId,
					"expectedRoom", activeCast.RoomId, "actualRoom", mob.Character.RoomId)
				continue
			}

			// Validate targets still exist
			validUserTargets := []int{}
			for _, targetId := range activeCast.SpellInfo.TargetUserIds {
				if users.GetByUserId(targetId) != nil {
					validUserTargets = append(validUserTargets, targetId)
				}
			}
			activeCast.SpellInfo.TargetUserIds = validUserTargets

			// Recreate the aggro structure
			mob.Character.Aggro = &characters.Aggro{
				Type:          characters.SpellCast,
				RoundsWaiting: activeCast.RoundsWaiting,
				SpellInfo:     activeCast.SpellInfo,
			}

			restored++
			mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "SpellCastRestored",
				"mobInstanceId", activeCast.CasterId, "spellId", activeCast.SpellId)
		}
	}

	// Clean up the file after successful restoration
	os.Remove(spellBuffStateFile)

	mudlog.Info("Copyover", "subsystem", "SpellBuff", "action", "StateRestored",
		"restored", restored, "total", len(state.ActiveSpells))
	return nil
}
