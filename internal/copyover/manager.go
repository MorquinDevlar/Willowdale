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

	// State machine
	currentState   CopyoverPhase
	stateChangedAt time.Time
	status         *CopyoverStatus

	// Progress tracking
	progress map[string]int

	// History tracking
	history        []CopyoverHistory
	historyCounter int

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

	// Preserved listeners for copyover
	preservedListeners map[string]net.Listener
}

// StateGatherer is called to collect state before copyover
type StateGatherer func() (interface{}, error)

// StateRestorer is called to restore state after copyover
type StateRestorer func(state *CopyoverState) error

// Global manager instance
var (
	manager *Manager

	// Track if we're in recovery mode
	recoveryMu   sync.RWMutex
	isRecovering bool

	// Build number for display
	currentBuildNumber = "unknown"
)

func init() {
	// Initialize manager with proper state based on recovery status
	initialState := StateIdle
	if os.Getenv(CopyoverEnvVar) == "1" || fileExists(CopyoverDataFile) {
		// If we're in recovery mode, start in recovering state
		initialState = StateRecovering
	}

	manager = &Manager{
		fdMap:                make(map[string]int),
		stateGatherers:       make([]StateGatherer, 0),
		stateRestorers:       make([]StateRestorer, 0),
		recoveredConnections: make([]*connections.ConnectionDetails, 0),
		currentState:         initialState,
		stateChangedAt:       time.Now(),
		status: &CopyoverStatus{
			State:          initialState,
			StateChangedAt: time.Now(),
			VetoReasons:    make([]VetoInfo, 0),
		},
		progress:           make(map[string]int),
		history:            make([]CopyoverHistory, 0),
		historyCounter:     0,
		preservedListeners: make(map[string]net.Listener),
	}
}

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

// ClearRecoveryState clears the recovery flag after copyover is complete
func (m *Manager) ClearRecoveryState() {
	recoveryMu.Lock()
	defer recoveryMu.Unlock()
	isRecovering = false
	mudlog.Info("Copyover", "status", "Recovery state cleared")
}

// ClearRecoveryState clears the recovery flag after copyover is complete
func ClearRecoveryState() {
	recoveryMu.Lock()
	defer recoveryMu.Unlock()
	isRecovering = false
	mudlog.Info("Copyover", "status", "Recovery state cleared")
}

