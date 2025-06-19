# Pet/Minion System Copyover Integration

This document describes how the pet and minion systems handle copyover (hot-reload) operations in GoMud.

## Overview

GoMud has two types of companions: permanent pets (stored in character data) and temporary followers (charmed mobs/mercenaries). Permanent pets are automatically preserved through character persistence, while temporary followers require special copyover handling.

## Architecture

### Companion Types

1. **Permanent Pets**
   - Stored in `Character.Pet`
   - Automatically persisted with character YAML
   - Provide stat bonuses and buffs
   - Can carry items
   - Have hunger system
   - One pet per character limit

2. **Charmed Mobs**
   - Tracked in `Character.CharmedMobs[]`
   - Temporary or permanent based on charm type
   - Full mob AI and abilities
   - Limited by Tame skill level
   - Require copyover preservation

3. **Mercenaries** (Future)
   - Hired for specific duration
   - Similar to charmed mobs
   - Would require copyover handling

### Automatic Preservation

The following pet data is automatically preserved:

1. **Pet Data** (`Character.Pet`)
   - Pet name and type
   - Hunger level and last meal round
   - Items carried by pet
   - All pet attributes

2. **Charmed Mob List** (`Character.CharmedMobs`)
   - Array of mob instance IDs
   - Preserved with character data

### Manual Preservation Required

1. **Charmed Mob State**
   - The `Charmed` data on the mob itself
   - Charm duration/type information
   - Mob location and instance ID

2. **Mercenary State** (Future)
   - Hiring duration remaining
   - Contract details

## Implementation Details

### State Gathering

When copyover begins:
1. The copyover manager calls the Pet system's Gather function
2. Scans all online players for charmed mobs
3. Captures charm relationship data from both sides
4. Records mob locations and instance IDs
5. Returns state for central copyover system to save

### State Restoration

After copyover:
1. The copyover manager calls the Pet system's Restore function
2. Receives deserialized state from central system
3. Restores charmed relationships on mobs
4. Validates owner's charmed mob lists
5. Ensures bidirectional consistency
6. Removes invalid references

### Validation

Post-copyover validation ensures:
- Charmed mobs still exist
- Mob instance IDs match
- Owner references are consistent
- Dead/missing mobs are cleaned up

## Pet System Behavior

### What Continues Seamlessly

**Permanent Pets**:
- Pet remains with owner
- Name and customization preserved
- Inventory intact
- Hunger level continues
- Stat bonuses active

**Charmed Mobs**:
- Charm relationships maintained
- Mob continues following
- Charm duration preserved
- Combat assistance continues

### What Resets

- Pet emote states
- In-progress pet commands
- Mob AI state (resumes normal AI)
- Follow distances

## Integration Points

### Copyover System
The pet system is registered with the central copyover manager through:
```go
{
    Name: "Pets",
    Gather: func() (interface{}, error) {
        return gatherPetsState()
    },
    Restore: func(data interface{}) error {
        if state, ok := data.(*PetCopyoverState); ok {
            return restorePetsState(state)
        }
        return fmt.Errorf("invalid pets state type")
    },
}
```

### Files
- `internal/copyover/subsystem_handlers.go` - Pet system copyover implementation
- `internal/copyover/integrations.go` - Centralized integration registry
- `pet_copyover.dat` - Temporary state file

### Dependencies
- Combat system preserves charmed mob combat state
- Mob instance IDs must be consistent (handled by combat copyover)

## Testing Scenarios

### 1. Permanent Pet Preservation
```
1. Buy a pet from shop
2. Name the pet
3. Give items to pet
4. Initiate copyover
5. Verify:
   - Pet still present
   - Name preserved
   - Items intact
   - Can interact normally
```

### 2. Charmed Mob Continuity
```
1. Charm one or more mobs
2. Move to different room with charmed mobs
3. Initiate copyover
4. Verify:
   - Mobs still charmed
   - Still following
   - Respond to commands
   - Charm duration continues
```

### 3. Mixed Companions
```
1. Have both a pet and charmed mobs
2. Engage in combat with both helping
3. Copyover during combat
4. Verify all relationships preserved
```

## Best Practices

### For Builders

1. **Pet Design**: Balance stat bonuses and abilities
2. **Charm Limits**: Set appropriate skill requirements
3. **Pet Types**: Create variety with different benefits

### For Players

1. **Pet Care**: Pets are permanent unless replaced
2. **Charm Management**: Monitor charm durations
3. **Item Storage**: Use mules for carrying capacity

### For Developers

1. **Pet Data**: Store in `Character.Pet` for auto-persistence
2. **Followers**: Use `Character.CharmedMobs` array
3. **Validation**: Always validate relationships post-copyover

## Edge Cases

### 1. Mob Death During Copyover
- Dead mobs removed from charmed lists
- No error thrown, graceful cleanup

### 2. Skill Level Changes
- If Tame skill drops, excess charmed mobs may be released
- Validated post-copyover based on current skill

### 3. Pet Replacement
- Only one pet allowed
- Old pet lost when new one purchased
- No copyover issues since it's atomic

### 4. Room Destruction
- If room no longer exists, mobs are lost
- Charmed mob references cleaned up

## Comparison with Other Systems

**Combat System**:
- Handles charmed mob aggro state
- Preserves mob instance IDs
- Pet/minion system relies on this

**Follow Module**:
- Separate from pet system
- Handles temporary following
- Not used for permanent pets

## Technical Notes

### Mob Instance Preservation
Charmed mobs retain their instance IDs across copyover because:
1. Combat system preserves mob instance counter
2. Mobs respawn with same instance IDs
3. Relationships can be restored by ID lookup

### Performance Considerations
- Validation is O(n*m) where n=players, m=rooms
- Acceptable for typical MUD scales
- Could optimize with mob location index

## Future Enhancements

1. **Mercenary System**: Add hired follower support
2. **Pet Commands**: Preserve in-flight pet commands
3. **Pet Positions**: Save exact pet positions in room
4. **Mount System**: Integrate rideable pets
5. **Pet Skills**: Add learnable pet abilities

## Limitations

1. **Pet AI State**: Not preserved (uses fresh AI)
2. **Follow Distance**: Resets to default
3. **Pet Emotions**: Emote states not preserved
4. **Complex Commands**: Multi-step pet commands interrupted