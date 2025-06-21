package copyover

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

const (
	CopyoverDataFile = "copyover.dat"
	CopyoverEnvVar   = "GOMUD_COPYOVER"
	CopyoverTimeout  = 30 * time.Second
)

// Simplified state machine - only 5 states
type CopyoverState int

const (
	StateIdle CopyoverState = iota
	StateScheduled
	StatePreparing // Combines building, saving, gathering
	StateExecuting
	StateRecovering
)

func (s CopyoverState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateScheduled:
		return "scheduled"
	case StatePreparing:
		return "preparing"
	case StateExecuting:
		return "executing"
	case StateRecovering:
		return "recovering"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// Manager handles the copyover process (simplified)
type Manager struct {
	mu                   sync.Mutex
	state                CopyoverState
	startTime            time.Time
	scheduledFor         time.Time
	initiatedBy          int
	reason               string
	progress             int // Single progress percentage
	cancelChan           chan struct{}
	timer                *time.Timer
	extraFiles           []*os.File
	preservedState       *CopyoverStateData // For recovery
	stateGatherers       []StateGatherer
	stateRestorers       []StateRestorer
	buildNumber          string
	recoveredConnections []*connections.ConnectionDetails // Recovered connections that need input handlers
}

// CopyoverOptions for the single entry point
type CopyoverOptions struct {
	Countdown    int    // Seconds to wait (0 = immediate)
	IncludeBuild bool   // Whether to rebuild executable
	Reason       string // Why copyover is happening
	InitiatedBy  int    // User ID who initiated
}

// StateGatherer collects state before copyover
type StateGatherer func() (interface{}, error)

// StateRestorer restores state after copyover
type StateRestorer func(state *CopyoverStateData) error

var (
	manager      *Manager
	isRecovering bool
	recoveryMu   sync.RWMutex
)

func init() {
	initialState := StateIdle
	if os.Getenv(CopyoverEnvVar) == "1" {
		initialState = StateRecovering
	} else if fileExists(CopyoverDataFile) {
		// Clean up stale file
		mudlog.Warn("Copyover", "action", "Removing stale copyover.dat")
		os.Remove(CopyoverDataFile)
	}

	manager = &Manager{
		state:          initialState,
		stateGatherers: make([]StateGatherer, 0),
		stateRestorers: make([]StateRestorer, 0),
		buildNumber:    "unknown",
	}
}

// GetManager returns the global copyover manager
func GetManager() *Manager {
	return manager
}

// Copyover is the single entry point for all copyover operations
func (m *Manager) Copyover(options CopyoverOptions) error {
	m.mu.Lock()

	// Check if we can proceed
	if m.state != StateIdle {
		m.mu.Unlock()
		return fmt.Errorf("copyover already in progress (state: %s)", m.state)
	}

	// Check module vetoes
	if moduleReady, vetoes := CheckModuleVetoes(); !moduleReady {
		m.mu.Unlock()
		return fmt.Errorf("modules not ready: %v", vetoes)
	}

	// Set common fields
	m.initiatedBy = options.InitiatedBy
	m.reason = options.Reason
	m.startTime = time.Now()
	m.progress = 0

	// Handle scheduling vs immediate execution
	if options.Countdown > 0 {
		m.state = StateScheduled
		m.scheduledFor = time.Now().Add(time.Duration(options.Countdown) * time.Second)
		m.cancelChan = make(chan struct{})

		// Create timer for scheduled execution
		m.timer = time.NewTimer(time.Duration(options.Countdown) * time.Second)
		m.mu.Unlock()

		// Announce scheduling
		mudlog.Info("Copyover", "action", "Scheduled copyover", "countdown", options.Countdown)
		m.announce("copyover/copyover-announce", map[string]interface{}{
			"Seconds": options.Countdown,
			"Time":    m.scheduledFor.Format("15:04:05"),
		})

		// Fire scheduled event for countdown announcements
		events.AddToQueue(events.CopyoverScheduled{
			ScheduledAt: time.Now(),
			Countdown:   options.Countdown,
			Reason:      options.Reason,
			InitiatedBy: options.InitiatedBy,
		})

		// Start goroutine to handle execution
		go m.waitAndExecute(options)
		return nil
	}

	// Immediate execution
	m.state = StatePreparing
	m.mu.Unlock()

	// Execute synchronously
	return m.execute(options)
}

// Cancel cancels a scheduled copyover
func (m *Manager) Cancel(reason string) error {
	m.mu.Lock()

	if m.state != StateScheduled {
		m.mu.Unlock()
		return fmt.Errorf("no scheduled copyover to cancel")
	}

	// Stop timer
	if m.timer != nil {
		m.timer.Stop()
	}

	// Signal cancellation
	if m.cancelChan != nil {
		close(m.cancelChan)
	}

	// Reset state
	m.state = StateIdle
	m.scheduledFor = time.Time{}
	m.mu.Unlock()

	// Announce cancellation
	m.announce("copyover/copyover-cancelled", map[string]interface{}{
		"Reason": reason,
	})

	// Notify modules
	CleanupModulesAfterCopyover()

	mudlog.Info("Copyover", "action", "Cancelled", "reason", reason)
	return nil
}

// GetStatus returns current status (simplified)
func (m *Manager) GetStatus() (state string, progress int, scheduledFor time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state.String(), m.progress, m.scheduledFor
}

// IsScheduledReady checks if a scheduled copyover should execute
func (m *Manager) IsScheduledReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateScheduled {
		return false
	}

	return time.Now().After(m.scheduledFor)
}

