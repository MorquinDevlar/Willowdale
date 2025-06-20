package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/copyover"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Copyover initiates a hot reload of the server
func Copyover(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {
	args := strings.Fields(rest)

	// Handle different copyover commands
	if len(args) == 0 {
		// Show help
		user.SendText(`<ansi fg="command">copyover</ansi> - Performs a hot reload of the server without disconnecting players.

Usage:
  <ansi fg="command">copyover now</ansi>     - Immediate copyover
  <ansi fg="command">copyover [seconds]</ansi> - Copyover with countdown (default: 10)
  <ansi fg="command">copyover test</ansi>    - Test copyover readiness
  <ansi fg="command">copyover status</ansi>  - Show copyover status and history
  <ansi fg="command">copyover cancel</ansi>  - Cancel a pending copyover

During copyover:
- All player connections are preserved
- Game state is maintained
- The server process is replaced with a new one
- Players experience only a brief pause`)
		return true, nil
	}

	mgr := copyover.GetManager()

	switch strings.ToLower(args[0]) {
	case "test":
		// Test copyover readiness
		user.SendText("Testing copyover readiness...")

		// Check if copyover is already in progress
		if mgr.IsInProgress() {
			user.SendText("<ansi fg=\"red\">Copyover is already in progress!</ansi>")
			return true, nil
		}

		// Check for copyover data file
		if copyover.IsCopyoverRecovery() {
			user.SendText("<ansi fg=\"yellow\">Warning: Copyover data file exists. This may indicate a previous failed copyover.</ansi>")
		}

		// Additional readiness checks could include: active battles, pending saves, etc.
		user.SendText("<ansi fg=\"green\">Copyover system appears ready.</ansi>")

	case "cancel":
		// Cancel pending copyover
		if err := mgr.CancelCopyover(fmt.Sprintf("Cancelled by %s", user.Username)); err != nil {
			user.SendText(fmt.Sprintf("<ansi fg=\"red\">%s</ansi>", err))
			return true, nil
		}
		user.SendText("<ansi fg=\"green\">Copyover cancelled.</ansi>")

	case "status":
		// Show copyover status
		status := mgr.GetStatus()

		user.SendText("<ansi fg=\"cyan-bold\">═══ Copyover Status ═══</ansi>")
		user.SendText(fmt.Sprintf("State: <ansi fg=\"yellow\">%s</ansi>", status.State))

		if status.State.IsActive() {
			user.SendText(fmt.Sprintf("Progress: <ansi fg=\"green\">%d%%</ansi>", status.GetProgress()))

			if status.State == 1 && !status.ScheduledFor.IsZero() { // StateScheduled
				duration := status.GetTimeUntilCopyover()
				user.SendText(fmt.Sprintf("Scheduled in: <ansi fg=\"yellow\">%s</ansi>", duration))
			}
		}

		if status.LastError != "" {
			user.SendText(fmt.Sprintf("Last Error: <ansi fg=\"red\">%s</ansi> (at %s)",
				status.LastError, status.LastErrorAt.Format("15:04:05")))
		}

		// Show veto reasons if any
		if len(status.VetoReasons) > 0 {
			user.SendText("\n<ansi fg=\"red-bold\">Active Vetoes:</ansi>")
			for _, veto := range status.VetoReasons {
				user.SendText(fmt.Sprintf("  • %s: %s (<ansi fg=\"yellow\">%s</ansi>)",
					veto.Module, veto.Reason, veto.Type))
			}
		}

		// Show statistics
		user.SendText(fmt.Sprintf("\nTotal Copyovers: <ansi fg=\"cyan\">%d</ansi>", status.TotalCopyovers))
		if !status.LastCopyoverAt.IsZero() {
			user.SendText(fmt.Sprintf("Last Copyover: <ansi fg=\"cyan\">%s</ansi>",
				status.LastCopyoverAt.Format("2006-01-02 15:04:05")))
		}
		if status.AverageDuration > 0 {
			user.SendText(fmt.Sprintf("Average Duration: <ansi fg=\"cyan\">%s</ansi>", status.AverageDuration))
		}

		// Show recent history
		history := mgr.GetHistory(5)
		if len(history) > 0 {
			user.SendText("\n<ansi fg=\"cyan-bold\">Recent History:</ansi>")
			for _, h := range history {
				status := "<ansi fg=\"green\">SUCCESS</ansi>"
				if !h.Success {
					status = "<ansi fg=\"red\">FAILED</ansi>"
				}
				user.SendText(fmt.Sprintf("  [%s] %s - Duration: %s, Connections: %d saved, %d lost",
					h.StartedAt.Format("15:04:05"), status, h.Duration,
					h.ConnectionsSaved, h.ConnectionsLost))
				if h.ErrorMessage != "" {
					user.SendText(fmt.Sprintf("    Error: <ansi fg=\"red\">%s</ansi>", h.ErrorMessage))
				}
			}
		}

		user.SendText("<ansi fg=\"cyan-bold\">═══════════════════════</ansi>")

	case "now":
		// Immediate copyover
		user.SendText("<ansi fg=\"yellow-bold\">Initiating immediate copyover...</ansi>")

		// Set initiator
		status := mgr.GetStatus()
		status.InitiatedBy = user.UserId
		status.Reason = "Manual copyover by admin"

		// Queue a system event to perform copyover outside of event processing
		events.AddToQueue(events.System{
			Command: "copyover",
			Data: map[string]interface{}{
				"countdown": 0,
			},
		})

	default:
		// Try to parse as countdown seconds
		countdown, err := strconv.Atoi(args[0])
		if err != nil || countdown < 0 {
			user.SendText("Invalid countdown value. Use a positive number of seconds.")
			return true, nil
		}

		if countdown > 300 {
			user.SendText("Maximum countdown is 300 seconds (5 minutes).")
			return true, nil
		}

		if countdown == 0 {
			countdown = 10 // Default
		}

		user.SendText(fmt.Sprintf("<ansi fg=\"yellow-bold\">Initiating copyover in %d seconds...</ansi>", countdown))

		// Set initiator
		status := mgr.GetStatus()
		status.InitiatedBy = user.UserId
		status.Reason = "Scheduled copyover by admin"

		// Queue a system event to perform copyover outside of event processing
		// The countdown will be handled by the copyover manager
		events.AddToQueue(events.System{
			Command: "copyover",
			Data: map[string]interface{}{
				"countdown": countdown,
			},
		})
	}

	return true, nil
}
