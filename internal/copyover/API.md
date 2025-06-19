# Copyover API Documentation

## Overview
The copyover package provides APIs for managing hot-reload server restarts without disconnecting players.

## Manager API

### GetManager() *Manager
Returns the global copyover manager instance.
```go
mgr := copyover.GetManager()
```

### Status and Information APIs

#### GetStatus() *CopyoverStatus
Returns the current copyover system status including state, progress, and statistics.
```go
status := mgr.GetStatus()
fmt.Printf("Current state: %s\n", status.State)
fmt.Printf("Progress: %d%%\n", status.GetProgress())
```

**Returns**: A copy of the current CopyoverStatus struct containing:
- `State`: Current CopyoverPhase
- `Progress`: Overall progress percentage (0-100)
- `VetoReasons`: Active vetoes preventing copyover
- `Statistics`: Total copyovers, average duration, last copyover time

#### GetHistory(limit int) []CopyoverHistory
Returns recent copyover history, newest first.
```go
history := mgr.GetHistory(10) // Get last 10 copyovers
for _, h := range history {
    fmt.Printf("[%s] %s - Duration: %s\n", 
        h.StartedAt, h.Success ? "SUCCESS" : "FAILED", h.Duration)
}
```

**Parameters**:
- `limit`: Maximum number of history records to return (0 = all)

**Returns**: Slice of CopyoverHistory records

#### IsInProgress() bool
Returns true if a copyover is currently in progress.
```go
if mgr.IsInProgress() {
    fmt.Println("Copyover in progress!")
}
```

### Control APIs

#### InitiateCopyover(countdown int) (*CopyoverResult, error)
Initiates a copyover with an optional countdown period.
```go
// Immediate copyover
result, err := mgr.InitiateCopyover(0)

// Copyover with 30 second countdown
result, err := mgr.InitiateCopyover(30)
```

**Parameters**:
- `countdown`: Seconds to wait before copyover (0 = immediate)

**Returns**: 
- `*CopyoverResult`: Result containing success status and statistics
- `error`: Error if copyover cannot be initiated

**Note**: This function will not return on success as the process is replaced.

#### ScheduleCopyover(when time.Time, initiatedBy int, reason string) error
Schedules a copyover to occur at a specific future time.
```go
// Schedule copyover for 5 minutes from now
when := time.Now().Add(5 * time.Minute)
err := mgr.ScheduleCopyover(when, user.UserId, "Scheduled maintenance")
```

**Parameters**:
- `when`: Time when copyover should occur
- `initiatedBy`: User ID of admin scheduling the copyover
- `reason`: Human-readable reason for the copyover

**Returns**: Error if scheduling fails

#### CancelCopyover(reason string) error
Cancels a scheduled or in-progress copyover (if possible).
```go
err := mgr.CancelCopyover("Emergency cancellation by admin")
```

**Parameters**:
- `reason`: Reason for cancellation

**Returns**: Error if cancellation fails

**Note**: Only certain states can be cancelled (Scheduled, Announcing, Building).

### System Integration

Systems integrate with copyover through the central registry in `integrations.go`:
```go
// Example system integration
var registeredSystems = []SystemIntegration{
    {
        Name: "MySystem",
        Gather: func() (interface{}, error) {
            // Collect and return state to preserve
            return myState, nil
        },
        Restore: func(data interface{}) error {
            if state, ok := data.(*MyState); ok {
                // Restore state from copyover data
                return restoreMyState(state)
            }
            return fmt.Errorf("invalid state type")
        },
    },
}
```

The manager automatically calls all registered Gather functions during state collection and Restore functions during recovery.

## Status API Methods

### CopyoverStatus Methods

#### CanCopyover() (bool, []string)
Checks if a copyover can be initiated and returns any blocking reasons.
```go
canStart, reasons := status.CanCopyover()
if !canStart {
    for _, reason := range reasons {
        fmt.Printf("Blocked: %s\n", reason)
    }
}
```

