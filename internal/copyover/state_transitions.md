# Copyover State Machine Documentation

## State Transitions

The copyover system uses a finite state machine to manage the copyover process. Each state represents a specific phase of the copyover operation, and transitions between states are strictly controlled.

## States

### 1. StateIdle
- **Description**: No copyover in progress
- **Initial State**: Yes
- **Terminal State**: Yes
- **Can Transition To**: StateScheduled, StateBuilding

### 2. StateScheduled
- **Description**: Copyover has been scheduled but not started
- **Can Transition To**: StateAnnouncing, StateCancelling

### 3. StateAnnouncing
- **Description**: Sending countdown announcements to players
- **Can Transition To**: StateBuilding, StateCancelling

### 4. StateBuilding
- **Description**: Building the new executable
- **Can Transition To**: StateSaving, StateFailed, StateCancelling

### 5. StateSaving
- **Description**: Saving player and world state to disk
- **Can Transition To**: StateGathering, StateFailed

### 6. StateGathering
- **Description**: Gathering state from all subsystems
- **Can Transition To**: StateExecuting, StateFailed

### 7. StateExecuting
- **Description**: Executing the new process
- **Can Transition To**: StateRecovering, StateFailed

### 8. StateRecovering
- **Description**: Recovering state in the new process
- **Can Transition To**: StateIdle, StateFailed

### 9. StateCancelling
- **Description**: Cancellation in progress
- **Can Transition To**: StateIdle

### 10. StateFailed
- **Description**: Copyover failed
- **Terminal State**: Yes
- **Can Transition To**: StateIdle

## State Transition Diagram

```
                    ┌─────────────┐
                    │  StateIdle  │◄────────────────┐
                    └──────┬──────┘                 │
                           │                        │
                 ┌─────────┴─────────┐              │
                 ▼                   ▼              │
         ┌──────────────┐    ┌──────────────┐      │
         │StateScheduled│    │StateBuilding │      │
         └──────┬───────┘    └──────┬───────┘      │
                │                    │              │
       ┌────────┴────┐               │              │
       ▼             ▼               ▼              │
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│StateAnnouncing│  │StateCancelling│  │ StateSaving │
└──────┬───────┘  └──────┬───────┘  └──────┬──────┘
       │                 │                  │       │
       ▼                 ▼                  ▼       │
┌──────────────┐         │           ┌──────────────┐
│StateBuilding │◄────────┘           │StateGathering│
└──────┬───────┘                     └──────┬──────┘
       │                                    │       │
       ▼                                    ▼       │
┌──────────────┐                     ┌──────────────┐
│ StateFailed  │                     │StateExecuting│
└──────┬───────┘                     └──────┬──────┘
       │                                    │       │
       │                                    ▼       │
       │                             ┌──────────────┐
       │                             │StateRecovering│
       │                             └──────┬──────┘
       │                                    │       │
       └────────────────────────────────────┴───────┘
```

## Validation Rules

1. **Initial State**: The system always starts in `StateIdle`
2. **Terminal States**: Only `StateIdle` and `StateFailed` are terminal states
3. **Active States**: All states except `StateIdle` and `StateFailed` are considered active
4. **Cancellation**: Only certain states (`StateScheduled`, `StateAnnouncing`, `StateBuilding`) can be cancelled
5. **Failure Recovery**: From `StateFailed`, the only valid transition is back to `StateIdle`

## Progress Tracking

The system tracks progress through each phase:

- **StateBuilding**: 0-25% of total progress
- **StateSaving**: 25-50% of total progress  
- **StateGathering**: 50-75% of total progress
- **StateExecuting**: Fixed at 75%
- **StateRecovering**: 75-100% of total progress

## Event Notifications

The system fires `CopyoverPhaseChange` events when transitioning between states, including:
- Old state name
- New state name
- Current overall progress percentage

## Usage Example

```go
// Check if we can initiate copyover
canStart, reasons := manager.GetStatus().CanCopyover()
if !canStart {
    // Handle veto reasons
}

// Initiate copyover with 30 second countdown
result, err := manager.InitiateCopyover(30)
```

## Implementation Notes

1. All state transitions are atomic and thread-safe
2. Invalid transitions return an error and leave the state unchanged
3. The state machine ensures copyover operations follow the correct sequence
4. Progress updates trigger events at 10% intervals for UI updates