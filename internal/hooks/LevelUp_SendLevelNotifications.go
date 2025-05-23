package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func SendLevelNotifications(e events.Event) events.ListenerReturn {

	evt, typeOk := e.(events.LevelUp)
	if !typeOk {
		mudlog.Error("Event", "Expected Type", "LevelUp", "Actual Type", e.Type())
		return events.Cancel
	}

	user := users.GetByUserId(evt.UserId)
	if user == nil {
		return events.Continue
	}

	levelUpData := map[string]interface{}{
		"levelsGained":   evt.LevelsGained,
		"level":          evt.NewLevel,
		"statsDelta":     evt.StatsDelta,
		"trainingPoints": evt.TrainingPoints,
		"statPoints":     evt.StatPoints,
		"livesUp":        evt.LivesGained,
	}
	levelUpStr, _ := templates.Process("character/levelup", levelUpData, user.UserId)

	user.SendText(levelUpStr)

	user.PlaySound(`levelup`, `other`)

	events.AddToQueue(events.Broadcast{
		Text: fmt.Sprintf(`<ansi fg="magenta-bold">***</ansi> <ansi fg="username">%s</ansi> <ansi fg="yellow">has reached level %d!</ansi> <ansi fg="magenta-bold">***</ansi>%s`, evt.CharacterName, evt.NewLevel, term.CRLFStr),
	})

	return events.Continue
}
