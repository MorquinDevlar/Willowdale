package copyover

import (
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/inputhandlers"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RegisterConnectionGatherers adds connection-related state gatherers to the manager
func RegisterConnectionGatherers(listeners map[string]net.Listener) {
	manager := GetManager()

	// Store listeners for copyover
	manager.StoreListenersForCopyover(listeners)

	// Gather listener state
	manager.RegisterStateGatherer(func() (interface{}, error) {
		listenerStates := make(map[string]ListenerState)
		fdIndex := 3 // Start after stdin/stdout/stderr

		// Use provided listeners or fall back to preserved ones
		listenersToUse := listeners
		if len(listenersToUse) == 0 {
			listenersToUse = manager.GetPreservedListeners()
		}

		// Sort listener names for consistent ordering
		var listenerNames []string
		for name := range listenersToUse {
			if listenersToUse[name] != nil {
				listenerNames = append(listenerNames, name)
			}
		}
		sort.Strings(listenerNames)

		for _, name := range listenerNames {
			listener := listenersToUse[name]

			// Get the file descriptor
			var file *os.File
			var err error

			// Handle different listener types
			switch l := listener.(type) {
			case *net.TCPListener:
				file, err = l.File()
			default:
				mudlog.Warn("Copyover", "warning", "Unknown listener type", "type", fmt.Sprintf("%T", l))
				continue
			}

			if err != nil {
				mudlog.Error("Copyover", "error", "Failed to get listener FD", "name", name, "err", err)
				continue
			}

			// IMPORTANT: When we call File() on a listener, it creates a duplicate FD
			// The original listener should NOT be closed until after exec, as it's still
			// being used to accept connections. The duplicate FD will be passed to the child.

			listenerStates[name] = ListenerState{
				Type:    "tcp",
				Address: listener.Addr().String(),
				FD:      fdIndex,
			}

			// Store the file in manager for later
			manager.extraFiles = append(manager.extraFiles, file)
			fdIndex++
		}

		// Store listener states for later
		if manager.preservedState == nil {
			manager.preservedState = &CopyoverStateData{
				Version:     "1.0",
				Timestamp:   time.Now(),
				Environment: make(map[string]string),
				Listeners:   make(map[string]ListenerState),
				Connections: make([]ConnectionState, 0),
			}
		}
		manager.preservedState.Listeners = listenerStates

		// Log what we're storing
		mudlog.Info("Copyover", "debug", "Stored listeners in state", "count", len(listenerStates))
		for name, ls := range listenerStates {
			mudlog.Info("Copyover", "debug", "Listener state", "name", name, "fd", ls.FD, "address", ls.Address)
		}

		return listenerStates, nil
	})

	// Gather connection state
	manager.RegisterStateGatherer(func() (interface{}, error) {
		connStates := []ConnectionState{}
		fdIndex := 3 + len(listeners) // After listeners

		// Get all active connections
		for _, connId := range connections.GetAllConnectionIds() {
			cd := connections.Get(connId)
			if cd == nil {
				continue
			}

			// Only preserve logged-in connections
			if cd.State() != connections.LoggedIn {
				continue
			}

			// Get user info
			userId := 0
			if user := users.GetByConnectionId(connId); user != nil {
				userId = user.UserId
			}

			// Skip if no user
			if userId == 0 {
				continue
			}

			connState := ConnectionState{
				ConnectionID: uint64(connId),
				Type:         "telnet",
				RemoteAddr:   cd.RemoteAddr().String(),
				ConnectedAt:  time.Now(), // Note: This is copyover time, not original connection time
				UserID:       userId,
				RoomID:       0, // Initialize to 0
			}

			// Get room information from user
			if user := users.GetByConnectionId(connId); user != nil && user.Character != nil {
				connState.RoomID = user.Character.RoomId
			}

			// Check if websocket
			if cd.IsWebSocket() {
				connState.Type = "websocket"
			}

			// Handle telnet connections
			if !cd.IsWebSocket() {
				// Get file descriptor
				file, err := connections.GetFileDescriptor(connId)
				if err != nil {
					mudlog.Error("Copyover", "error", "Failed to get connection FD", "id", connId, "err", err)
					continue
				}
				if file == nil {
					mudlog.Warn("Copyover", "warning", "Could not get file descriptor", "id", connId)
					continue
				}

				// Clear close-on-exec flag - file descriptors should be preserved
				// Note: On some systems we'd clear FD_CLOEXEC, but Go's cmd.ExtraFiles handles this

				connState.FD = fdIndex

				// Store the file in manager for later
				manager.extraFiles = append(manager.extraFiles, file)
				fdIndex++
			} else {
				// WebSocket connections need special handling
				// For now, we'll mark them but not preserve the FD
				mudlog.Info("Copyover", "info", "WebSocket connection will need reconnection", "id", connId, "user", userId)
			}

			connStates = append(connStates, connState)
		}

		// Store connection states for later
		if manager.preservedState == nil {
			manager.preservedState = &CopyoverStateData{
				Version:     "1.0",
				Timestamp:   time.Now(),
				Environment: make(map[string]string),
				Listeners:   make(map[string]ListenerState),
				Connections: make([]ConnectionState, 0),
			}
		}
		manager.preservedState.Connections = connStates
		return connStates, nil
	})
}

// RegisterConnectionRestorers adds connection-related state restorers to the manager
func RegisterConnectionRestorers() {
	manager := GetManager()

	// Restore listeners
	manager.RegisterStateRestorer(func(state *CopyoverStateData) error {
		// Listeners are restored in main.go before starting the server
		// This is just for logging
		for name, listener := range state.Listeners {
			mudlog.Info("Copyover", "info", "Listener to restore", "name", name, "address", listener.Address)
		}
		return nil
	})

	// Restore connections
	manager.RegisterStateRestorer(func(state *CopyoverStateData) error {
		// Connections need to be restored after the server is running
		// This is handled in the recovery process
		mudlog.Info("Copyover", "info", "Connections to restore", "count", len(state.Connections))

		// Store the state for later recovery
		manager.preservedState = state

		return nil
	})
}

// RecoverListeners recovers listener FDs from the copyover state
func RecoverListeners(state *CopyoverStateData) map[string]net.Listener {
	if state == nil || len(state.Listeners) == 0 {
		mudlog.Info("Copyover", "info", "No listeners to recover")
		return nil
	}

	mudlog.Info("Copyover", "info", "Starting listener recovery", "count", len(state.Listeners))

	// Check if we're in copyover mode
	if os.Getenv(CopyoverEnvVar) != "1" {
		mudlog.Warn("Copyover", "warning", "Not in copyover mode, skipping listener recovery")
		return nil
	}

	recovered := make(map[string]net.Listener)
	fdIndex := 3 // Start after stdin/stdout/stderr

	// Sort listener names for consistent ordering (same as when gathering)
	var listenerNames []string
	for name := range state.Listeners {
		listenerNames = append(listenerNames, name)
	}
	sort.Strings(listenerNames)

	for _, name := range listenerNames {
		listenerState := state.Listeners[name]
		mudlog.Info("Copyover", "debug", "Recovering listener", "name", name, "address", listenerState.Address, "fd", listenerState.FD)
		if listenerState.FD != fdIndex {
			mudlog.Error("Copyover", "error", "FD mismatch", "name", name, "expected", fdIndex, "got", listenerState.FD)
			fdIndex++
			continue
		}

		// Recover the file descriptor
		file := os.NewFile(uintptr(fdIndex), fmt.Sprintf("listener-%s", name))
		if file == nil {
			mudlog.Error("Copyover", "error", "Failed to create file from FD", "name", name, "fd", fdIndex)
			fdIndex++
			continue
		}

		// Log file descriptor details
		if stat, err := file.Stat(); err == nil {
			mudlog.Info("Copyover", "debug", "Recovered FD stat", "name", name, "fd", fdIndex, "mode", stat.Mode())
		} else {
			mudlog.Warn("Copyover", "warning", "Could not stat recovered FD", "name", name, "fd", fdIndex, "err", err)
		}

		// Convert to listener
		listener, err := net.FileListener(file)
		if err != nil {
			mudlog.Error("Copyover", "error", "Failed to create listener from file", "name", name, "err", err)
			// Only close the file if we failed to create the listener
			if closeErr := file.Close(); closeErr != nil {
				mudlog.Error("Copyover", "error", "Failed to close file", "name", name, "err", closeErr)
			}
			fdIndex++
			continue
		}

		// IMPORTANT: Do NOT close the file after successful conversion to listener
		// The listener now owns the file descriptor

		// Test the listener is valid
		if listener.Addr() == nil {
			mudlog.Error("Copyover", "error", "Recovered listener has nil address", "name", name)
			fdIndex++
			continue
		}

		// Test if we can actually accept on this listener by trying to set a deadline
		// This is a non-blocking way to check if the listener is functional
		if tcpListener, ok := listener.(*net.TCPListener); ok {
			// Try to set a deadline - this will fail if the listener is invalid
			testDeadline := time.Now().Add(1 * time.Second)
			if err := tcpListener.SetDeadline(testDeadline); err != nil {
				mudlog.Error("Copyover", "error", "Recovered listener failed deadline test", "name", name, "err", err)
				// Don't continue, try to use it anyway
			} else {
				// Clear the deadline
				tcpListener.SetDeadline(time.Time{})
				mudlog.Info("Copyover", "debug", "Listener passed deadline test", "name", name)
			}
		}

		recovered[name] = listener
		mudlog.Info("Copyover", "success", "Recovered listener", "name", name, "address", listener.Addr().String(), "expected", listenerState.Address)
		fdIndex++
	}

	return recovered
}

// RecoverConnections recovers connection FDs from the copyover state
func RecoverConnections(state *CopyoverStateData) []*connections.ConnectionDetails {
	mudlog.Info("Copyover", "info", "Starting connection recovery", "count", len(state.Connections))

	var recoveredConnections []*connections.ConnectionDetails

	fdIndex := 3 + len(state.Listeners) // After listeners

	for i, connState := range state.Connections {
		if connState.Type == "websocket" {
			// WebSocket connections need to reconnect
			mudlog.Info("Copyover", "info", "WebSocket user needs to reconnect", "userId", connState.UserID)
			continue
		}

		if connState.FD != fdIndex {
			mudlog.Error("Copyover", "error", "FD mismatch", "index", i, "expected", fdIndex, "got", connState.FD)
			fdIndex++
			continue
		}

		// Recover the file descriptor
		file := os.NewFile(uintptr(fdIndex), fmt.Sprintf("conn-%d", i))
		if file == nil {
			mudlog.Error("Copyover", "error", "Failed to create file from FD", "index", i, "fd", fdIndex)
			fdIndex++
			continue
		}

		// Convert to connection
		conn, err := net.FileConn(file)
		if err != nil {
			mudlog.Error("Copyover", "error", "Failed to create connection from file", "index", i, "err", err)
			if closeErr := file.Close(); closeErr != nil {
				mudlog.Error("Copyover", "error", "Failed to close file", "index", i, "err", closeErr)
			}
			fdIndex++
			continue
		}

		// Use the original connection ID
		cd := connections.AddWithId(connections.ConnectionId(connState.ConnectionID), conn, nil)
		cd.SetState(connections.LoggedIn)

		// Set up input handlers for a logged-in connection
		// These are the standard handlers for telnet connections
		cd.AddInputHandler("TelnetIACHandler", inputhandlers.TelnetIACHandler)
		cd.AddInputHandler("AnsiHandler", inputhandlers.AnsiHandler)
		cd.AddInputHandler("CleanserInputHandler", inputhandlers.CleanserInputHandler)

		// Add the standard handlers for logged-in users
		cd.AddInputHandler("EchoInputHandler", inputhandlers.EchoInputHandler)
		cd.AddInputHandler("HistoryInputHandler", inputhandlers.HistoryInputHandler)

		// Add signal handler for Ctrl commands
		cd.AddInputHandler("SignalHandler", inputhandlers.SignalHandler, "AnsiHandler")

		// Set connection state to LoggedIn
		cd.SetState(connections.LoggedIn)

		// Associate the connection with the user
		if connState.UserID > 0 {
			// Get the user from the user manager
			user := users.GetByUserId(connState.UserID)
			if user == nil {
				// User not in memory yet, we need to find and load them
				// This can happen during copyover recovery
				mudlog.Info("Copyover", "info", "User not in memory, searching for user", "userId", connState.UserID)

				// Search offline users to find the username
				var foundUser *users.UserRecord
				users.SearchOfflineUsers(func(u *users.UserRecord) bool {
					if u.UserId == connState.UserID {
						foundUser = u
						return false // Stop searching
					}
					return true // Continue searching
				})

				if foundUser != nil {
					// Load the user
					loadedUser, err := users.LoadUser(foundUser.Username)
					if err == nil {
						user = loadedUser
						mudlog.Info("Copyover", "success", "Loaded user from disk", "userId", connState.UserID, "username", foundUser.Username)
					} else {
						mudlog.Error("Copyover", "error", "Failed to load user", "userId", connState.UserID, "username", foundUser.Username, "err", err)
					}
				} else {
					mudlog.Error("Copyover", "error", "Could not find user with userId", "userId", connState.UserID)
				}
			}

			if user != nil {
				// Re-establish the connection mapping
				users.ReconnectUser(user, cd.ConnectionId())
				mudlog.Info("Copyover", "success", "Reconnected user", "userId", connState.UserID, "username", user.Username)

				// Mark this user as recovering from copyover
				user.SetConfigOption("copyover_recovery", "true")

				// Add admin command handler if user is admin
				if user.Role == users.RoleAdmin {
					cd.AddInputHandler("SystemCommandInputHandler", inputhandlers.SystemCommandInputHandler)
				}

				// Store the room ID in the state for later
				if connState.RoomID == 0 && user.Character != nil {
					connState.RoomID = user.Character.RoomId
				}

				// We'll need to send the user back into the world after recovery is complete
				// This will be done in a separate step since we can't access worldManager from here
				mudlog.Info("Copyover", "info", "User will rejoin world", "userId", user.UserId, "roomId", connState.RoomID)
			} else {
				mudlog.Error("Copyover", "error", "User not found", "userId", connState.UserID)
				// Close the connection since we can't recover without user data
				connections.SendTo([]byte("\r\n=== COPYOVER FAILED: User data not found. Please reconnect. ===\r\n"), cd.ConnectionId())
				cd.Close()
				fdIndex++
				continue
			}
		}

		mudlog.Info("Copyover", "success", "Recovered connection", "userId", connState.UserID, "addr", connState.RemoteAddr)

		// Send a message to let them know copyover completed
		// Calculate duration if we have the start time
		var duration time.Duration
		if manager.preservedState != nil && !manager.preservedState.StartTime.IsZero() {
			duration = time.Since(manager.preservedState.StartTime)
		}

		tplData := map[string]interface{}{
			"BuildNumber": GetBuildNumber(),
			"Duration":    duration.Round(time.Millisecond).String(),
		}
		if tplText, err := templates.Process("copyover/copyover-post", tplData); err == nil {
			// Parse ANSI tags before sending
			parsedText := templates.AnsiParse(tplText)
			connections.SendTo([]byte("\r\n"+parsedText+"\r\n"), cd.ConnectionId())
		} else {
			// Fallback if template fails
			buildInfo := fmt.Sprintf("\r\n=== COPYOVER COMPLETE (Build %s) ===\r\n", GetBuildNumber())
			connections.SendTo([]byte(buildInfo), cd.ConnectionId())
		}

		// Store the connection for starting input handler later
		manager.recoveredConnections = append(manager.recoveredConnections, cd)
		recoveredConnections = append(recoveredConnections, cd)

		fdIndex++
	}

	return recoveredConnections
}

// CompleteUserRecovery should be called after the world is running to re-add users to the world
func CompleteUserRecovery(worldEnterFunc func(userId int, roomId int)) []*connections.ConnectionDetails {
	mudlog.Info("Copyover", "info", "Completing user recovery")

	// First recover the connections
	if manager.preservedState != nil {
		mudlog.Info("Copyover", "info", "Recovering connections first")
		RecoverConnections(manager.preservedState)

		// Give connections a moment to settle
		time.Sleep(100 * time.Millisecond)
	}

	// Emit restore state event for systems to restore their state
	events.AddToQueue(events.CopyoverRestoreState{
		Phase: "connections",
	})

	// Get all online users and send them back into the world
	activeUsers := users.GetAllActiveUsers()
	mudlog.Info("Copyover", "info", "Found active users", "count", len(activeUsers))

	for _, user := range activeUsers {
		mudlog.Info("Copyover", "info", "Processing user", "userId", user.UserId, "username", user.Username, "roomId", user.Character.RoomId)
		if user.Character.RoomId > 0 {
			mudlog.Info("Copyover", "info", "Sending user back to world", "userId", user.UserId, "username", user.Username, "roomId", user.Character.RoomId)
			worldEnterFunc(user.UserId, user.Character.RoomId)

			// Don't clear the flag here - let it be cleared after the first prompt is drawn
		}
	}

	// Clear the recovery state now that all users are back in the world
	manager.ClearRecoveryState()

	// Return the recovered connections that need input handlers
	return manager.GetRecoveredConnections()
}
