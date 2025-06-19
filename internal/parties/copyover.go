package parties

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// PartyCopyoverState represents all party state during copyover
type PartyCopyoverState struct {
	// All active parties
	Parties []PartyState `json:"parties"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

// PartyState represents a single party's state
type PartyState struct {
	LeaderUserId  int            `json:"leader_user_id"`
	UserIds       []int          `json:"user_ids"`
	InviteUserIds []int          `json:"invite_user_ids"`
	AutoAttackers []int          `json:"auto_attackers"`
	Position      map[int]string `json:"position"`
}

const partyStateFile = "party_copyover.dat"

// GatherPartyState collects all party state for copyover
func GatherPartyState() (*PartyCopyoverState, error) {
	state := &PartyCopyoverState{
		Parties: []PartyState{},
		SavedAt: time.Now(),
	}

	// Capture all parties from the global party map
	for leaderId, party := range partyMap {
		partyState := PartyState{
			LeaderUserId:  leaderId,
			UserIds:       make([]int, len(party.UserIds)),
			InviteUserIds: make([]int, len(party.InviteUserIds)),
			AutoAttackers: make([]int, len(party.AutoAttackers)),
			Position:      make(map[int]string),
		}

		// Deep copy slices to avoid references
		copy(partyState.UserIds, party.UserIds)
		copy(partyState.InviteUserIds, party.InviteUserIds)
		copy(partyState.AutoAttackers, party.AutoAttackers)

		// Copy position map
		for userId, pos := range party.Position {
			partyState.Position[userId] = pos
		}

		state.Parties = append(state.Parties, partyState)
	}

	return state, nil
}

// RestorePartyState restores all party state after copyover
func RestorePartyState(state *PartyCopyoverState) error {
	if state == nil {
		return nil
	}

	// Clear any existing parties (should be empty after restart)
	partyMap = make(map[int]*Party)

	// Restore each party
	for _, partyState := range state.Parties {
		party := &Party{
			LeaderUserId:  partyState.LeaderUserId,
			UserIds:       make([]int, len(partyState.UserIds)),
			InviteUserIds: make([]int, len(partyState.InviteUserIds)),
			AutoAttackers: make([]int, len(partyState.AutoAttackers)),
			Position:      make(map[int]string),
		}

		// Copy data
		copy(party.UserIds, partyState.UserIds)
		copy(party.InviteUserIds, partyState.InviteUserIds)
		copy(party.AutoAttackers, partyState.AutoAttackers)

		// Copy position map
		for userId, pos := range partyState.Position {
			party.Position[userId] = pos
		}

		// Add to global map
		partyMap[partyState.LeaderUserId] = party

		// Also add entries for all members pointing to this party
		for _, userId := range party.UserIds {
			if userId != party.LeaderUserId {
				partyMap[userId] = party
			}
		}
	}

	return nil
}

// SavePartyStateForCopyover saves party state to a file
func SavePartyStateForCopyover() error {
	state, err := GatherPartyState()
	if err != nil {
		return fmt.Errorf("failed to gather party state: %w", err)
	}

	if state == nil {
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal party state: %w", err)
	}

	if err := os.WriteFile(partyStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write party state file: %w", err)
	}

	return nil
}

// LoadPartyStateFromCopyover loads and restores party state
func LoadPartyStateFromCopyover() error {
	data, err := os.ReadFile(partyStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No party state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read party state file: %w", err)
	}

	var state PartyCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal party state: %w", err)
	}

	if err := RestorePartyState(&state); err != nil {
		return fmt.Errorf("failed to restore party state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(partyStateFile)

	return nil
}

// ValidatePartyState ensures party state is consistent after copyover
func ValidatePartyState() error {
	// Validate all parties have valid members
	for leaderId, party := range partyMap {
		// Ensure leader is in the party members
		leaderFound := false
		for _, userId := range party.UserIds {
			if userId == leaderId {
				leaderFound = true
				break
			}
		}

		if !leaderFound && party.LeaderUserId == leaderId {
			// Add leader to members if missing
			party.UserIds = append([]int{leaderId}, party.UserIds...)
		}

		// Remove any offline members from invites
		validInvites := []int{}
		for _, inviteId := range party.InviteUserIds {
			// This would check if user is online
			// For now, keep all invites
			validInvites = append(validInvites, inviteId)
		}
		party.InviteUserIds = validInvites
	}

	return nil
}
