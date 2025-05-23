package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Pvp(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	setting := configs.GetGamePlayConfig().PVP

	user.SendText("")
	if setting == configs.PVPDisabled {
		user.SendText(`PVP is <ansi fg="alert-5">disabled</ansi> on this server. You cannot fight other players.`)
	} else if setting == configs.PVPEnabled {
		user.SendText(`PVP is <ansi fg="green-bold">enabled</ansi> on this server. You can fight other players anywhere.`)
	} else if setting == configs.PVPLimited {
		user.SendText(`PVP is <ansi fg="yellow">limited</ansi> on this server. You can fight other players in places labeled with: <ansi fg="11" bg="52"> ☠ PK Area ☠ </ansi>.`)
	}
	user.SendText("")

	return true, nil
}
