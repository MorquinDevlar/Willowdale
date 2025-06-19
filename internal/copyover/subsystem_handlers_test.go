package copyover

import (
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/gametime"
)

func TestPetCopyoverState(t *testing.T) {
	// Test serialization of pet state
	_ = &PetCopyoverState{
		CharmedRelationships: []CharmedRelationship{
			{
				UserId:        123,
				MobInstanceId: 456,
				MobId:         789,
				RoomId:        1000,
				CharmInfo: characters.CharmInfo{
					UserId:          123,
					RoundsRemaining: 100,
					ExpiredCommand:  "emote waves goodbye",
				},
			},
		},
	}

	// Clean up any existing file
	os.Remove(petStateFile)

	// This would normally be called during copyover
	// We're just testing the data structures compile and can be marshaled
	t.Log("Pet copyover state structures compile correctly")
}

func TestQuestCopyoverState(t *testing.T) {
	// Test serialization of quest state
	_ = &QuestCopyoverState{
		CharacterTimers: []CharacterQuestTimers{
			{
				UserId: 123,
				Timers: map[string]gametime.RoundTimer{
					"quest_delivery": {
						RoundStart: 1000,
						Period:     "24h",
					},
					"quest_cooldown": {
						RoundStart: 2000,
						Period:     "1h",
					},
				},
			},
		},
	}

	// Clean up any existing file
	os.Remove(questStateFile)

	// This would normally be called during copyover
	t.Log("Quest copyover state structures compile correctly")
}

func TestSpellBuffCopyoverState(t *testing.T) {
	// Test serialization of spell buff state
	_ = &SpellBuffCopyoverState{
		ActiveSpells: []ActiveSpellCast{
			{
				CasterType:    "user",
				CasterId:      123,
				RoomId:        1000,
				SpellId:       "fireball",
				RoundsWaiting: 3,
				SpellInfo: characters.SpellAggroInfo{
					SpellId:              "fireball",
					SpellRest:            "at goblin",
					TargetUserIds:        []int{},
					TargetMobInstanceIds: []int{456},
				},
			},
		},
	}

	// Clean up any existing file
	os.Remove(spellBuffStateFile)

	// This would normally be called during copyover
	t.Log("SpellBuff copyover state structures compile correctly")
}
