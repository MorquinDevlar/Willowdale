package copyover

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

const (
	// CopyoverDataFile is the temporary file used to store state
	CopyoverDataFile = "copyover.dat"
	
	// CopyoverEnvVar indicates we're in copyover recovery mode
	CopyoverEnvVar = "GOMUD_COPYOVER"
	
	// CopyoverTimeout is how long to wait for operations
	CopyoverTimeout = 30 * time.Second
)

// Manager handles the copyover process
type Manager struct {
	mu sync.RWMutex
	
	// State tracking
	inProgress bool
	startTime  time.Time
	state      *CopyoverState
	
	// File descriptor mapping
	fdMap map[string]int // Maps connection ID to FD index
	
	// Extra files to pass to child process
	extraFiles []*os.File
	
	// Callbacks for gathering state
	stateGatherers []StateGatherer
	
	// Callbacks for restoring state
	stateRestorers []StateRestorer
	
	// Recovered connections that need input handlers
	recoveredConnections []*connections.ConnectionDetails
}

// StateGatherer is called to collect state before copyover
type StateGatherer func() (interface{}, error)

// StateRestorer is called to restore state after copyover
type StateRestorer func(state *CopyoverState) error

// Global manager instance
var (
	manager = &Manager{
		fdMap:                make(map[string]int),
		stateGatherers:       make([]StateGatherer, 0),
		stateRestorers:       make([]StateRestorer, 0),
		recoveredConnections: make([]*connections.ConnectionDetails, 0),
	}
	
	// Track if we're in recovery mode
	recoveryMu   sync.RWMutex
	isRecovering bool
)

// GetManager returns the global copyover manager
func GetManager() *Manager {
	return manager
}

// GetState returns the current copyover state
func (m *Manager) GetState() (*CopyoverState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.state == nil {
		// Try to load from disk
		state, err := m.loadState()
		if err != nil {
			return nil, err
		}
		m.state = state
	}
	
	return m.state, nil
}

// RegisterStateGatherer adds a callback to collect state
func (m *Manager) RegisterStateGatherer(gatherer StateGatherer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateGatherers = append(m.stateGatherers, gatherer)
}

// RegisterStateRestorer adds a callback to restore state
func (m *Manager) RegisterStateRestorer(restorer StateRestorer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateRestorers = append(m.stateRestorers, restorer)
}

// IsInProgress returns true if copyover is happening
func (m *Manager) IsInProgress() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.inProgress
}

// IsCopyoverRecovery checks if we're starting from a copyover
func IsCopyoverRecovery() bool {
	recoveryMu.RLock()
	recovering := isRecovering
	recoveryMu.RUnlock()
	return recovering || os.Getenv(CopyoverEnvVar) == "1" || fileExists(CopyoverDataFile)
}

// IsRecovering returns true if we're in the recovery process
func IsRecovering() bool {
	recoveryMu.RLock()
	defer recoveryMu.RUnlock()
	return isRecovering
}

// GetRecoveredConnections returns the list of recovered connections
func (m *Manager) GetRecoveredConnections() []*connections.ConnectionDetails {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.recoveredConnections
}

// InitiateCopyover starts the copyover process
func (m *Manager) InitiateCopyover(countdown int) (*CopyoverResult, error) {
	m.mu.Lock()
	if m.inProgress {
		m.mu.Unlock()
		return nil, fmt.Errorf("copyover already in progress")
	}
	m.inProgress = true
	m.startTime = time.Now()
	m.mu.Unlock()
	
	defer func() {
		m.mu.Lock()
		m.inProgress = false
		// Clean up any extra files if we didn't exec successfully
		for _, f := range m.extraFiles {
			if f != nil {
				if err := f.Close(); err != nil {
					mudlog.Error("Copyover", "error", "Failed to close extra file", "err", err)
				}
			}
		}
		m.extraFiles = nil
		m.mu.Unlock()
	}()
	
	result := &CopyoverResult{
		Success: false,
	}
	
	// Announce copyover with countdown
	if countdown > 0 {
		if err := m.announceCountdown(countdown); err != nil {
			result.Error = fmt.Sprintf("failed to announce: %v", err)
			return result, err
		}
	}
	
	// First, build the new executable
	m.announceTemplate("copyover/copyover-building", nil)
	mudlog.Info("Copyover", "status", "Building new executable")
	
	if err := m.buildExecutable(); err != nil {
		m.announceTemplate("copyover/copyover-build-failed", nil)
		result.Error = fmt.Sprintf("failed to build: %v", err)
		return result, err
	}
	
	mudlog.Info("Copyover", "status", "Build successful")
	m.announceTemplate("copyover/copyover-build-complete", nil)
	
	// Save all active users before gathering state
	m.announceTemplate("copyover/copyover-saving", nil)
	if err := m.saveAllUsers(); err != nil {
		mudlog.Error("Copyover", "error", "Failed to save some users", "err", err)
		// Continue anyway - better to copyover with some users not saved than to fail
	}
	
	mudlog.Info("Copyover", "status", "Gathering state")
	
	// Gather current state
	state, err := m.gatherState()
	if err != nil {
		result.Error = fmt.Sprintf("failed to gather state: %v", err)
		return result, err
	}
	
	// Save state to file
	if err := m.saveState(state); err != nil {
		result.Error = fmt.Sprintf("failed to save state: %v", err)
		return result, err
	}
	
	// Prepare file descriptors
	extraFiles, err := m.prepareFileDescriptors(state)
	if err != nil {
		result.Error = fmt.Sprintf("failed to prepare FDs: %v", err)
		return result, err
	}
	
	mudlog.Info("Copyover", "info", "Prepared file descriptors", "count", len(extraFiles))
	mudlog.Info("Copyover", "status", "Executing new process")
	m.announceTemplate("copyover/copyover-restarting", nil)
	
	// Execute new process
	if err := m.executeNewProcess(extraFiles); err != nil {
		result.Error = fmt.Sprintf("failed to execute: %v", err)
		return result, err
	}
	
	// If we get here, exec failed (shouldn't happen)
	result.Error = "exec returned unexpectedly"
	return result, fmt.Errorf("exec returned unexpectedly")
}

