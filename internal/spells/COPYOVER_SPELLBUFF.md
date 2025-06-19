# Spell/Buff System Copyover Integration

This document describes how the spell and buff systems handle copyover (hot-reload) operations in GoMud.

## Overview

The spell/buff system leverages a hybrid approach for copyover: most buff and cooldown state is automatically preserved through character persistence, while active spell casting requires special handling since it's stored in transient aggro state.

## Architecture

### Automatic Preservation (via Character YAML)

The following state is automatically preserved without special copyover handling:

1. **Active Buffs** (`Character.Buffs`)
   - All buff instances with their current state
   - Round counters and triggers remaining
   - Permanent buffs from equipment/race
   - Buff flags and modifiers

2. **Cooldowns** (`Character.Cooldowns`)
   - All active cooldown timers
   - Rounds remaining for each cooldown
   - Spell and ability cooldowns

3. **Spell Knowledge** (`Character.SpellBook`)
   - Known spells and proficiency levels
   - Spell access permissions

### Manual Preservation Required

The following state requires special copyover handling:

1. **Active Spell Casting** (`ActiveSpellCast`)
   - Spells currently being cast (with wait rounds)
   - Stored in `Character.Aggro` when type is `SpellCast`
   - Includes caster, targets, and rounds remaining

2. **Room Buffs** (Future Enhancement)
   - Buffs applied to rooms rather than characters
   - Area effect spells

### Data Structures

```go
type SpellBuffCopyoverState struct {
    ActiveSpells []ActiveSpellCast
    RoomBuffs    []RoomBuffState
    GlobalState  map[string]interface{}
    SavedAt      time.Time
}

type ActiveSpellCast struct {
    CasterType    string // "user" or "mob"
    CasterId      int
    RoomId        int
    SpellId       string
    RoundsWaiting int
    SpellInfo     SpellAggroInfo
}
```

## Implementation Details

### State Gathering

When copyover begins:

1. The copyover manager calls the SpellBuff system's Gather function
2. Scans all online users and mobs
3. Identifies characters with `Aggro.Type == SpellCast`
4. Captures spell casting state
5. Returns state for central copyover system to save

### State Restoration

After copyover completes:

1. The copyover manager calls the SpellBuff system's Restore function
2. Receives deserialized state from central system
3. Recreates `Aggro` structures for active casters
4. Spell casting resumes at next round

## Buff System Behavior

### What Continues Seamlessly

- **Buff Duration**: Round counters continue from where they left off
- **Buff Effects**: Stat modifiers remain active
- **Buff Triggers**: Next trigger happens on schedule
- **Buff Expiration**: Buffs expire at the correct time
- **Cooldown Timers**: Continue counting down

### What Resets

- **Buff Scripts**: JavaScript context is recreated
- **Visual Effects**: Any client-side buff indicators
- **Buff Sounds**: Audio cues need retriggering

### Buff Validation

The buff system automatically validates buffs each round:
- `Buffs.Prune()` removes expired buffs
- Invalid buff IDs are removed
- Corrupted buff data is cleaned up

## Spell Casting Behavior

### Preserved State

- **Spell ID**: Which spell is being cast
- **Caster**: Who is casting (user/mob)
- **Targets**: Selected targets (users/mobs)
- **Cast Time**: Rounds remaining to complete cast

### Edge Cases

1. **Interrupted Casts**: If caster dies during copyover, spell is cancelled on restore
2. **Missing Targets**: If targets disappear, spell fails gracefully
3. **Invalid Spells**: If spell no longer exists, cast is cancelled
4. **Mob Casters**: Mob instance IDs must be preserved for continuity

## Integration Points

### Copyover System
The spell/buff system is registered with the central copyover manager through:
```go
{
    Name: "SpellBuff",
    Gather: func() (interface{}, error) {
        return gatherSpellBuffState()
    },
    Restore: func(data interface{}) error {
        if state, ok := data.(*SpellBuffCopyoverState); ok {
            return restoreSpellBuffState(state)
        }
        return fmt.Errorf("invalid spellbuff state type")
    },
}
```

### Files
- `internal/copyover/subsystem_handlers.go` - SpellBuff system copyover implementation
- `internal/copyover/integrations.go` - Centralized integration registry
- `spellbuff_copyover.dat` - Temporary state file

## Testing

### Unit Tests
See `copyover_test.go` for tests covering:
- State serialization/deserialization
- Active spell preservation
- File operations
- Edge case handling

### Manual Testing

1. **Buff Preservation**:
   - Apply various buffs with different durations
   - Note remaining triggers
   - Copyover
   - Verify buffs continue with correct durations

2. **Spell Casting**:
   - Begin casting a spell with wait time
   - Copyover during cast
   - Verify spell completes or is properly handled

3. **Cooldowns**:
   - Use abilities to trigger cooldowns
   - Note remaining time
   - Copyover
   - Verify cooldowns continue counting

## Best Practices

### For Spell Designers

1. **Avoid Long Casts**: Very long cast times increase interruption chance
2. **Use Standard Patterns**: Follow existing spell structures
3. **Test With Copyover**: Ensure spells handle interruption gracefully

### For Developers

1. **Buff State**: Store in `Character.Buffs` for automatic preservation
2. **Cooldowns**: Use `Character.Cooldowns` map
3. **Casting State**: Use `Aggro` with `SpellCast` type
4. **Validation**: Implement proper target validation

## Example Scenarios

### Scenario 1: Buff Across Copyover
```
1. Player casts "Shield" (10 minute duration)
2. 3 minutes pass (7 minutes remaining)
3. Copyover occurs
4. Shield continues with 7 minutes remaining
5. Shield expires on schedule
```

### Scenario 2: Spell Cast Interruption
```
1. Player begins casting "Fireball" (3 round cast)
2. After 1 round, copyover occurs
3. After copyover, spell continues with 2 rounds left
4. Spell completes and damages target
```

### Scenario 3: Cooldown Preservation
```
1. Player uses "Bash" (5 minute cooldown)
2. 2 minutes pass (3 minutes remaining)
3. Copyover occurs
4. Cooldown shows 3 minutes remaining
5. Player can use Bash again after cooldown
```

## Limitations

1. **Script State**: Buff JavaScript contexts are recreated, not preserved
2. **Complex Effects**: Multi-stage spell effects may need redesign
3. **Channel Spells**: Continuous channel spells would need special handling
4. **Ground Effects**: Persistent ground effects need room buff implementation

## Future Enhancements

1. Implement room buff preservation
2. Add spell script state preservation
3. Support channeled spell continuity
4. Create buff effect visualization system
5. Add metrics for buff/spell copyover performance