// IsInProgress returns true if copyover is active
func (m *Manager) IsInProgress() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state != StateIdle
}

// RegisterStateGatherer adds a state gatherer
func (m *Manager) RegisterStateGatherer(gatherer StateGatherer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateGatherers = append(m.stateGatherers, gatherer)
}

// RegisterStateRestorer adds a state restorer
func (m *Manager) RegisterStateRestorer(restorer StateRestorer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateRestorers = append(m.stateRestorers, restorer)
}

// RecoverFromCopyover restores state after copyover
func (m *Manager) RecoverFromCopyover() error {
	mudlog.Info("Copyover", "status", "Starting recovery")

	// Lock MUD during recovery
	util.LockMud()
	defer util.UnlockMud()

	// Mark recovering
	recoveryMu.Lock()
	isRecovering = true
	recoveryMu.Unlock()

	// Load and restore state
	state, err := m.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %v", err)
	}

	// Clean up state file
	os.Remove(CopyoverDataFile)

	// Restore environment
	for key, value := range state.Environment {
		os.Setenv(key, value)
	}

	// Call restorers
	for _, restorer := range m.stateRestorers {
		if err := restorer(state); err != nil {
			mudlog.Error("Copyover", "error", "Restorer failed", "err", err)
		}
	}

	// Reset to idle
	m.mu.Lock()
	m.state = StateIdle
	m.mu.Unlock()

	// DO NOT clear recovery flag here - it needs to stay set until
	// CompleteUserRecovery is called after the world is running
	mudlog.Info("Copyover", "status", "Recovery complete (connections pending)")
	return nil
}

// Private methods

func (m *Manager) waitAndExecute(options CopyoverOptions) {
	select {
	case <-m.timer.C:
		// Timer expired, execute copyover
		if err := m.execute(options); err != nil {
			mudlog.Error("Copyover", "error", "Scheduled execution failed", "err", err)
			m.announce("copyover/copyover-build-failed", nil)

			m.mu.Lock()
			m.state = StateIdle
			m.mu.Unlock()
		}
	case <-m.cancelChan:
		// Cancelled, already handled by Cancel()
		return
	}
}

