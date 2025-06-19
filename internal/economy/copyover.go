package economy

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// EconomyCopyoverState represents economy-related state during copyover
type EconomyCopyoverState struct {
	// Shop states for mobs/players with shops
	ShopStates []ShopState `json:"shop_states"`

	// Any pending gold transfers (if we implement delayed transfers)
	PendingTransfers []PendingTransfer `json:"pending_transfers"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

// ShopState represents a shop's current inventory state
type ShopState struct {
	OwnerId   int                   `json:"owner_id"`   // Mob instance ID or negative user ID
	OwnerType string                `json:"owner_type"` // "mob" or "user"
	RoomId    int                   `json:"room_id"`
	ShopItems []characters.ShopItem `json:"shop_items"`
}

// PendingTransfer represents a gold/item transfer in progress
type PendingTransfer struct {
	FromUserId int       `json:"from_user_id"`
	ToUserId   int       `json:"to_user_id"`
	Amount     int       `json:"amount"`
	ItemId     int       `json:"item_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

const economyStateFile = "economy_copyover.dat"

// GatherEconomyState collects economy-related state for copyover
func GatherEconomyState() (*EconomyCopyoverState, error) {
	state := &EconomyCopyoverState{
		ShopStates:       []ShopState{},
		PendingTransfers: []PendingTransfer{},
		SavedAt:          time.Now(),
	}

	// Gather shop states from all rooms
	// This ensures shop inventory quantities are preserved exactly
	for _, room := range rooms.GetAllRooms() {
		// Check mobs with shops
		for _, mobId := range room.GetMobs() {
			mob := mobs.GetInstance(mobId)
			if mob != nil && mob.Character.Shop != nil && len(mob.Character.Shop) > 0 {
				shopState := ShopState{
					OwnerId:   mob.InstanceId,
					OwnerType: "mob",
					RoomId:    room.RoomId,
					ShopItems: copyShopItems(mob.Character.Shop),
				}
				state.ShopStates = append(state.ShopStates, shopState)
			}
		}

		// Check players with shops (player vendors)
		for _, userId := range room.GetPlayers() {
			if user := users.GetByUserId(userId); user != nil {
				if user.Character.Shop != nil && len(user.Character.Shop) > 0 {
					shopState := ShopState{
						OwnerId:   -userId, // Negative to distinguish from mob IDs
						OwnerType: "user",
						RoomId:    room.RoomId,
						ShopItems: copyShopItems(user.Character.Shop),
					}
					state.ShopStates = append(state.ShopStates, shopState)
				}
			}
		}
	}

	// Note: Most transactions in GoMud are immediate (gold changes hands instantly)
	// so there are typically no pending transfers to track

	// Auction state is handled by the auction module's own copyover support

	// Inbox messages with gold/items are persisted with user data

	return state, nil
}

// RestoreEconomyState restores economy state after copyover
func RestoreEconomyState(state *EconomyCopyoverState) error {
	if state == nil {
		return nil
	}

	// Restore shop states
	// This ensures shop quantities match exactly what they were before copyover
	for _, shopState := range state.ShopStates {
		room := rooms.LoadRoom(shopState.RoomId)
		if room == nil {
			continue
		}

		if shopState.OwnerType == "mob" {
			// Find the mob by instance ID
			for _, mobId := range room.GetMobs() {
				mob := mobs.GetInstance(mobId)
				if mob != nil && mob.InstanceId == shopState.OwnerId {
					// Restore shop quantities
					if mob.Character.Shop != nil {
						restoreShopQuantities(mob.Character.Shop, shopState.ShopItems)
					}
					break
				}
			}
		} else if shopState.OwnerType == "user" {
			// Find the player
			userId := -shopState.OwnerId // Convert back from negative
			if user := users.GetByUserId(userId); user != nil {
				if user.Character.Shop != nil {
					restoreShopQuantities(user.Character.Shop, shopState.ShopItems)
				}
			}
		}
	}

	// Process any pending transfers
	// Currently unused as all transfers are immediate

	return nil
}

// SaveEconomyStateForCopyover saves economy state to a file
func SaveEconomyStateForCopyover() error {
	state, err := GatherEconomyState()
	if err != nil {
		return fmt.Errorf("failed to gather economy state: %w", err)
	}

	if state == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal economy state: %w", err)
	}

	if err := os.WriteFile(economyStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write economy state file: %w", err)
	}

	return nil
}

// LoadEconomyStateFromCopyover loads and restores economy state
func LoadEconomyStateFromCopyover() error {
	data, err := os.ReadFile(economyStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No economy state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read economy state file: %w", err)
	}

	var state EconomyCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal economy state: %w", err)
	}

	if err := RestoreEconomyState(&state); err != nil {
		return fmt.Errorf("failed to restore economy state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(economyStateFile)

	return nil
}

// copyShopItems creates a deep copy of shop items
func copyShopItems(items []characters.ShopItem) []characters.ShopItem {
	copied := make([]characters.ShopItem, len(items))
	copy(copied, items)
	return copied
}

// restoreShopQuantities updates shop quantities from saved state
func restoreShopQuantities(current []characters.ShopItem, saved []characters.ShopItem) {
	// Create a map for quick lookup
	savedMap := make(map[int]int) // ItemId -> Quantity
	for _, item := range saved {
		if item.ItemId > 0 {
			savedMap[item.ItemId] = item.Quantity
		}
	}

	// Update current quantities
	for i := range current {
		if current[i].ItemId > 0 {
			if savedQty, exists := savedMap[current[i].ItemId]; exists {
				current[i].Quantity = savedQty
			}
		}
	}
}
