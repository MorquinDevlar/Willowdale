package usercommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/copyover"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
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
		if !mgr.IsInProgress() {
			user.SendText("No copyover in progress.")
			return true, nil
		}
		
		// Cancellation would require tracking countdown goroutine
		user.SendText("<ansi fg=\"yellow\">Copyover cancellation not yet implemented.</ansi>")
		
	case "now":
		// Immediate copyover
		user.SendText("<ansi fg=\"yellow-bold\">Initiating immediate copyover...</ansi>")
		
		result, err := mgr.InitiateCopyover(0)
		if err != nil {
			user.SendText(fmt.Sprintf("<ansi fg=\"red\">Copyover failed: %s</ansi>", err))
			return true, err
		}
		
		// We shouldn't reach here if copyover succeeds
		user.SendText(fmt.Sprintf("<ansi fg=\"red\">Copyover failed: %s</ansi>", result.Error))
		
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
		
		// Start copyover with countdown
		go func() {
			result, err := mgr.InitiateCopyover(countdown)
			if err != nil {
				// Log the error since we can't send to user after goroutine
				mudlog.Error("Copyover", "error", "Failed to initiate copyover", "err", err)
			}
			if result != nil && !result.Success {
				mudlog.Error("Copyover", "error", "Copyover failed", "reason", result.Error)
			}
		}()
	}
	
	return true, nil
}