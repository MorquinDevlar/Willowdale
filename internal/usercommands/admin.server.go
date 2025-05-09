package usercommands

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/gametime"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var (
	memoryReportCache = map[string]util.MemoryResult{}
)

/*
* Role Permissions:
* server 				(All)
 */
func Server(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	if rest == "" {
		infoOutput, _ := templates.Process("admincommands/help/command.server", nil, user.UserId)
		user.SendText(infoOutput)
		return true, nil
	}

	args := util.SplitButRespectQuotes(rest)
	if args[0] == "set" {

		args = args[1:]

		if len(args) < 1 {

			user.SendText(``)

			cfgData := configs.GetConfig().AllConfigData()
			cfgKeys := make([]string, 0, len(cfgData))
			for k := range cfgData {
				cfgKeys = append(cfgKeys, k)
			}

			// sort the keys
			slices.Sort(cfgKeys)

			lastPrefix := ``
			longestKey := 0

			for _, k := range cfgKeys {
				if len(k) > longestKey {
					longestKey = len(k)
				}
			}

			lineLength := 158 - longestKey

			for _, k := range cfgKeys {
				displayName := k
				nameColorized := ``
				if strings.Index(k, `.`) != -1 {
					parts := strings.Split(k, `.`)
					if len(parts) > 1 {
						if lastPrefix != parts[0] {
							lastPrefix = parts[0]
							user.SendText(``)
						}
						nameColorized = `<ansi fg="yellow">` + parts[0] + `.</ansi>`
						displayName = strings.Join(parts[1:], `.`)
					}
				}
				extraSpace := strings.Repeat(` `, longestKey-len(k))

				user.SendText(`<ansi fg="yellow-bold">` + nameColorized + displayName + `</ansi>: <ansi fg="red-bold">` + extraSpace + util.SplitStringNL(fmt.Sprintf(`%v`, cfgData[k]), lineLength, strings.Repeat(` `, longestKey+2)) + `</ansi>`)

			}

			return true, nil
		}

		if args[0] == "day" {
			gametime.SetToDay(-1)
			gd := gametime.GetDate()
			user.SendText(`Time set to ` + gd.String())
			return true, nil
		} else if args[0] == "night" {
			gametime.SetToNight(-1)
			gd := gametime.GetDate()
			user.SendText(`Time set to ` + gd.String())
			return true, nil
		}

		configName := strings.ToLower(args[0])
		configValue := strings.Join(args[1:], ` `)

		if err := configs.SetVal(configName, configValue); err != nil {
			user.SendText(fmt.Sprintf(`config change error: %s=%s (%s)`, configName, configValue, err))
			return true, nil
		}

		user.SendText(fmt.Sprintf(`config changed: %s=%s`, configName, configValue))

		return true, nil
	}

	if rest == "reload-ansi" {
		templates.LoadAliases()
		user.SendText(`ansi aliases reloaded`)
		return true, nil
	}

	if rest == "ansi-strip" {
		templates.SetAnsiFlag(templates.AnsiTagsStrip)
	}

	if rest == "ansi-mono" {
		templates.SetAnsiFlag(templates.AnsiTagsMono)
	}

	if rest == "ansi-normal" {
		templates.SetAnsiFlag(templates.AnsiTagsDefault)
	}

	if rest == "stats" || rest == "info" {

		//
		// General Go stats
		//
		user.SendText(``)
		user.SendText(fmt.Sprintf(`<ansi fg="yellow-bold">IP/Port:</ansi>    <ansi fg="red">%s</ansi>`, util.GetServerAddress()))
		user.SendText(``)

		//
		// Special timing related stats
		//
		headers := []string{"Routine", "Avg", "Low", "High", "Ct", "/sec"}
		rows := [][]string{}
		formatting := []string{`<ansi fg="yellow-bold">%s</ansi>`, `<ansi fg="cyan-bold">%s</ansi>`, `<ansi fg="cyan-bold">%s</ansi>`, `<ansi fg="cyan-bold">%s</ansi>`, `<ansi fg="black-bold">%s</ansi>`, `<ansi fg="black-bold">%s</ansi>`}

		allTimers := map[string]util.Accumulator{}
		allNames := []string{}

		times := util.GetTimeTrackers()
		for _, timeAcc := range times {

			allNames = append(allNames, timeAcc.Name)
			allTimers[timeAcc.Name] = timeAcc
		}

		sort.Strings(allNames)
		for _, name := range allNames {
			acc := allTimers[name]
			lowest, highest, average, ct := acc.Stats()
			rows = append(rows, []string{acc.Name,
				fmt.Sprintf(`%4.3fms`, average*1000),
				fmt.Sprintf(`%4.3fms`, lowest*1000),
				fmt.Sprintf(`%4.3fms`, highest*1000),
				fmt.Sprintf(`%d`, int(ct)),
				fmt.Sprintf(`%4.3f`, ct/time.Since(acc.Start).Seconds()),
			})
		}

		tblData := templates.GetTable(`Timer Stats`, headers, rows, formatting)
		tplTxt, _ := templates.Process("tables/generic", tblData, user.UserId)
		user.SendText(tplTxt)

		//
		// Alternative rendering
		//
		memRepHeaders := []string{"Section  ", "Items    ", "KB       ", "MB       ", "GB       ", "Change   "}
		memRepFormatting := []string{`<ansi fg="yellow-bold">%s</ansi>`,
			`<ansi fg="black-bold">%s</ansi>`,
			`<ansi fg="cyan-bold">%s</ansi>`,
			`<ansi fg="red">%s</ansi>`,
			`<ansi fg="red-bold">%s</ansi>`,
			`<ansi fg="black-bold">%s</ansi>`}

		memRepRows := [][]string{}
		memRepTotalTotal := uint64(0)

		sectionNames, memReports := util.GetMemoryReport()

		for idx, memReport := range memReports {

			sectionName := sectionNames[idx]

			tmpRowStorage := map[string]util.MemoryResult{}
			var memRepRowNames []string = []string{}
			var memRepTotal uint64 = 0

			for name, memResult := range memReport {
				usage := memResult.Memory
				memRepRowNames = append(memRepRowNames, name)
				tmpRowStorage[name] = memResult
				memRepTotal += usage
			}

			memRepRows = append(memRepRows, []string{`[ ` + sectionName + ` ]`, ``, ``, ``, ``, ``})
			sort.Strings(memRepRowNames)
			for _, name := range memRepRowNames {

				var rowData []string

				var prevString string = ``
				var prevCtString string = ``
				if cachedMemResult, ok := memoryReportCache[name]; ok {
					val := cachedMemResult.Memory
					if val > tmpRowStorage[name].Memory { // It has gone down
						prevString = fmt.Sprintf(`↓%s`, util.FormatBytes(val-tmpRowStorage[name].Memory))
					} else if val < tmpRowStorage[name].Memory {
						prevString = fmt.Sprintf(`↑%s`, util.FormatBytes(tmpRowStorage[name].Memory-val))
					}

					ct := cachedMemResult.Count
					if ct > tmpRowStorage[name].Count { // It has gone down
						prevCtString = fmt.Sprintf(`(↓%d)`, ct-tmpRowStorage[name].Count)
					} else if ct < tmpRowStorage[name].Count {
						prevCtString = fmt.Sprintf(`(↑%d)`, tmpRowStorage[name].Count-ct)
					}
				}
				memoryReportCache[name] = tmpRowStorage[name] // Cache the new val

				// foramt the new val
				bFormatted := util.FormatBytes(tmpRowStorage[name].Memory)

				count := ``
				if tmpRowStorage[name].Count > 0 {
					count = fmt.Sprintf(`%d %s`, tmpRowStorage[name].Count, prevCtString)
				}
				if strings.Contains(bFormatted, `KB`) {
					rowData = []string{name, count, bFormatted, ``, ``, prevString}
				} else if strings.Contains(bFormatted, `MB`) {
					rowData = []string{name, count, ``, bFormatted, ``, prevString}
				} else if strings.Contains(bFormatted, `GB`) {
					rowData = []string{name, count, ``, ``, bFormatted, prevString}
				} else {
					rowData = []string{name, count, ``, ``, ``, prevString}
				}

				memRepRows = append(memRepRows, rowData)
			}
			memRepRows = append(memRepRows, []string{``, ``, ``, ``, ``, ``})

			if sectionName != `Go` {
				memRepTotalTotal += memRepTotal
			}
			memRepTotal = 0
		}

		var rowData []string

		var name string = `Total (Non Go)`
		var prevString string = ``
		if cachedMemResult, ok := memoryReportCache[name]; ok {
			val := cachedMemResult.Memory
			if val > memRepTotalTotal { // It has gone down
				prevString = fmt.Sprintf(`↓%s`, util.FormatBytes(val-memRepTotalTotal))
			} else if val < memRepTotalTotal {
				prevString = fmt.Sprintf(`↑%s`, util.FormatBytes(memRepTotalTotal-val))
			}
		}

		memoryReportCache[name] = util.MemoryResult{memRepTotalTotal, 0} // Cache the new val

		bFormatted := util.FormatBytes(memRepTotalTotal)
		if strings.Contains(bFormatted, `KB`) {
			rowData = []string{`Total (Non Go)`, ``, bFormatted, ``, ``, prevString}
		} else if strings.Contains(bFormatted, `MB`) {
			rowData = []string{`Total (Non Go)`, ``, ``, bFormatted, ``, prevString}
		} else if strings.Contains(bFormatted, `GB`) {
			rowData = []string{`Total (Non Go)`, ``, ``, ``, bFormatted, prevString}
		} else {
			rowData = []string{`Total (Non Go)`, ``, ``, ``, ``, prevString}
		}

		memRepRows = append(memRepRows, rowData)
		memRepTblData := templates.GetTable(`Specific Memory`, memRepHeaders, memRepRows, memRepFormatting)
		memRepTxt, _ := templates.Process("tables/generic", memRepTblData, user.UserId)
		user.SendText(memRepTxt)
	}

	return true, nil
}
