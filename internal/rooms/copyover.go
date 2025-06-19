package rooms

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// RoomCopyoverState represents all room runtime state during copyover
type RoomCopyoverState struct {
	// Runtime state for each room
	RoomStates map[int]RoomRuntimeState `json:"room_states"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

// RoomRuntimeState represents runtime state for a single room
type RoomRuntimeState struct {
	RoomId int `json:"room_id"`

	// Temporary exits that expire
	ExitsTemp map[string]exit.TemporaryRoomExit `json:"exits_temp,omitempty"`

	// Temporary data store
	TempDataStore map[string]interface{} `json:"temp_data_store,omitempty"`

	// Active mutators (we store their IDs to recreate)
	ActiveMutatorIds []string `json:"active_mutator_ids,omitempty"`

	// Recent visitors tracking
	Visitors    map[VisitorType]map[int]uint64 `json:"visitors,omitempty"`
	LastVisited uint64                         `json:"last_visited,omitempty"`

	// Corpses in the room
	Corpses []Corpse `json:"corpses,omitempty"`

	// Signs with expiration
	Signs []Sign `json:"signs,omitempty"`

	// Last idle message index
	LastIdleMessage uint8 `json:"last_idle_message,omitempty"`

	// Exit lock states (if different from defaults)
	ExitLocks map[string]bool `json:"exit_locks,omitempty"`
}

const roomStateFile = "room_copyover.dat"

// GatherRoomState collects all room runtime state for copyover
func GatherRoomState() (*RoomCopyoverState, error) {
	state := &RoomCopyoverState{
		RoomStates: make(map[int]RoomRuntimeState),
		SavedAt:    time.Now(),
	}

	// Iterate through all loaded rooms
	for _, room := range GetAllRooms() {
		roomState := RoomRuntimeState{
			RoomId: room.RoomId,
		}

		// Save temporary exits
		if len(room.ExitsTemp) > 0 {
			roomState.ExitsTemp = make(map[string]exit.TemporaryRoomExit)
			for dir, tempExit := range room.ExitsTemp {
				roomState.ExitsTemp[dir] = tempExit
			}
		}

		// Save temporary data store
		if len(room.tempDataStore) > 0 {
			roomState.TempDataStore = make(map[string]interface{})
			for key, value := range room.tempDataStore {
				roomState.TempDataStore[key] = value
			}
		}

		// Save active mutator IDs
		room.ActiveMutators(func(mut mutators.Mutator) bool {
			if spec := mut.GetSpec(); spec != nil {
				roomState.ActiveMutatorIds = append(roomState.ActiveMutatorIds, spec.MutatorId)
			}
			return false
		})

		// Save visitors
		if len(room.visitors) > 0 {
			roomState.Visitors = make(map[VisitorType]map[int]uint64)
			for vType, visitors := range room.visitors {
				roomState.Visitors[vType] = make(map[int]uint64)
				for id, lastSeen := range visitors {
					roomState.Visitors[vType][id] = lastSeen
				}
			}
			roomState.LastVisited = room.lastVisited
		}

		// Save corpses
		if len(room.Corpses) > 0 {
			roomState.Corpses = make([]Corpse, len(room.Corpses))
			copy(roomState.Corpses, room.Corpses)
		}

		// Save signs
		if len(room.Signs) > 0 {
			roomState.Signs = make([]Sign, len(room.Signs))
			copy(roomState.Signs, room.Signs)
		}

		// Save last idle message
		roomState.LastIdleMessage = room.LastIdleMessage

		// Save exit lock states that differ from defaults
		roomState.ExitLocks = make(map[string]bool)
		for exitName, exitInfo := range room.Exits {
			if exitInfo.Lock.IsLocked() {
				// Check if this differs from the default
				if defaultRoom := LoadRoom(room.RoomId); defaultRoom != nil {
					if defaultExit, ok := defaultRoom.Exits[exitName]; ok {
						if exitInfo.Lock.IsLocked() != defaultExit.Lock.IsLocked() {
							roomState.ExitLocks[exitName] = exitInfo.Lock.IsLocked()
						}
					}
				}
			}
		}

		// Only save room state if it has runtime data
		if hasRuntimeState(roomState) {
			state.RoomStates[room.RoomId] = roomState
		}
	}

	return state, nil
}

// RestoreRoomState restores all room runtime state after copyover
func RestoreRoomState(state *RoomCopyoverState) error {
	if state == nil {
		return nil
	}

	// Restore each room's state
	for roomId, roomState := range state.RoomStates {
		room := LoadRoom(roomId)
		if room == nil {
			continue // Room no longer exists
		}

		// Restore temporary exits
		if len(roomState.ExitsTemp) > 0 {
			room.ExitsTemp = make(map[string]exit.TemporaryRoomExit)
			for dir, tempExit := range roomState.ExitsTemp {
				room.ExitsTemp[dir] = tempExit
			}
		}

		// Restore temporary data store
		if len(roomState.TempDataStore) > 0 {
			room.tempDataStore = make(map[string]any)
			for key, value := range roomState.TempDataStore {
				room.tempDataStore[key] = value
			}
		}

		// Restore active mutators
		if len(roomState.ActiveMutatorIds) > 0 {
			for _, mutatorId := range roomState.ActiveMutatorIds {
				if spec := mutators.GetMutatorSpec(mutatorId); spec != nil {
					// Create a new mutator instance from the spec
					mut := mutators.Mutator{
						MutatorId:    mutatorId,
						SpawnedRound: util.GetRoundCount(), // Reset spawn time to current
					}
					room.Mutators = append(room.Mutators, mut)
				}
			}
		}

		// Restore visitors
		if len(roomState.Visitors) > 0 {
			room.visitors = make(map[VisitorType]map[int]uint64)
			for vType, visitors := range roomState.Visitors {
				room.visitors[vType] = make(map[int]uint64)
				for id, lastSeen := range visitors {
					room.visitors[vType][id] = lastSeen
				}
			}
			room.lastVisited = roomState.LastVisited
		}

		// Restore corpses
		if len(roomState.Corpses) > 0 {
			room.Corpses = make([]Corpse, len(roomState.Corpses))
			copy(room.Corpses, roomState.Corpses)
		}

		// Restore signs
		if len(roomState.Signs) > 0 {
			room.Signs = make([]Sign, len(roomState.Signs))
			copy(room.Signs, roomState.Signs)
		}

		// Restore last idle message
		room.LastIdleMessage = roomState.LastIdleMessage

		// Restore exit lock states
		for exitName, isLocked := range roomState.ExitLocks {
			if exit, ok := room.Exits[exitName]; ok {
				if isLocked {
					exit.Lock.SetLocked()
				} else {
					exit.Lock.SetUnlocked()
				}
				room.Exits[exitName] = exit
			}
		}
	}

	return nil
}

// SaveRoomStateForCopyover saves room state to a file
func SaveRoomStateForCopyover() error {
	state, err := GatherRoomState()
	if err != nil {
		return fmt.Errorf("failed to gather room state: %w", err)
	}

	if state == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal room state: %w", err)
	}

	if err := os.WriteFile(roomStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write room state file: %w", err)
	}

	return nil
}

// LoadRoomStateFromCopyover loads and restores room state
func LoadRoomStateFromCopyover() error {
	data, err := os.ReadFile(roomStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No room state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read room state file: %w", err)
	}

	var state RoomCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal room state: %w", err)
	}

	if err := RestoreRoomState(&state); err != nil {
		return fmt.Errorf("failed to restore room state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(roomStateFile)

	return nil
}

// hasRuntimeState checks if a room state has any runtime data worth saving
func hasRuntimeState(state RoomRuntimeState) bool {
	return len(state.ExitsTemp) > 0 ||
		len(state.TempDataStore) > 0 ||
		len(state.ActiveMutatorIds) > 0 ||
		len(state.Visitors) > 0 ||
		len(state.Corpses) > 0 ||
		len(state.Signs) > 0 ||
		len(state.ExitLocks) > 0 ||
		state.LastIdleMessage > 0
}

// ValidateRoomState ensures room state is consistent after copyover
func ValidateRoomState() error {
	// Prune expired visitors
	for _, room := range GetAllRooms() {
		room.PruneVisitors()

		// Remove expired temporary exits
		// TODO: Parse tempExit.Expires to check if expired

		// Remove expired corpses
		// TODO: Implement corpse validation based on decay time
		// This would require parsing the CorpseDecayTime string and getting current round
	}

	return nil
}