#### GetProgress() int
Returns the overall copyover progress as a percentage (0-100).
```go
progress := status.GetProgress()
```

#### GetTimeUntilCopyover() time.Duration
Returns the duration until a scheduled copyover (0 if not scheduled).
```go
duration := status.GetTimeUntilCopyover()
if duration > 0 {
    fmt.Printf("Copyover in %s\n", duration)
}
```

## State Machine

### CopyoverPhase States
- `StateIdle`: No copyover in progress
- `StateScheduled`: Copyover has been scheduled
- `StateBuilding`: Building new executable
- `StateTransferring`: Saving state and transferring to new process
- `StateRecovering`: Recovering state in new process
- `StateAborted`: Copyover was cancelled or aborted

### State Methods

#### String() string
Returns the string representation of a state.
```go
fmt.Println(state.String()) // "building"
```

#### IsTerminal() bool
Returns true if this is a terminal state (Idle or Failed).

#### IsActive() bool
Returns true if copyover is in progress (not Idle or Failed).

#### CanTransitionTo(target CopyoverPhase) bool
Checks if a transition to the target state is valid.

## Events

The copyover system fires the following events:

### CopyoverScheduled
Fired when a copyover is scheduled.
```go
type CopyoverScheduled struct {
    ScheduledAt time.Time
    Countdown   int
    Reason      string
    InitiatedBy int
}
```

### CopyoverPhaseChange
Fired when the copyover system transitions between phases.
```go
type CopyoverPhaseChange struct {
    OldState string
    NewState string
    Progress int
}
```

### CopyoverCancelled
Fired when a copyover is cancelled.
```go
type CopyoverCancelled struct {
    Reason      string
    CancelledBy int
}
```

### CopyoverCompleted
Fired after successful copyover.
```go
type CopyoverCompleted struct {
    Duration         time.Duration
    BuildNumber      string
    OldBuildNumber   string
    ConnectionsSaved int
    ConnectionsLost  int
    StartTime        time.Time
    EndTime          time.Time
}
```

## Helper Functions

### IsCopyoverRecovery() bool
Returns true if the server is starting from a copyover.
```go
if copyover.IsCopyoverRecovery() {
    // Handle recovery mode
}
```

### SetBuildNumber(bn string)
Sets the build number for display in copyover messages.
```go
copyover.SetBuildNumber("v1.2.3-abc123")
```

### GetBuildNumber() string
Returns the current build number.

## Error Handling

Common errors returned by the API:
- "copyover already in progress"
- "cannot initiate copyover: [reasons]"
- "cannot schedule copyover in the past"
- "no copyover in progress"
- "cannot cancel copyover in [state] state"
- "invalid state transition from [state] to [state]"

## Usage Examples

### Basic Copyover
```go
mgr := copyover.GetManager()

// Check if we can copyover
status := mgr.GetStatus()
canStart, reasons := status.CanCopyover()
if !canStart {
    log.Printf("Cannot copyover: %v", reasons)
    return
}

// Initiate with 30 second countdown
result, err := mgr.InitiateCopyover(30)
if err != nil {
    log.Printf("Copyover failed: %v", err)
}
```

### Scheduled Copyover
```go
mgr := copyover.GetManager()

// Schedule for 3am
now := time.Now()
scheduled := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())

err := mgr.ScheduleCopyover(scheduled, adminId, "Nightly maintenance")
if err != nil {
    log.Printf("Failed to schedule: %v", err)
}
```

### Status Monitoring
```go
mgr := copyover.GetManager()
status := mgr.GetStatus()

if status.State.IsActive() {
    fmt.Printf("Copyover in progress: %s (%d%%)\n", 
        status.State, status.GetProgress())
    
    if status.State == 1 { // StateScheduled
        fmt.Printf("Starting in: %s\n", status.GetTimeUntilCopyover())
    }
}
```