func (m *Manager) execute(options CopyoverOptions) error {
	// Update state
	m.mu.Lock()
	m.state = StatePreparing
	m.progress = 0
	m.mu.Unlock()

	// Build phase - do this BEFORE locking the MUD
	if options.IncludeBuild {
		m.updateProgress(10)
		m.announce("copyover/copyover-building", nil)

		buildStart := time.Now()
		if err := m.buildExecutable(); err != nil {
			m.announce("copyover/copyover-build-failed", nil)
			m.mu.Lock()
			m.state = StateIdle
			m.mu.Unlock()
			return fmt.Errorf("build failed: %v", err)
		}

		buildDuration := time.Since(buildStart)
		mudlog.Info("Copyover", "status", "Build completed", "duration", buildDuration)
		m.updateProgress(25)
		m.announce("copyover/copyover-build-complete", nil)
	}

	// NOW lock MUD for state gathering - this should be very quick
	mudlog.Info("Copyover", "status", "Acquiring global mutex")
	lockStart := time.Now()
	util.LockMud()
	mudlog.Info("Copyover", "status", "Mutex acquired", "waitTime", time.Since(lockStart))

	// This will be unlocked by process termination on success
	execSuccess := false
	defer func() {
		if !execSuccess {
			util.UnlockMud()
		}
	}()

	// Prepare modules
	PrepareModulesForCopyover()

	// Save users
	m.updateProgress(40)
	m.announce("copyover/copyover-saving", nil)
	if err := m.saveAllUsers(); err != nil {
		mudlog.Error("Copyover", "error", "Failed to save users", "err", err)
	}

	// Gather state
	m.updateProgress(60)
	state, err := m.gatherState()
	if err != nil {
		return fmt.Errorf("failed to gather state: %v", err)
	}

	// Save state
	m.updateProgress(80)
	mudlog.Info("Copyover", "status", "About to save state", "hasState", state != nil)
	if err := m.saveState(state); err != nil {
		mudlog.Error("Copyover", "error", "saveState failed", "err", err)
		return fmt.Errorf("failed to save state: %v", err)
	}
	mudlog.Info("Copyover", "status", "State save completed")

	// Prepare file descriptors
	extraFiles, err := m.prepareFileDescriptors(state)
	if err != nil {
		return fmt.Errorf("failed to prepare FDs: %v", err)
	}

	// Execute new process
	m.updateProgress(95)
	m.announce("copyover/copyover-restarting", nil)

	m.mu.Lock()
	m.state = StateExecuting
	m.mu.Unlock()

	// Notify systems to prepare for shutdown
	// The main.go should close listeners when it gets this event
	mudlog.Info("Copyover", "status", "Notifying shutdown")
	events.AddToQueue(events.System{
		Command: "shutdown_listeners",
	})

	// Give time for listeners to close
	time.Sleep(200 * time.Millisecond)

	// Log total time the MUD was locked
	lockedDuration := time.Since(lockStart)
	mudlog.Info("Copyover", "status", "About to execute new process",
		"extraFiles", len(extraFiles),
		"lockedDuration", lockedDuration)

	execSuccess = true
	if err := m.executeNewProcess(extraFiles); err != nil {
		mudlog.Error("Copyover", "error", "executeNewProcess failed", "err", err)
		execSuccess = false
		return fmt.Errorf("failed to execute: %v", err)
	}

	// Should never reach here
	return fmt.Errorf("exec returned unexpectedly")
}

func (m *Manager) updateProgress(percent int) {
	m.mu.Lock()
	m.progress = percent
	m.mu.Unlock()
}

func (m *Manager) announce(template string, data interface{}) {
	tplText, err := templates.Process(template, data)
	if err != nil {
		mudlog.Error("Copyover", "error", "Template failed", "template", template, "err", err)
		return
	}

	// Use direct broadcast instead of events for immediate delivery
	connections.Broadcast([]byte(templates.AnsiParse(tplText) + "\r\n"))
}

func (m *Manager) buildExecutable() error {
	// Use go build directly for faster builds during copyover
	// The -a flag forces a full rebuild, but we can skip it for copyover
	mudlog.Info("Copyover", "status", "Building executable")

	// Build without -a flag for faster incremental builds
	cmd := exec.Command("go", "build", "-trimpath", "-o", "go-mud-server")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	return cmd.Run()
}