// RecoverFromCopyover restores state after a copyover
func (m *Manager) RecoverFromCopyover() error {
	mudlog.Info("Copyover", "status", "Starting recovery")
	
	// Mark that we're recovering
	recoveryMu.Lock()
	isRecovering = true
	recoveryMu.Unlock()
	
	// Load saved state
	state, err := m.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %v", err)
	}
	
	// Clean up state file immediately to prevent loops
	if err := os.Remove(CopyoverDataFile); err != nil && !os.IsNotExist(err) {
		mudlog.Error("Copyover", "error", "Failed to remove state file", "err", err)
		// Continue anyway - better to continue recovery than to fail
	}
	
	// Restore environment variables
	for key, value := range state.Environment {
		if err := os.Setenv(key, value); err != nil {
			mudlog.Error("Copyover", "error", "Failed to set environment variable", "key", key, "err", err)
			// Continue anyway - non-critical error
		}
	}
	
	mudlog.Info("Copyover", "status", "Restoring state", "connections", len(state.Connections))
	
	// Call all registered restorers
	for _, restorer := range m.stateRestorers {
		if err := restorer(state); err != nil {
			mudlog.Error("Copyover", "error", "Restorer failed", "err", err)
			// Continue with other restorers even if one fails
		}
	}
	
	mudlog.Info("Copyover", "status", "Recovery complete")
	return nil
}

// gatherState collects all state to be preserved
func (m *Manager) gatherState() (*CopyoverState, error) {
	m.state = &CopyoverState{
		Version:     "1.0",
		Timestamp:   time.Now(),
		Environment: make(map[string]string),
		Listeners:   make(map[string]ListenerState),
		Connections: make([]ConnectionState, 0),
	}
	
	// Reset extra files
	m.extraFiles = make([]*os.File, 0)
	
	// Save important environment variables
	for _, key := range []string{"CONFIG_PATH", "LOG_LEVEL", "LOG_PATH", "LOG_NOCOLOR", "CONSOLE_GMCP_OUTPUT"} {
		if value := os.Getenv(key); value != "" {
			m.state.Environment[key] = value
		}
	}
	
	// Call all registered gatherers
	for _, gatherer := range m.stateGatherers {
		if _, err := gatherer(); err != nil {
			mudlog.Error("Copyover", "error", "Gatherer failed", "err", err)
			// Continue with other gatherers
		}
	}
	
	return m.state, nil
}

// saveState writes state to disk
func (m *Manager) saveState(state *CopyoverState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	
	// Write to temp file first
	tempFile := CopyoverDataFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return err
	}
	
	// Atomic rename
	return os.Rename(tempFile, CopyoverDataFile)
}

// loadState reads state from disk
func (m *Manager) loadState() (*CopyoverState, error) {
	data, err := os.ReadFile(CopyoverDataFile)
	if err != nil {
		return nil, err
	}
	
	var state CopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	
	return &state, nil
}

// prepareFileDescriptors collects all FDs to preserve
func (m *Manager) prepareFileDescriptors(state *CopyoverState) ([]*os.File, error) {
	// The extra files have been collected by the gatherers
	mudlog.Info("Copyover", "info", "Prepared file descriptors", "count", len(m.extraFiles))
	
	// Log what we're preserving
	fdIndex := 3 // Start after stdin/stdout/stderr
	for name, listener := range state.Listeners {
		mudlog.Info("Copyover", "listener", name, "fd", listener.FD)
		fdIndex++
	}
	
	for i, conn := range state.Connections {
		if conn.Type != "websocket" && conn.FD > 0 {
			mudlog.Info("Copyover", "connection", i, "fd", conn.FD, "userId", conn.UserID)
			fdIndex++
		}
	}
	
	return m.extraFiles, nil
}

