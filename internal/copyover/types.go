package copyover

import (
	"time"
)

// CopyoverStateData represents the complete state to be preserved during copyover
type CopyoverStateData struct {
	// Version information
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	StartTime time.Time `json:"start_time"` // When copyover started

	// Environment preservation
	Environment map[string]string `json:"environment"`
	ConfigPath  string            `json:"config_path"`

	// Network state
	Listeners   map[string]ListenerState `json:"listeners"`
	Connections []ConnectionState        `json:"connections"`

	// Game state
	GameState  GameSnapshot  `json:"game_state"`
	EventQueue []QueuedEvent `json:"event_queue"`
}

// ListenerState represents a listening socket's state
type ListenerState struct {
	Type    string `json:"type"`    // "telnet" or "websocket"
	Address string `json:"address"` // e.g., ":1111" or ":80"
	FD      int    `json:"fd"`      // File descriptor in ExtraFiles
}

// ConnectionState represents an active connection's state
type ConnectionState struct {
	// Connection info
	ConnectionID uint64    `json:"connection_id"` // Original connection ID
	Type         string    `json:"type"`          // "telnet" or "websocket"
	FD           int       `json:"fd"`            // File descriptor in ExtraFiles (-1 for websocket)
	RemoteAddr   string    `json:"remote_addr"`   // Client's address
	ConnectedAt  time.Time `json:"connected_at"`  // When they connected

	// User state
	UserID int `json:"user_id"` // 0 if not logged in
	RoomID int `json:"room_id"` // Current room
}

// GameSnapshot represents the world state at copyover time
type GameSnapshot struct {
	// Time and round info
	CurrentRound int       `json:"current_round"`
	GameTime     time.Time `json:"game_time"`
	RealTime     time.Time `json:"real_time"`

	// Active entities
	ActiveCombats []CombatState `json:"active_combats"`
	ActiveBuffs   []BuffState   `json:"active_buffs"`
	ActiveSpells  []SpellState  `json:"active_spells"`

	// Mob states (position changes, etc.)
	MobStates map[int]MobSnapshot `json:"mob_states"`

	// Room states (temporary changes)
	RoomStates map[int]RoomSnapshot `json:"room_states"`
}

// CombatState represents active combat
type CombatState struct {
	RoomID     int   `json:"room_id"`
	Combatants []int `json:"combatants"` // User IDs
	RoundCount int   `json:"round_count"`
	TurnOrder  []int `json:"turn_order"`
}

// BuffState represents an active buff/debuff
type BuffState struct {
	BuffID     int       `json:"buff_id"`
	TargetType string    `json:"target_type"` // "user", "mob", "room"
	TargetID   int       `json:"target_id"`
	RoundsLeft int       `json:"rounds_left"`
	AppliedAt  time.Time `json:"applied_at"`
}

// SpellState represents an in-flight spell
type SpellState struct {
	SpellID    int    `json:"spell_id"`
	CasterType string `json:"caster_type"` // "user" or "mob"
	CasterID   int    `json:"caster_id"`
	TargetType string `json:"target_type"`
	TargetID   int    `json:"target_id"`
	CastTime   int    `json:"cast_time"` // Rounds until cast
	StartRound int    `json:"start_round"`
}

// MobSnapshot represents a mob's temporary state
type MobSnapshot struct {
	InstanceID int    `json:"instance_id"`
	RoomID     int    `json:"room_id"`
	Health     int    `json:"health"`
	Mana       int    `json:"mana"`
	Position   string `json:"position"`  // "standing", "sitting", etc.
	Following  int    `json:"following"` // User ID if following
}

// RoomSnapshot represents a room's temporary state
type RoomSnapshot struct {
	RoomID    int            `json:"room_id"`
	TempExits map[string]int `json:"temp_exits"` // Temporary exits
	TempFlags []string       `json:"temp_flags"` // Temporary flags
	Mutators  []int          `json:"mutators"`   // Active mutator IDs
}

// QueuedEvent represents a pending event in the queue
type QueuedEvent struct {
	EventType   string                 `json:"event_type"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	Data        map[string]interface{} `json:"data"`
}

// CopyoverResult represents the result of a copyover operation
type CopyoverResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Statistics
	ConnectionsPreserved int           `json:"connections_preserved"`
	ConnectionsFailed    int           `json:"connections_failed"`
	Duration             time.Duration `json:"duration"`
}