// InitiateCopyover starts the copyover process
func (m *Manager) InitiateCopyover(countdown int) (*CopyoverResult, error) {
	// Check if we can start
	canStart, reasons := m.status.CanCopyover()
	if !canStart {
		return nil, fmt.Errorf("cannot initiate copyover: %v", reasons)
	}

	// Check module vetoes (currently only used by auctions module)
	moduleReady, moduleVetoes := CheckModuleVetoes()
	if !moduleReady {
		// Add module vetoes to status
		m.status.VetoReasons = append(m.status.VetoReasons, moduleVetoes...)
		return nil, fmt.Errorf("cannot initiate copyover: modules not ready")
	}

	m.mu.Lock()
	if m.inProgress {
		m.mu.Unlock()
		return nil, fmt.Errorf("copyover already in progress")
	}
	m.inProgress = true
	m.startTime = time.Now()

	// Update status
	m.status.StartedAt = m.startTime
	m.status.ScheduledAt = m.startTime
	if countdown > 0 {
		m.status.ScheduledFor = m.startTime.Add(time.Duration(countdown) * time.Second)
	}
	m.mu.Unlock()

	// Start with scheduled or building state
	if countdown > 0 {
		if err := m.changeState(StateScheduled); err != nil {
			return nil, err
		}

		// Fire scheduled event
		events.AddToQueue(events.CopyoverScheduled{
			ScheduledAt: time.Now(),
			Countdown:   countdown,
			Reason:      m.status.Reason,
			InitiatedBy: m.status.InitiatedBy,
		})
	} else {
		if err := m.changeState(StateBuilding); err != nil {
			return nil, err
		}
	}

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

		// Return to idle state if we're not in recovery
		if m.currentState != StateRecovering {
			m.changeState(StateIdle)
		}
	}()

	result := &CopyoverResult{
		Success: false,
	}

	// Announce copyover with countdown
	if countdown > 0 {
		if err := m.changeState(StateAnnouncing); err != nil {
			return nil, err
		}
		if err := m.announceCountdown(countdown); err != nil {
			result.Error = fmt.Sprintf("failed to announce: %v", err)
			m.changeState(StateFailed)
			return result, err
		}
		if err := m.changeState(StateBuilding); err != nil {
			return nil, err
		}
	}

	// First, build the new executable
	m.announceTemplate("copyover/copyover-building", nil)
	mudlog.Info("Copyover", "status", "Building new executable")

	m.updateProgress("build", 0)
	if err := m.buildExecutable(); err != nil {
		m.announceTemplate("copyover/copyover-build-failed", nil)
		result.Error = fmt.Sprintf("failed to build: %v", err)
		m.changeState(StateFailed)
		return result, err
	}
	m.updateProgress("build", 100)

	mudlog.Info("Copyover", "status", "Build successful")
	m.announceTemplate("copyover/copyover-build-complete", nil)

	// Transition to saving state
	if err := m.changeState(StateSaving); err != nil {
		return nil, err
	}

	// Prepare modules for copyover (currently only used by auctions module)
	if err := PrepareModulesForCopyover(); err != nil {
		mudlog.Error("Copyover", "error", "Some modules failed to prepare", "err", err)
		// Continue anyway - non-critical for auctions
	}

	// Save all active users before gathering state
	m.announceTemplate("copyover/copyover-saving", nil)
	m.updateProgress("save", 0)
	if err := m.saveAllUsers(); err != nil {
		mudlog.Error("Copyover", "error", "Failed to save some users", "err", err)
		// Continue anyway - better to copyover with some users not saved than to fail
	}
	m.updateProgress("save", 100)

	// Transition to gathering state
	if err := m.changeState(StateGathering); err != nil {
		return nil, err
	}

	mudlog.Info("Copyover", "status", "Gathering state")
	m.updateProgress("gather", 0)

	// Gather current state
	state, err := m.gatherState()
	if err != nil {
		result.Error = fmt.Sprintf("failed to gather state: %v", err)
		m.changeState(StateFailed)
		return result, err
	}
	m.updateProgress("gather", 100)

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

	// Transition to executing
	if err := m.changeState(StateExecuting); err != nil {
		return nil, err
	}

	// Execute new process
	if err := m.executeNewProcess(extraFiles); err != nil {
		result.Error = fmt.Sprintf("failed to execute: %v", err)
		m.changeState(StateFailed)
		return result, err
	}

	// If we get here, exec failed (shouldn't happen)
	result.Error = "exec returned unexpectedly"
	return result, fmt.Errorf("exec returned unexpectedly")
}

