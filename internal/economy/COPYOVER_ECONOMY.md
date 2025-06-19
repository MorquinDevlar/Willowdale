# Economy System Copyover Integration

This document describes how the economy systems handle copyover (hot-reload) operations in GoMud.

## Overview

The economy system in GoMud uses immediate transactions, which simplifies copyover handling. Most economic state is automatically preserved through character and world persistence. The copyover integration focuses on preserving shop inventory quantities and ensuring transaction atomicity.

## Architecture

### Automatically Preserved (via Persistence)

1. **Character Wealth**
   - `Character.Gold` - Gold on hand
   - `Character.Bank` - Banked gold
   - Automatically saved with character data

2. **Inbox Messages**
   - Pending gold/item deliveries
   - Persisted with user data

3. **Auction State**
   - Handled by the auction module's own copyover system
   - Active bids and auction timers preserved

### Manually Preserved

1. **Shop Inventories** (`ShopState`)
   - Current stock quantities for all shops
   - Both mob and player shops
   - Prevents stock inconsistencies during copyover

2. **Pending Transfers** (Future Enhancement)
   - Currently unused as all transfers are immediate
   - Structure in place for future delayed transfers

## Transaction Design

### Immediate Transactions

GoMud uses immediate transactions for all economic activities:

1. **Buying**: Gold deducted immediately, item given instantly
2. **Selling**: Item removed immediately, gold given instantly
3. **Trading**: Items/gold transferred instantly between players
4. **Banking**: Deposits/withdrawals are instant

This design eliminates most copyover complexity since there are no "in-flight" transactions.

### Transaction Safety

During copyover:
1. Commands are queued but not processed
2. No new transactions can start during state gathering
3. All transactions complete before or after copyover
4. No partial transactions possible

## Implementation Details

### State Gathering

When copyover begins:
1. The copyover manager calls the Economy system's Gather function
2. Captures all shop inventories with current quantities
3. Records shop owner (mob/player) and location
4. Returns state for central copyover system to save

### State Restoration

After copyover:
1. The copyover manager calls the Economy system's Restore function
2. Receives deserialized state from central system
3. Restores exact quantities to each shop
4. Ensures consistency with pre-copyover state

## Shop System Behavior

### What Is Preserved

- **Stock Quantities**: Exact item counts in shops
- **Shop Configuration**: All shop settings via YAML
- **Custom Prices**: Preserved in shop definitions
- **Restock Timers**: Continue via game time system

### What Resets

- **Price Negotiations**: Any in-progress haggling
- **Browse State**: What player was looking at
- **Shop UI State**: Any open shop interfaces

## Integration Points

### Copyover System
The economy system is registered with the central copyover manager through:
```go
{
    Name: "Economy",
    Gather: func() (interface{}, error) {
        return economy.GatherEconomyState()
    },
    Restore: func(data interface{}) error {
        if state, ok := data.(*economy.EconomyCopyoverState); ok {
            return economy.RestoreEconomyState(state)
        }
        return fmt.Errorf("invalid economy state type")
    },
}
```

### Files
- `internal/economy/copyover.go` - Core copyover logic
- `internal/copyover/integrations.go` - Centralized integration registry
- `economy_copyover.dat` - Temporary state file

## Testing Scenarios

### 1. Shop Stock Preservation
```
1. Check shop inventory quantities
2. Buy some items to change stock
3. Initiate copyover
4. Verify stock quantities match exactly
```

### 2. Gold Preservation
```
1. Note gold on hand and in bank
2. Perform some transactions
3. Copyover
4. Verify gold amounts unchanged
```

### 3. Auction Continuity
```
1. Start an auction (handled by auction module)
2. Place bids
3. Copyover during auction
4. Verify auction continues normally
```

### 4. Transaction Atomicity
```
1. Start a buy command
2. Initiate copyover immediately
3. Verify either:
   - Transaction completed before copyover
   - Transaction happens after copyover
   - No partial transaction occurs
```

## Best Practices

### For Builders

1. **Shop Design**: Use restock rates appropriately
2. **Pricing**: Set base prices in item definitions
3. **Stock Limits**: Use -1 for temporary items, 0 for unlimited

### For Developers

1. **Keep Transactions Atomic**: Complete immediately or not at all
2. **Avoid State**: Don't store transaction state in memory
3. **Use Events**: Trigger EquipmentChange events for gold changes

## Edge Cases

### 1. Shop Restocking During Copyover
- Restock timers based on game time
- May trigger immediately after copyover if due
- Stock quantities preserved until restock occurs

### 2. Multiple Simultaneous Buyers
- Each transaction is atomic
- No race conditions due to immediate processing
- Copyover waits for current command to complete

### 3. Offline Vendor Returns
- Player shops persist with character
- Items remain available when player returns
- No special copyover handling needed

## Comparison with Auction System

The auction module implements full copyover support because:
- Auctions have long-running state (bids over time)
- Time-sensitive operations (auction endings)
- Complex veto logic (preventing copyover near auction end)

The core economy doesn't need this complexity because:
- Transactions are instantaneous
- No long-running economic state
- Shop inventories are the only mutable state

## Future Enhancements

1. **Escrow System**: Would require copyover support for held funds
2. **Trade Windows**: Multi-step trades would need state preservation
3. **Market Orders**: Buy/sell orders would need persistence
4. **Economic Metrics**: Track transaction volumes across copyover

## Limitations

1. **No Transaction History**: Recent transactions not preserved
2. **No Price Memory**: Dynamic pricing resets to base
3. **No Haggle State**: In-progress negotiations lost

These limitations are acceptable given the immediate transaction model and the rarity of copyover events.