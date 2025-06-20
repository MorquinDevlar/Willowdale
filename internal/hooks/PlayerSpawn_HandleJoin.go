package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/scripting"
	"github.com/GoMudEngine/GoMud/internal/users"
)

//
// Execute on join commands
//

func HandleJoin(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.PlayerSpawn)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "PlayerSpawn", "Actual Type", e.Type())
		return events.Cancel
	}

	user := users.GetByUserId(evt.UserId)
	if user == nil {
		mudlog.Error("HandleJoin", "error", fmt.Sprintf(`user %d not found`, evt.UserId))
		return events.Cancel
	}

	user.EventLog.Add(`conn`, fmt.Sprintf(`<ansi fg="username">%s</ansi> entered the world`, user.Character.Name))

	users.RemoveZombieUser(evt.UserId)

	room := rooms.LoadRoom(user.Character.RoomId)
	if room == nil {

		mudlog.Error("EnterWorld", "error", fmt.Sprintf(`room %d not found`, user.Character.RoomId))

		if err := rooms.MoveToRoom(user.UserId, 0); err != nil {
			mudlog.Error("EnterWorld", "msg", "could not move to room 0", "error", err)
		}

		room = rooms.LoadRoom(user.Character.RoomId)
	}

	// Check if this is a copyover recovery
	isCopyoverRecovery := user.GetConfigOption("copyover_recovery") == "true"
	if isCopyoverRecovery {
		// Don't clear the flag yet - we need it for prompt handling
		mudlog.Info("HandleJoin", "info", "Skipping OnLoginCommands for copyover recovery", "userId", evt.UserId)
	} else {
		// Execute OnLoginCommands only for normal logins
		loginCmds := configs.GetConfig().Server.OnLoginCommands
		if len(loginCmds) > 0 {

			for _, cmd := range loginCmds {

				events.AddToQueue(events.Input{
					UserId:    evt.UserId,
					InputText: cmd,
					ReadyTurn: 0, // No delay between execution of commands
				})

			}

		}
	}

	if room != nil {
		if doLook, err := scripting.TryRoomScriptEvent(`onEnter`, user.UserId, user.Character.RoomId); err != nil || doLook {
			user.CommandFlagged(`look`, events.CmdSecretly) // Do a secret look.
		}
	}

	return events.Continue
}
