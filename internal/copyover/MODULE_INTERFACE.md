# Module Participation Interface Documentation

## Overview

The Module Participation Framework allows GoMud modules to participate in the copyover process by saving and restoring their state, vetoing copyovers when not ready, and preparing for server restarts.

## Interface Definition

### CopyoverModule

All modules that want to participate in copyover must implement the `CopyoverModule` interface:

```go
type CopyoverModule interface {
    SaveState() error
    RestoreState() error
}
```

This simplified interface allows modules to manage their own state persistence.

### Method Details

#### SaveState() error
Called before copyover to save module-specific state to disk.

**When called:** During the `StateTransferring` phase of copyover

**Requirements:**
- Must write state to a file that can be read after copyover
- Should complete quickly (< 1 second)
- Should handle its own serialization and file management
- Return nil if no state needs preservation

**Example:**
```go
func (m *AuctionModule) SaveState() error {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    state := AuctionState{
        ActiveAuctions: m.activeAuctions,
        Bids:          m.currentBids,
        NextAuctionID: m.nextID,
    }
    
    data, err := json.Marshal(state)
    if err != nil {
        return fmt.Errorf("failed to marshal auction state: %w", err)
    }
    
    return os.WriteFile("auction_copyover.dat", data, 0644)
}
```

#### RestoreState() error
Called after copyover to restore the module's state from disk.

**When called:** During the `StateRecovering` phase of copyover

**Requirements:**
- Must read state from the file written by SaveState()
- Should validate restored data
- Must be idempotent (safe to call multiple times)
- Should clean up state files after successful restore

**Example:**
```go
func (m *AuctionModule) RestoreState() error {
    data, err := os.ReadFile("auction_copyover.dat")
    if err != nil {
        if os.IsNotExist(err) {
            // No state to restore
            return nil
        }
        return fmt.Errorf("failed to read auction state: %w", err)
    }
    
    var state AuctionState
    if err := json.Unmarshal(data, &state); err != nil {
        return fmt.Errorf("failed to unmarshal auction state: %w", err)
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.activeAuctions = state.ActiveAuctions
    m.currentBids = state.Bids
    m.nextID = state.NextAuctionID
    
    // Restart auction timers
    for _, auction := range m.activeAuctions {
        m.scheduleAuctionEnd(auction)
    }
    
    // Clean up state file
    os.Remove("auction_copyover.dat")
    
    return nil
}
```

## Registration

Modules must register themselves to participate in copyover:

```go
func init() {
    module := &MyModule{
        // initialization
    }
    
    copyover.RegisterModule("mymodule", module)
}
```

The copyover manager will automatically call SaveState() and RestoreState() at the appropriate times.

## State Data Guidelines

### Serialization Requirements

State data must be JSON-serializable. Use these types:
- Basic types: string, int, float64, bool
- Slices and maps of basic types
- Structs with exported fields and json tags
- time.Time (automatically handled)

### What to Save

**Essential State:**
- Active transactions/auctions/trades
- Temporary game state (buffs, cooldowns)
- Queue contents
- Timers and scheduled events
- Module-specific IDs and counters

**What NOT to Save:**
- Network connections (recreate instead)
- File handles
- Goroutine references
- Channel references
- Function pointers

### State Size Considerations

- Keep state data reasonably sized (< 1MB per module)
- Consider compression for large data sets
- Store references (IDs) instead of full objects when possible

## Best Practices

### 1. Fast Operations
All interface methods should complete quickly:
- GatherState: < 1 second
- RestoreState: < 2 seconds  
- CanCopyover: < 100ms
- PrepareCopyover: < 500ms
- CleanupCopyover: < 500ms

### 2. Error Handling
- Log errors with context
- Don't panic in interface methods
- Return meaningful error messages
- Continue operation when possible

### 3. Thread Safety
- Use proper locking in all methods
- Avoid deadlocks between methods
- Keep critical sections small

### 4. Veto Guidelines

**Use Hard Vetoes For:**
- Critical operations that cannot be interrupted
- Data corruption risks
- User-facing transactions in progress

**Use Soft Vetoes For:**
- Performance warnings
- Non-critical operations
- Informational alerts

