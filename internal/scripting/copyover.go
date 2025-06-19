package scripting

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ScriptCopyoverState represents the script system state during copyover
type ScriptCopyoverState struct {
	// VM cache states - which VMs are loaded
	RoomVMs  []int    `json:"room_vms"`  // Room IDs
	MobVMs   []string `json:"mob_vms"`   // Mob instance keys
	ItemVMs  []string `json:"item_vms"`  // Item keys
	SpellVMs []string `json:"spell_vms"` // Spell keys
	BuffVMs  []int    `json:"buff_vms"`  // Buff IDs

	// Text wrapper states
	UserTextWrapState TextWrapperStyle `json:"user_text_wrap"`
	RoomTextWrapState TextWrapperStyle `json:"room_text_wrap"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

const scriptStateFile = "script_copyover.dat"

// GatherScriptState collects all script system state for copyover
func GatherScriptState() (*ScriptCopyoverState, error) {
	state := &ScriptCopyoverState{
		RoomVMs:  []int{},
		MobVMs:   []string{},
		ItemVMs:  []string{},
		SpellVMs: []string{},
		BuffVMs:  []int{},
		SavedAt:  time.Now(),
	}

	// Gather room VM cache
	for roomId := range roomVMCache {
		state.RoomVMs = append(state.RoomVMs, roomId)
	}

	// Gather mob VM cache
	for mobKey := range mobVMCache {
		state.MobVMs = append(state.MobVMs, mobKey)
	}

	// Gather item VM cache
	for itemKey := range itemVMCache {
		state.ItemVMs = append(state.ItemVMs, itemKey)
	}

	// Gather spell VM cache
	for spellKey := range spellVMCache {
		state.SpellVMs = append(state.SpellVMs, spellKey)
	}

	// Gather buff VM cache
	for buffId := range buffVMCache {
		state.BuffVMs = append(state.BuffVMs, buffId)
	}

	// Save text wrapper states
	state.UserTextWrapState = userTextWrap
	state.RoomTextWrapState = roomTextWrap

	return state, nil
}

// RestoreScriptState restores the script system state after copyover
func RestoreScriptState(state *ScriptCopyoverState) error {
	if state == nil {
		return nil
	}

	// Note: We don't restore the actual VMs, just mark which ones were loaded
	// The VMs will be recreated on-demand when scripts are executed
	// This approach is safer than trying to serialize JavaScript VM state

	// Restore text wrapper states
	userTextWrap = state.UserTextWrapState
	roomTextWrap = state.RoomTextWrapState

	// Log what VMs were active for debugging
	if len(state.RoomVMs) > 0 {
		// VMs will be recreated on demand
	}

	return nil
}

// SaveScriptStateForCopyover saves script system state to a file
func SaveScriptStateForCopyover() error {
	state, err := GatherScriptState()
	if err != nil {
		return fmt.Errorf("failed to gather script state: %w", err)
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal script state: %w", err)
	}

	if err := os.WriteFile(scriptStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write script state file: %w", err)
	}

	return nil
}

// LoadScriptStateFromCopyover loads and restores script state
func LoadScriptStateFromCopyover() error {
	data, err := os.ReadFile(scriptStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No script state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read script state file: %w", err)
	}

	var state ScriptCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal script state: %w", err)
	}

	if err := RestoreScriptState(&state); err != nil {
		return fmt.Errorf("failed to restore script state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(scriptStateFile)

	return nil
}
