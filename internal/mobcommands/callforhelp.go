package mobcommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Should check adjacent rooms for mobs and call them into the room to help if of the same group
// Format should be:
// callforhelp blows his horn
// "blows his horn" will be emoted to the room
// Other valid formats:
// callforhelp 5:blows a horn, calling for help
// callforhelp 5:guard:blows a horn, calling for help
// callforhelp guard:blows a horn, calling for help
func CallForHelp(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	calledForHelp := false

	maxRange := 1
	// Can prefix callforhelp string with a number and : to force range.
	// "callforhelp 5:blows a horn, calling for help"
	if i := strings.Index(rest, ":"); i >= 0 {
		newRangeStr := strings.TrimSpace(rest[:i])
		if newRange, err := strconv.Atoi(newRangeStr); err == nil {
			maxRange = newRange
			if i < len(rest)-1 {
				rest = strings.TrimSpace(rest[i+1:])
			} else {
				rest = ``
			}
		}
	}

	mobNameSearch := ``
	// "callforhelp 5:guard:blows a horn, calling for help"
	// "callforhelp guard:blows a horn, calling for help"
	if i := strings.Index(rest, ":"); i >= 0 {
		mobNameSearch = strings.ToLower(strings.TrimSpace(rest[:i]))

		if i < len(rest)-1 {
			rest = strings.TrimSpace(rest[i+1:])
		} else {
			rest = ``
		}
	}

	m := mapper.GetMapper(room.RoomId)
	roomList := m.FindRoomsInDistance(room.RoomId, maxRange, 0)

	for _, roomId := range roomList {
		testRoom := rooms.LoadRoom(roomId)
		if testRoom == nil {
			continue
		}

		for _, nearbyMobInstanceId := range testRoom.GetMobs(rooms.FindNeutral, rooms.FindHostile) {

			if mobInfo := mobs.GetInstance(nearbyMobInstanceId); mobInfo != nil {

				if mobNameSearch == `` {
					if !mobInfo.ConsidersAnAlly(mob) { // Only help allies
						continue
					}
				} else {
					if mobNameSearch != strings.ToLower(mobInfo.Character.Name) {
						continue
					}
				}

				if !calledForHelp {
					calledForHelp = true

					if rest != `` {
						mob.Command(fmt.Sprintf(`emote %s`, rest))
					} else {
						mob.Command(`emote calls for help`)
					}
				}

				_, randomRoomId := room.GetRandomExit()

				if randomRoomId > 0 {
					mobInfo.Command(fmt.Sprintf(`go %d`, randomRoomId), 1.0)
				}

				mobInfo.Command(fmt.Sprintf(`go %d`, room.RoomId), 1.0)

				if mob.Character.Aggro != nil && mob.Character.Aggro.UserId > 0 {
					mobInfo.Command(fmt.Sprintf(`attack @%d`, mob.Character.Aggro.UserId), 0.25)
				}
			}
		}

	}

	return true, nil
}
