package usercommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

/*
* Role Permissions:
* questtoken 				(All)
 */
func QuestToken(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	// args should look like one of the following:
	// questtoken <tokenname>
	// questtoken list
	args := util.SplitButRespectQuotes(rest)

	if len(args) == 0 {
		// send some sort of help info?
		infoOutput, _ := templates.Process("admincommands/help/command.questtoken", nil, user.UserId)
		user.SendText(infoOutput)
	} else if args[0] == "list" {

		allTokens := user.Character.GetQuestProgress()
		headers := []string{"Quest Name", "Token/Steps"}
		rows := [][]string{}

		if len(allTokens) == 0 {
			rows = append(rows, []string{"None", "None"})
		} else {
			for qid, qt := range allTokens {
				qTokenStr := ``
				qToken := fmt.Sprintf(`%d-%s`, qid, qt)
				qInfo := quests.GetQuest(qToken)
				for _, step := range qInfo.Steps {
					if step.Id == qt {
						qTokenStr += fmt.Sprintf(`[%d-%s] `, qid, step.Id)
					} else {
						qTokenStr += fmt.Sprintf(`%d-%s `, qid, step.Id)
					}
				}
				rows = append(rows, []string{qInfo.Name, qTokenStr})
			}
		}

		searchResultsTable := templates.GetTable("Quest Tokens", headers, rows)
		tplTxt, _ := templates.Process("tables/generic", searchResultsTable, user.UserId)
		user.SendText(tplTxt)

	} else if args[0] == "all" {

		allQuests := quests.GetAllQuests()
		headers := []string{"Quest Name", "Token/Steps"}
		rows := [][]string{}

		if len(allQuests) == 0 {
			rows = append(rows, []string{"None", "None"})
		} else {
			for _, qInfo := range allQuests {
				qTokenStr := ``
				for _, step := range qInfo.Steps {
					qTokenStr += fmt.Sprintf(`%d-%s `, qInfo.QuestId, step.Id)
				}
				rows = append(rows, []string{qInfo.Name, qTokenStr})
			}
		}

		searchResultsTable := templates.GetTable("Quest Tokens", headers, rows)
		tplTxt, _ := templates.Process("tables/generic", searchResultsTable, user.UserId)
		user.SendText(tplTxt)

	} else {

		events.AddToQueue(events.Quest{
			UserId:     user.UserId,
			QuestToken: args[0],
		})

	}

	return true, nil
}