func (m *Manager) saveAllUsers() error {
	activeUsers := users.GetAllActiveUsers()
	mudlog.Info("Copyover", "info", "Saving users", "count", len(activeUsers))

	for _, user := range activeUsers {
		if err := users.SaveUser(*user); err != nil {
			mudlog.Error("Copyover", "error", "Failed to save user", "userId", user.UserId, "err", err)
		}
	}

	return nil
}

func (m *Manager) gatherState() (*CopyoverStateData, error) {
	mudlog.Info("Copyover", "status", "Starting state gathering")
	state := &CopyoverStateData{
		Version:     "1.0",
		Timestamp:   time.Now(),
		StartTime:   m.startTime,
		Environment: make(map[string]string),
		Listeners:   make(map[string]ListenerState),
		Connections: make([]ConnectionState, 0),
	}

	// Reset extra files
	m.extraFiles = make([]*os.File, 0)

	// Save environment
	for _, key := range []string{"CONFIG_PATH", "LOG_LEVEL", "LOG_PATH", "LOG_NOCOLOR", "CONSOLE_GMCP_OUTPUT"} {
		if value := os.Getenv(key); value != "" {
			state.Environment[key] = value
		}
	}

	// Fire gather event
	events.AddToQueue(events.CopyoverGatherState{
		Phase: "gathering",
	})

	// Call gatherers
	mudlog.Info("Copyover", "status", "Calling state gatherers", "count", len(m.stateGatherers))
	for i, gatherer := range m.stateGatherers {
		mudlog.Info("Copyover", "status", "Calling gatherer", "index", i)
		if _, err := gatherer(); err != nil {
			mudlog.Error("Copyover", "error", "Gatherer failed", "index", i, "err", err)
		}
	}

	// Use the preserved state that was populated by gatherers
	if m.preservedState != nil {
		mudlog.Info("Copyover", "status", "Using preserved state from gatherers",
			"listeners", len(m.preservedState.Listeners),
			"connections", len(m.preservedState.Connections))
		// Copy over the gathered data
		state.Listeners = m.preservedState.Listeners
		state.Connections = m.preservedState.Connections
	}

	m.preservedState = state
	return state, nil
}

func (m *Manager) saveState(state *CopyoverStateData) error {
	mudlog.Info("Copyover", "status", "Saving state to file", "file", CopyoverDataFile)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	compressed := util.Compress(data)
	if err := util.SafeSave(CopyoverDataFile, compressed); err != nil {
		mudlog.Error("Copyover", "error", "Failed to save state file", "err", err)
		return err
	}

	mudlog.Info("Copyover", "status", "State saved", "size", len(compressed))
	return nil
}

