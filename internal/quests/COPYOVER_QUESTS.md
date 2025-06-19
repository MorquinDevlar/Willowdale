# Quest System Copyover Integration

This document describes how the quest system handles copyover (hot-reload) operations in GoMud.

## Overview

The quest system leverages the existing character persistence model to maintain quest progress during copyover. Quest progress is automatically preserved as part of character data, while quest-specific timers and pending events require special handling.

## Architecture

### State Preservation

The quest copyover system preserves:

1. **Quest Progress** (Automatic)
   - Stored in `Character.QuestProgress map[int]string`
   - Automatically persisted with character saves
   - No special copyover handling needed

2. **Quest Timers** (`CharacterQuestTimers`)
   - Quest-related entries from `Character.Timers`
   - Identified by "quest" prefix in timer names
   - Preserved separately during copyover

3. **Pending Quest Events**
   - Quest events are handled by the Event Queue copyover system
   - Automatically preserved and restored with all other events

### Data Structures

```go
type QuestCopyoverState struct {
    CharacterTimers []CharacterQuestTimers
    PendingEvents   []PendingQuestEvent
    SavedAt         time.Time
}

type CharacterQuestTimers struct {
    UserId int
    Timers map[string]gametime.RoundTimer
}
```

## Implementation Details

### State Gathering

When copyover begins:

1. The copyover manager calls the Quest system's Gather function
2. Iterates through online characters
3. Extracts quest-related timers (names starting with "quest")
4. Returns state for central copyover system to save

### State Restoration

After copyover completes:

1. The copyover manager calls the Quest system's Restore function
2. Receives deserialized state from central system
3. Restores quest timers to characters
4. Validates quest progress (optional)

## Quest System Behavior

### What Is Preserved

- **Quest Progress**: Current step for each active quest
- **Quest Timers**: Time-limited quest objectives
- **Quest Tokens**: Player's achieved quest milestones
- **Quest Rewards**: Pending reward distributions

### What Resets

- **Quest NPCs**: Return to default positions/states
- **Quest Items**: In-world quest items respawn normally
- **Quest Dialogs**: Conversation states reset

### Token System

Quest progress uses tokens in format: `{questId}-{stepName}`
- Example: "1000000-start", "1000000-givegold", "1000000-end"
- Tokens are preserved in character data
- Sequential progression enforced by the quest system

## Integration Points

### Copyover System
The quest system is registered with the central copyover manager through:
```go
{
    Name: "Quests",
    Gather: func() (interface{}, error) {
        return gatherQuestsState()
    },
    Restore: func(data interface{}) error {
        if state, ok := data.(*QuestCopyoverState); ok {
            return restoreQuestsState(state)
        }
        return fmt.Errorf("invalid quests state type")
    },
}
```

### Files
- `internal/copyover/subsystem_handlers.go` - Quest system copyover implementation
- `internal/copyover/integrations.go` - Centralized integration registry
- `quest_copyover.dat` - Temporary state file
- `events.Quest` - Quest progress events

## Error Handling

The quest copyover system is fault-tolerant:

1. **Missing Quests**: Invalid quest IDs are removed from progress
2. **Invalid Steps**: Reset to "start" step
3. **Timer Errors**: Logged but don't block copyover
4. **File Errors**: Quest state loss doesn't prevent copyover

## Testing

Unit tests in `copyover_test.go` cover:
- State serialization/deserialization
- Timer detection logic
- Quest progress validation
- File operations

Manual testing checklist:
1. Start a quest with multiple steps
2. Progress partway through the quest
3. Start a timed quest objective
4. Initiate copyover
5. Verify quest progress preserved
6. Verify timers continue counting
7. Complete the quest successfully

## Best Practices

### For Quest Designers

1. **Use Standard Steps**: Always start with "start", end with "end"
2. **Name Timers Properly**: Prefix quest timers with "quest"
3. **Avoid State Dependencies**: Don't rely on non-persisted state
4. **Test Copyover**: Verify quests work across copyover

### For Developers

1. **Quest Progress**: Store in `Character.QuestProgress`
2. **Timers**: Use `Character.Timers` with "quest" prefix
3. **Events**: Use `events.Quest` for progress updates
4. **Validation**: Implement quest validation after copyover

## Example Quest Flow

```yaml
# Quest Definition
QuestId: 1000000
Name: "The Test Quest"
Steps:
  - Id: "start"
    Description: "Talk to the quest giver"
  - Id: "collectitems"
    Description: "Collect 5 wolf pelts"
  - Id: "return"
    Description: "Return to quest giver"
  - Id: "end"
    Description: "Quest complete!"
```

During copyover:
1. Player at step "collectitems" with 3/5 pelts
2. Copyover initiated
3. After copyover:
   - Still at "collectitems" step
   - Still has 3 pelts in inventory
   - Can continue collecting remaining pelts
   - Quest completes normally

## Limitations

1. **Complex State**: Multi-phase boss fights may reset
2. **World Events**: Triggered world events don't persist
3. **Group Quests**: Party quest synchronization needs care
4. **Quest Instances**: Instanced quest areas reset

## Future Enhancements

1. Preserve quest instance states
2. Add quest checkpoint system
3. Implement quest state compression
4. Support complex multi-user quest states