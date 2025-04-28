// Round ticks for players
package hooks

import (
	"fmt"
	"strconv"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

//
// Handle mobs that are bored
//

func IdleMobs(e events.Event) events.ListenerReturn {

	mobPathAnnounce := false // useful for debugging purposes.

	mc := configs.GetMemoryConfig()

	maxBoredom := uint8(mc.MaxMobBoredom)

	allMobInstances := mobs.GetAllMobInstanceIds()

	allowedUnloadCt := len(allMobInstances) - int(mc.MobUnloadThreshold)
	if allowedUnloadCt < 0 {
		allowedUnloadCt = 0
	}

	// Handle idle mob behavior
	tStart := time.Now()
	for _, mobId := range allMobInstances {

		mob := mobs.GetInstance(mobId)
		if mob == nil {
			allowedUnloadCt--
			continue
		}

		if allowedUnloadCt > 0 && mob.BoredomCounter >= maxBoredom {

			if mob.Despawns() {
				mob.Command(`despawn` + fmt.Sprintf(` depression %d/%d`, mob.BoredomCounter, maxBoredom))
				allowedUnloadCt--

			} else {
				mob.BoredomCounter = 0
			}

			continue
		}

		// If idle prevented, it's a one round interrupt (until another comes along)
		if mob.PreventIdle {
			mob.PreventIdle = false
			continue
		}

		// If they are doing some sort of combat thing,
		// Don't do idle actions
		if mob.Character.Aggro != nil {
			if mob.Character.Aggro.UserId > 0 {
				user := users.GetByUserId(mob.Character.Aggro.UserId)
				if user == nil || user.Character.RoomId != mob.Character.RoomId {
					mob.Command(`emote mumbles about losing their quarry.`)
					mob.Character.Aggro = nil
				}
			}
			continue
		}

		if mob.InConversation() {
			mob.Converse()
			continue
		}

		// Check whether they are currently in the middle of a path, or have one waiting to start.
		// This comes after checks for whether they are currently in a conersation, or in combat, etc.
		if currentStep := mob.Path.Current(); currentStep != nil || mob.Path.Len() > 0 {

			if currentStep == nil {
				if mobPathAnnounce {
					mob.Command(`say I'm beginning a new path.`)
				}
			} else {

				// If their currentStep isnt' actually the room they are in
				// They've somehow been moved. Reclaculate a new path.
				if currentStep.RoomId() != mob.Character.RoomId {
					if mobPathAnnounce {
						mob.Command(`say I seem to have wandered off my path.`)
					}

					reDoWaypoints := mob.Path.Waypoints()
					if len(reDoWaypoints) > 0 {
						newCommand := `pathto`
						for _, wpInt := range reDoWaypoints {
							newCommand += ` ` + strconv.Itoa(wpInt)
						}
						mob.Command(newCommand)
						continue
					}

					// if we were unable to come up with a new path, send them home.
					mob.Command(`pathto home`)

					continue
				}

				if currentStep.Waypoint() {
					if mobPathAnnounce {
						mob.Command(`say I've reached a waypoint.`)
					}
				}
			}

			if nextStep := mob.Path.Next(); nextStep != nil {

				if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
					if exitInfo, ok := room.Exits[nextStep.ExitName()]; ok {
						if exitInfo.RoomId == nextStep.RoomId() {
							mob.Command(nextStep.ExitName())
							continue
						}
					}
				}

			}

			if mobPathAnnounce {
				mob.Command(`say I'm.... done.`)
			}
			mob.Path.Clear()
		}

		events.AddToQueue(events.MobIdle{MobInstanceId: mobId})

	}

	util.TrackTime(`IdleMobs()`, time.Since(tStart).Seconds())

	return events.Continue
}
