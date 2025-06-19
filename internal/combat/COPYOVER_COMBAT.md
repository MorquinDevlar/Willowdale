# Combat System Copyover Integration

This document describes how the combat system handles copyover (hot-reload) operations in GoMud.

## Overview

The combat system preserves all active combat states during copyover, allowing battles to continue seamlessly after the server restarts. This includes player vs mob, player vs player, and multi-target combat scenarios.

## Architecture

### State Preservation

The combat copyover system preserves the following state:

1. **Player Combat State** (`PlayerCombatState`)
   - User ID and Room ID
   - Active Aggro pointer (target and combat type)
   - Damage tracking map (who dealt how much damage)
   - Last damage timestamp

2. **Mob Combat State** (`MobCombatState`)
   - Mob ID and Instance ID
   - Room ID for mob location
   - Active Aggro pointer
   - Player damage tracking
   - Charmed relationships

3. **Global State**
   - Mob instance counter (ensures consistent IDs)

### File Structure

- `internal/combat/copyover.go` - Core combat state preservation
- `internal/copyover/integrations.go` - Centralized integration with copyover system
- `internal/copyover/manager.go` - Copyover state machine and coordination

## Implementation Details

### State Gathering

When copyover begins, the system:

1. The copyover manager calls the Combat system's Gather function
2. Iterates through all online players and active mobs
3. Captures combat-related state for entities in combat
4. Returns state for central copyover system to save

```go
func GatherCombatState() (*CombatCopyoverState, error) {
    // Collect player combat states
    // Collect mob combat states  
    // Save mob instance counter
}
```

### State Restoration

After copyover completes:

1. The copyover manager calls the Combat system's Restore function
2. Receives deserialized state from central system
3. Restores mob instance counter first
4. Re-establishes player combat states
5. Re-establishes mob combat states
6. Validates all aggro targets still exist

```go
func RestoreCombatState(state *CombatCopyoverState) error {
    // Restore instance counter
    // Restore player aggro
    // Restore mob aggro
    // Validate targets
}
```

## Combat Behavior During Copyover

### What Continues
- Active combat relationships (aggro)
- Damage tracking for attribution
- Mob instance IDs remain consistent
- Charmed mob relationships

### What Resets
- In-flight attack animations (resume next round)
- Spell casting with rounds waiting (cancelled)
- Combat messages in transit

### Round Processing
Combat rounds are atomic operations. Copyover occurs between rounds, so:
- No partial damage calculations
- No interrupted attack sequences
- Combat resumes at the next full round

## Integration Points

### Copyover System
The combat system is registered with the central copyover manager through:
```go
{
    Name: "Combat",
    Gather: func() (interface{}, error) {
        return combat.GatherCombatState()
    },
    Restore: func(data interface{}) error {
        if state, ok := data.(*combat.CombatCopyoverState); ok {
            return combat.RestoreCombatState(state)
        }
        return fmt.Errorf("invalid combat state type")
    },
}
```

### Dependencies
- `mobs.GetInstanceCounter()` / `SetInstanceCounter()`
- Character aggro pointers
- Room mob lists

## Error Handling

The system is resilient to:
- Missing aggro targets (clears invalid aggro)
- Changed room layouts (mobs stay in original rooms)
- Offline players (their combat state is preserved)

State save/load errors are logged but don't block copyover.

## Testing

See `combat_copyover_test.go` for unit tests covering:
- State serialization/deserialization
- Instance counter preservation
- Combat state validation

See `_datafiles/world/default/combat_copyover_test.md` for manual testing procedures.

## Limitations

1. **Spell Casting**: Spells with `RoundsWaiting > 0` are cancelled
2. **Ranged Combat**: Cross-room combat requires exit validation
3. **Temporary Effects**: Non-persistent buffs may need re-application

## Future Enhancements

1. Preserve spell casting state with round counters
2. Add combat pause flags for smoother transitions  
3. Implement combat state compression for large battles
4. Add metrics for combat copyover performance