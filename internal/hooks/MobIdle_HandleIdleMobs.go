package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/conversations"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scripting"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// Handles default mob idle behavior
//

func HandleIdleMobs(e events.Event) events.ListenerReturn {

	evt := e.(events.MobIdle)

	mob := mobs.GetInstance(evt.MobInstanceId)
	if mob == nil {
		return events.Cancel
	}

	isCharmed := mob.Character.IsCharmed()

	// if a mob shouldn't be allowed to leave their area (via wandering)
	// but has somehow been displaced, such as pulling through combat, spells, or otherwise
	// tell them to path back home
	if mob.MaxWander == 0 && mob.Character.RoomId != mob.HomeRoomId {
		if !isCharmed {
			mob.Command("pathto home")
		}
	}

	if conversations.HasConverseFile(int(mob.MobId), mob.Character.Zone) && util.Rand(100) < int(configs.GetGamePlayConfig().MobConverseChance) {
		if mobRoom := rooms.LoadRoom(mob.Character.RoomId); mobRoom != nil {
			mobcommands.Converse(``, mob, mobRoom) // Execute this directly so that target mob doesn't leave the room before this command executes
		}
	}

	// If they have idle commands, maybe do one of them?
	handled, _ := scripting.TryMobScriptEvent("onIdle", mob.InstanceId, 0, ``, nil)
	if !handled {

		if isCharmed {
			// Only some mobs can apply first aid
			// If a charmed mob can aid someone, try.
			if mob.Character.KnowsFirstAid() {
				mob.Command(`lookforaid`)
			}

			return events.Continue
		}

		if mob.MaxWander > -1 && mob.WanderCount > mob.MaxWander {
			// Not charmed and far from home, and should never leave home.
			// So go home.
			mob.Command(`pathto home`)
			return events.Continue
		}

		//
		// Look for trouble
		//
		idleCmd := `lookfortrouble`
		if util.Rand(100) < mob.ActivityLevel {
			idleCmd = mob.GetIdleCommand()
			if idleCmd == `` {
				idleCmd = `lookfortrouble`
			}
		}
		mob.Command(idleCmd)

	}

	return events.Continue
}