// RecoverFromCopyover restores state after a copyover
func (m *Manager) RecoverFromCopyover() error {
	mudlog.Info("Copyover", "status", "Starting recovery")

	// Check if we're already in recovering state (from init)
	if m.currentState != StateRecovering {
		// Set recovering state
		if err := m.changeState(StateRecovering); err != nil {
			return err
		}
	}

	// Mark that we're recovering
	recoveryMu.Lock()
	isRecovering = true
	recoveryMu.Unlock()

	m.updateProgress("restore", 0)

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

	// Update progress as we restore
	m.updateProgress("restore", 25)

	// Call all registered restorers
	totalRestorers := len(m.stateRestorers)
	for i, restorer := range m.stateRestorers {
		if err := restorer(state); err != nil {
			mudlog.Error("Copyover", "error", "Restorer failed", "err", err)
			// Continue with other restorers even if one fails
		}
		// Update progress (25-100%)
		progress := 25 + (75 * (i + 1) / totalRestorers)
		m.updateProgress("restore", progress)
	}

	m.updateProgress("restore", 100)
	mudlog.Info("Copyover", "status", "Recovery complete")

	// Transition back to idle
	if err := m.changeState(StateIdle); err != nil {
		return err
	}

	// Fire completion event
	events.AddToQueue(events.CopyoverCompleted{
		Duration:         time.Since(m.startTime),
		BuildNumber:      currentBuildNumber,
		OldBuildNumber:   "", // TODO: Track old build number
		ConnectionsSaved: len(state.Connections),
		ConnectionsLost:  0, // TODO: Track lost connections
		StartTime:        m.startTime,
		EndTime:          time.Now(),
	})

	// Add to history
	m.addToHistory(CopyoverHistory{
		StartedAt:        m.startTime,
		CompletedAt:      time.Now(),
		Duration:         time.Since(m.startTime),
		Success:          true,
		InitiatedBy:      m.status.InitiatedBy,
		Reason:           m.status.Reason,
		BuildNumber:      currentBuildNumber,
		OldBuildNumber:   "", // TODO: Track old build number
		ConnectionsSaved: len(state.Connections),
		ConnectionsLost:  0,
	})

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

	// Emit gather state event for systems to save their state
	events.AddToQueue(events.CopyoverGatherState{
		Phase: "gathering",
	})

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

	// Log extra files being passed
	mudlog.Info("Copyover", "debug", "Passing file descriptors", "count", len(extraFiles))
	for i, f := range extraFiles {
		if f != nil {
			// Get file info to debug what we're passing
			if stat, err := f.Stat(); err == nil {
				mudlog.Info("Copyover", "debug", "FD", "index", i+3, "name", f.Name(), "mode", stat.Mode())
			} else {
				mudlog.Info("Copyover", "debug", "FD", "index", i+3, "name", f.Name())
			}
		}
	}

	// Start the new process
	if err := cmd.Start(); err != nil {
		return err
	}

	// Give the child process a moment to start
	time.Sleep(100 * time.Millisecond)

	// Original process should exit
	mudlog.Info("Copyover", "status", "Original process exiting")

	// Close all file descriptors we passed to child
	for _, f := range extraFiles {
		if f != nil {
			f.Close()
		}
	}

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

// State machine methods

// changeState transitions to a new state if valid
func (m *Manager) changeState(newState CopyoverPhase) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate transition
	if !m.currentState.CanTransitionTo(newState) {
		return fmt.Errorf("invalid state transition from %s to %s", m.currentState, newState)
	}

	oldState := m.currentState
	m.currentState = newState
	m.stateChangedAt = time.Now()

	// Update status
	m.status.State = newState
	m.status.StateChangedAt = m.stateChangedAt

	// Clear progress when entering new state
	switch newState {
	case StateBuilding:
		m.progress["build"] = 0
	case StateSaving:
		m.progress["save"] = 0
	case StateGathering:
		m.progress["gather"] = 0
	case StateRecovering:
		m.progress["restore"] = 0
	}

	// Fire state change event
	events.AddToQueue(events.CopyoverPhaseChange{
		OldState: oldState.String(),
		NewState: newState.String(),
		Progress: m.status.GetProgress(),
	})

	mudlog.Info("Copyover", "state_change", fmt.Sprintf("%s -> %s", oldState, newState))

	return nil
}

// updateProgress updates progress for the current phase
func (m *Manager) updateProgress(phase string, progress int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	m.progress[phase] = progress

	// Update status progress
	switch m.currentState {
	case StateBuilding:
		m.status.BuildProgress = progress
	case StateSaving:
		m.status.SaveProgress = progress
	case StateGathering:
		m.status.GatherProgress = progress
	case StateRecovering:
		m.status.RestoreProgress = progress
	}

	// Fire progress event if significant change (every 10%)
	if progress%10 == 0 {
		events.AddToQueue(events.CopyoverPhaseChange{
			OldState: m.currentState.String(),
			NewState: m.currentState.String(),
			Progress: m.status.GetProgress(),
		})
	}
}

// GetStatus returns the current copyover status
func (m *Manager) GetStatus() *CopyoverStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	statusCopy := *m.status
	statusCopy.VetoReasons = make([]VetoInfo, len(m.status.VetoReasons))
	copy(statusCopy.VetoReasons, m.status.VetoReasons)

	return &statusCopy
}