### 5. State Validation
Always validate restored state:
```go
func (m *Module) RestoreState(data interface{}) error {
    state, ok := data.(ModuleState)
    if !ok {
        return fmt.Errorf("invalid state type")
    }
    
    // Validate data
    if state.Version != CurrentVersion {
        return fmt.Errorf("incompatible state version")
    }
    
    if err := state.Validate(); err != nil {
        return fmt.Errorf("invalid state: %w", err)
    }
    
    // Restore
    m.state = state
    return nil
}
```

## Example: Complete Module Implementation

```go
package auction

import (
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"
    
    "github.com/GoMudEngine/GoMud/internal/copyover"
    "github.com/GoMudEngine/GoMud/internal/mudlog"
)

type AuctionModule struct {
    mu             sync.RWMutex
    activeAuctions map[int]*Auction
    nextID         int
    timers         map[int]*time.Timer
}

// Copyover state structure
type AuctionCopyoverState struct {
    Version        int                  `json:"version"`
    ActiveAuctions map[int]*Auction     `json:"active_auctions"`
    NextID         int                  `json:"next_id"`
    Timestamp      time.Time            `json:"timestamp"`
}

func (m *AuctionModule) SaveState() error {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // Stop all timers first
    for _, timer := range m.timers {
        timer.Stop()
    }
    
    state := AuctionCopyoverState{
        Version:        1,
        ActiveAuctions: m.activeAuctions,
        NextID:         m.nextID,
        Timestamp:      time.Now(),
    }
    
    data, err := json.Marshal(state)
    if err != nil {
        return fmt.Errorf("failed to marshal auction state: %w", err)
    }
    
    return os.WriteFile("auction_copyover.dat", data, 0644)
}

func (m *AuctionModule) RestoreState() error {
    data, err := os.ReadFile("auction_copyover.dat")
    if err != nil {
        if os.IsNotExist(err) {
            // No state to restore
            return nil
        }
        return fmt.Errorf("failed to read auction state: %w", err)
    }
    
    var state AuctionCopyoverState
    if err := json.Unmarshal(data, &state); err != nil {
        return fmt.Errorf("failed to unmarshal auction state: %w", err)
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.activeAuctions = state.ActiveAuctions
    m.nextID = state.NextID
    
    // Restart timers for active auctions
    for id, auction := range m.activeAuctions {
        remaining := time.Until(auction.EndTime)
        if remaining > 0 {
            m.timers[id] = time.AfterFunc(remaining, func() {
                m.endAuction(id)
            })
        } else {
            // Auction expired during copyover
            go m.endAuction(id)
        }
    }
    
    mudlog.Info("Auction", "status", "Restored state", 
        "auctions", len(m.activeAuctions))
    
    // Clean up state file
    os.Remove("auction_copyover.dat")
    
    return nil
}

// Register the module
func init() {
    module := &AuctionModule{
        activeAuctions: make(map[int]*Auction),
        timers:         make(map[int]*time.Timer),
    }
    
    copyover.RegisterModule("auction", module)
}
```

## Testing Module Integration

```go
// In your module test file
func TestCopyoverIntegration(t *testing.T) {
    module := NewTestModule()
    
    // Test registration
    copyover.RegisterModule("test", module)
    
    // Test state save
    err := module.SaveState()
    assert.NoError(t, err)
    
    // Verify state file exists
    _, err = os.Stat("test_copyover.dat")
    assert.NoError(t, err)
    
    // Test state restoration
    err = module.RestoreState()
    assert.NoError(t, err)
    
    // Verify state file cleaned up
    _, err = os.Stat("test_copyover.dat")
    assert.True(t, os.IsNotExist(err))
}
```

## Module Developer Checklist

- [ ] Implement SaveState() and RestoreState() methods
- [ ] State data is JSON-serializable
- [ ] Handle file I/O errors gracefully
- [ ] Clean up state files after successful restore
- [ ] Implement proper thread safety
- [ ] Test state save/restore cycle
- [ ] Register module on initialization
- [ ] Add logging for debugging
- [ ] Document state format
- [ ] Test with actual copyover