// buildExecutable builds the new server executable
func (m *Manager) buildExecutable() error {
	// Get the current executable name
	executableName := "WillowdaleMUD" // default
	if len(os.Args) > 0 {
		executableName = filepath.Base(os.Args[0])
	}
	
	buildCmd := exec.Command("go", "build", "-o", executableName)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	
	return buildCmd.Run()
}

// saveAllUsers saves all active users to disk
func (m *Manager) saveAllUsers() error {
	activeUsers := users.GetAllActiveUsers()
	mudlog.Info("Copyover", "info", "Saving all active users", "count", len(activeUsers))
	
	var errors []error
	for _, user := range activeUsers {
		if err := users.SaveUser(*user); err != nil {
			mudlog.Error("Copyover", "error", "Failed to save user", "userId", user.UserId, "username", user.Username, "err", err)
			errors = append(errors, err)
		} else {
			mudlog.Info("Copyover", "info", "Saved user", "userId", user.UserId, "username", user.Username, "roomId", user.Character.RoomId)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to save %d users", len(errors))
	}
	
	return nil
}

// executeNewProcess starts the new server process
func (m *Manager) executeNewProcess(extraFiles []*os.File) error {
	// Get current executable path
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	
	// Prepare command
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	// Preserve environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", CopyoverEnvVar))
	
	// Add extra file descriptors
	cmd.ExtraFiles = extraFiles
	
	// Go's cmd.ExtraFiles automatically handles the close-on-exec flags
	// The files in ExtraFiles will be available to the child process
	// starting at file descriptor 3
	
	// Start the new process
	if err := cmd.Start(); err != nil {
		return err
	}
	
	// Give the child process a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Original process should exit
	mudlog.Info("Copyover", "status", "Original process exiting")
	os.Exit(0)
	
	return nil
}

// announceToAll sends a message to all connected users
func (m *Manager) announceToAll(message string) {
	mudlog.Info("Copyover", "announce", message)
	// Send broadcast event
	events.AddToQueue(events.Broadcast{
		Text:             fmt.Sprintf("\r\n%s\r\n", message),
		TextScreenReader: message,
		IsCommunication:  false,
		SourceIsMod:      true,
		SkipLineRefresh:  false,
	})
}

// announceTemplate processes and announces a template to all users
func (m *Manager) announceTemplate(templateName string, data interface{}) {
	tplText, err := templates.Process(templateName, data)
	if err != nil {
		mudlog.Error("Copyover", "error", "Failed to process template", "template", templateName, "err", err)
		// Fall back to a plain message
		m.announceToAll(fmt.Sprintf("[Template Error: %s]", templateName))
		return
	}
	// Parse ANSI tags before announcing
	m.announceToAll(templates.AnsiParse(tplText))
}

// announceCountdown sends countdown messages to all players
func (m *Manager) announceCountdown(seconds int) error {
	// Initial announcement
	if seconds > 60 {
		minutes := seconds / 60
		m.announceTemplate("copyover/copyover-announce", map[string]interface{}{
			"Minutes": minutes,
		})
	} else if seconds > 10 {
		m.announceTemplate("copyover/copyover-announce", map[string]interface{}{
			"Seconds": seconds,
		})
	}
	
	// Sleep until we need to start more frequent announcements
	for seconds > 60 && seconds > 10 {
		time.Sleep(time.Second)
		seconds--
		
		// Announce at minute intervals
		if seconds%60 == 0 {
			minutes := seconds / 60
			m.announceTemplate("copyover/copyover-announce", map[string]interface{}{
				"Minutes": minutes,
			})
		}
	}
	
	// More frequent announcements as we get closer
	for i := seconds; i > 0; i-- {
		// Announce at specific intervals to avoid spam
		if i <= 10 || i == 15 || i == 30 || i == 60 {
			m.announceTemplate("copyover/copyover-countdown", map[string]interface{}{
				"Seconds": i,
			})
		}
		time.Sleep(time.Second)
	}
	
	// Final pre-copyover message
	m.announceTemplate("copyover/copyover-pre", nil)
	
	return nil
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExtractFD gets the file descriptor from a net.Listener
func ExtractListenerFD(listener net.Listener) (*os.File, error) {
	switch l := listener.(type) {
	case *net.TCPListener:
		return l.File()
	default:
		return nil, fmt.Errorf("unsupported listener type: %T", listener)
	}
}

// ExtractFD gets the file descriptor from a net.Conn
func ExtractConnFD(conn net.Conn) (*os.File, error) {
	switch c := conn.(type) {
	case *net.TCPConn:
		return c.File()
	default:
		return nil, fmt.Errorf("unsupported connection type: %T", conn)
	}
}