// GetHistory returns copyover history
func (m *Manager) GetHistory(limit int) []CopyoverHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// Return most recent first
	result := make([]CopyoverHistory, limit)
	start := len(m.history) - limit
	copy(result, m.history[start:])

	// Reverse to get newest first
	for i := 0; i < len(result)/2; i++ {
		j := len(result) - 1 - i
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// addToHistory adds a completed copyover to history
func (m *Manager) addToHistory(record CopyoverHistory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.historyCounter++
	record.ID = m.historyCounter

	m.history = append(m.history, record)

	// Keep only last 100 records
	if len(m.history) > 100 {
		m.history = m.history[len(m.history)-100:]
	}

	// Update statistics
	m.status.TotalCopyovers++
	m.status.LastCopyoverAt = record.CompletedAt

	// Calculate average duration from successful copyovers
	var totalDuration time.Duration
	var successCount int
	for _, h := range m.history {
		if h.Success {
			totalDuration += h.Duration
			successCount++
		}
	}
	if successCount > 0 {
		m.status.AverageDuration = totalDuration / time.Duration(successCount)
	}
}

// ScheduleCopyover schedules a copyover to occur at a specific time
func (m *Manager) ScheduleCopyover(when time.Time, initiatedBy int, reason string) error {
	// Check if we can schedule
	canStart, reasons := m.status.CanCopyover()
	if !canStart {
		return fmt.Errorf("cannot schedule copyover: %v", reasons)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check current state
	if m.currentState != StateIdle {
		return fmt.Errorf("cannot schedule copyover: system is in %s state", m.currentState)
	}

	// Validate time
	if when.Before(time.Now()) {
		return fmt.Errorf("cannot schedule copyover in the past")
	}

	// Update status
	m.status.ScheduledAt = time.Now()
	m.status.ScheduledFor = when
	m.status.InitiatedBy = initiatedBy
	m.status.Reason = reason

	// We need to unlock before calling changeState
	m.mu.Unlock()
	if err := m.changeState(StateScheduled); err != nil {
		return err
	}
	m.mu.Lock()

	// Fire scheduled event
	events.AddToQueue(events.CopyoverScheduled{
		ScheduledAt: time.Now(),
		Countdown:   int(time.Until(when).Seconds()),
		Reason:      reason,
		InitiatedBy: initiatedBy,
	})

	// Start a goroutine to trigger the copyover at the scheduled time
	go func() {
		duration := time.Until(when)
		if duration > 0 {
			// Use a timer so we can stop it if needed
			timer := time.NewTimer(duration)
			defer timer.Stop()

			select {
			case <-timer.C:
				// Check if still scheduled (might have been cancelled)
				m.mu.RLock()
				stillScheduled := m.currentState == StateScheduled
				m.mu.RUnlock()

				if stillScheduled {
					// Initiate the copyover
					m.InitiateCopyover(0) // No countdown, we already waited
				}
			}
		}
	}()

	return nil
}

// CancelCopyover cancels a scheduled or in-progress copyover
func (m *Manager) CancelCopyover(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we can cancel
	if !m.currentState.IsActive() {
		return fmt.Errorf("no copyover in progress")
	}

	// Only certain states can be cancelled
	if !m.currentState.CanTransitionTo(StateCancelling) {
		return fmt.Errorf("cannot cancel copyover in %s state", m.currentState)
	}

	// Transition to cancelling state
	if err := m.changeState(StateCancelling); err != nil {
		return err
	}

	// Fire cancellation event
	events.AddToQueue(events.CopyoverCancelled{
		Reason:      reason,
		CancelledBy: m.status.InitiatedBy,
	})

	// Announce cancellation
	m.announceTemplate("copyover/copyover-cancelled", map[string]interface{}{
		"Reason": reason,
	})

	// Clean up and return to idle
	go func() {
		time.Sleep(100 * time.Millisecond) // Give time for announcements

		// Notify modules that copyover was cancelled (currently only auctions)
		if err := CleanupModulesAfterCopyover(); err != nil {
			mudlog.Error("Copyover", "error", "Module cleanup failed", "err", err)
		}

		m.changeState(StateIdle)
		m.mu.Lock()
		m.inProgress = false
		m.mu.Unlock()
	}()

	return nil
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

// StoreListenersForCopyover stores listeners that should be preserved
func (m *Manager) StoreListenersForCopyover(listeners map[string]net.Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear any existing
	m.preservedListeners = make(map[string]net.Listener)

	// Store new ones
	for name, listener := range listeners {
		if listener != nil {
			m.preservedListeners[name] = listener
		}
	}
}

// GetPreservedListeners returns the stored listeners
func (m *Manager) GetPreservedListeners() map[string]net.Listener {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	result := make(map[string]net.Listener)
	for name, listener := range m.preservedListeners {
		result[name] = listener
	}
	return result
}

// SetBuildNumber sets the build number for display
func SetBuildNumber(bn string) {
	currentBuildNumber = bn
}

// GetBuildNumber returns the current build number
func GetBuildNumber() string {
	return currentBuildNumber
}