func (m *Manager) loadState() (*CopyoverStateData, error) {
	compressed, err := os.ReadFile(CopyoverDataFile)
	if err != nil {
		return nil, err
	}

	data := util.Decompress(compressed)
	if len(data) == 0 && len(compressed) > 0 {
		// Try uncompressed for backwards compatibility
		data = compressed
	}

	var state CopyoverStateData
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

func (m *Manager) prepareFileDescriptors(state *CopyoverStateData) ([]*os.File, error) {
	// Return the collected extra files
	return m.extraFiles, nil
}

func (m *Manager) executeNewProcess(extraFiles []*os.File) error {
	// Use the actual binary name that was built
	executable := "./go-mud-server"

	// Double-check the file exists
	if _, err := os.Stat(executable); err != nil {
		mudlog.Error("Copyover", "error", "Executable not found", "path", executable)
		// Fallback to current executable
		executable, err = os.Executable()
		if err != nil {
			return err
		}
	}

	mudlog.Info("Copyover", "status", "Starting new process", "exe", executable, "extraFiles", len(extraFiles))

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=1", CopyoverEnvVar))
	cmd.ExtraFiles = extraFiles

	if err := cmd.Start(); err != nil {
		mudlog.Error("Copyover", "error", "Failed to start new process", "err", err)
		return err
	}

	mudlog.Info("Copyover", "status", "New process started", "pid", cmd.Process.Pid)

	// Give child process time to start
	time.Sleep(100 * time.Millisecond)

	// Close FDs and exit
	mudlog.Info("Copyover", "status", "Closing FDs and exiting")
	for _, f := range extraFiles {
		if f != nil {
			f.Close()
		}
	}

	os.Exit(0)
	return nil
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Global recovery check functions

func IsCopyoverRecovery() bool {
	recoveryMu.RLock()
	defer recoveryMu.RUnlock()
	return isRecovering || os.Getenv(CopyoverEnvVar) == "1" || fileExists(CopyoverDataFile)
}

func IsRecovering() bool {
	recoveryMu.RLock()
	defer recoveryMu.RUnlock()
	return isRecovering
}

func ClearRecoveryState() {
	recoveryMu.Lock()
	defer recoveryMu.Unlock()
	isRecovering = false
	mudlog.Info("Copyover", "status", "Recovery state cleared")
}

func (m *Manager) ClearRecoveryState() {
	ClearRecoveryState()
}

// Backwards compatibility functions

func (m *Manager) InitiateCopyover(countdown int) (*CopyoverResult, error) {
	err := m.Copyover(CopyoverOptions{
		Countdown:    countdown,
		IncludeBuild: false,
	})
	return &CopyoverResult{Success: err == nil, Error: fmt.Sprintf("%v", err)}, err
}

func (m *Manager) InitiateCopyoverWithBuild(countdown int) (*CopyoverResult, error) {
	err := m.Copyover(CopyoverOptions{
		Countdown:    countdown,
		IncludeBuild: true,
	})
	return &CopyoverResult{Success: err == nil, Error: fmt.Sprintf("%v", err)}, err
}

func (m *Manager) CancelCopyover(reason string) error {
	return m.Cancel(reason)
}

func (m *Manager) ExecuteScheduledCopyover() (*CopyoverResult, error) {
	// This is now handled internally by the timer
	return &CopyoverResult{Success: true}, nil
}

func (m *Manager) GetTimeUntilCopyover() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateScheduled || m.scheduledFor.IsZero() {
		return 0
	}

	remaining := time.Until(m.scheduledFor)
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (m *Manager) GetState() (*CopyoverStateData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preservedState, nil
}

func (m *Manager) GetRecoveredConnections() []*connections.ConnectionDetails {
	return m.recoveredConnections
}

func (m *Manager) StoreListenersForCopyover(listeners map[string]net.Listener) {
	// Handled by gatherers
}

func (m *Manager) GetPreservedListeners() map[string]net.Listener {
	// Handled by restorers
	return nil
}

func SetBuildNumber(bn string) {
	manager.buildNumber = bn
}

func GetBuildNumber() string {
	return manager.buildNumber
}

func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any timer
	if m.timer != nil {
		m.timer.Stop()
	}

	// Close cancel channel
	if m.cancelChan != nil {
		close(m.cancelChan)
	}

	// Reset state
	m.state = StateIdle
	m.progress = 0
	m.scheduledFor = time.Time{}

	// Clean up files
	for _, f := range m.extraFiles {
		if f != nil {
			f.Close()
		}
	}
	m.extraFiles = nil

	// Remove state file
	if fileExists(CopyoverDataFile) {
		os.Remove(CopyoverDataFile)
	}

	return nil
}

// GetStatusStruct returns full status structure for backwards compatibility
func (m *Manager) GetStatusStruct() *CopyoverStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &CopyoverStatus{
		State:          CopyoverPhase(m.state),
		StateChangedAt: time.Now(),
		ScheduledFor:   m.scheduledFor,
		InitiatedBy:    m.initiatedBy,
		Reason:         m.reason,
		StartedAt:      m.startTime,
	}
}

func (m *Manager) GetHistory(limit int) []CopyoverHistory {
	// Simplified - no history tracking
	return []CopyoverHistory{}
}
