package events

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"
)

// EventCopyoverState represents the event queue state during copyover
type EventCopyoverState struct {
	// Pending events in priority order
	QueuedEvents []SerializedEvent `json:"queued_events"`

	// Current order counter for maintaining FIFO
	OrderCounter uint64 `json:"order_counter"`

	// Timestamp when state was gathered
	SavedAt time.Time `json:"saved_at"`
}

// SerializedEvent represents an event that can be serialized
type SerializedEvent struct {
	EventType string                 `json:"event_type"`
	Priority  int                    `json:"priority"`
	Order     uint64                 `json:"order"`
	Data      map[string]interface{} `json:"data"`
}

const eventStateFile = "event_copyover.dat"

// GatherEventState collects all pending events for copyover
func GatherEventState() (*EventCopyoverState, error) {
	qLock.Lock()
	defer qLock.Unlock()

	state := &EventCopyoverState{
		QueuedEvents: []SerializedEvent{},
		OrderCounter: orderCounter,
		SavedAt:      time.Now(),
	}

	// Create a temporary slice to hold all events
	tempEvents := make([]*prioritizedEvent, globalQueue.Len())

	// Pop all events from the queue
	i := 0
	for globalQueue.Len() > 0 {
		pe := heap.Pop(&globalQueue).(*prioritizedEvent)
		tempEvents[i] = pe
		i++
	}

	// Serialize each event
	for _, pe := range tempEvents {
		serialized, err := serializeEvent(pe)
		if err != nil {
			// Skip events that can't be serialized
			continue
		}
		state.QueuedEvents = append(state.QueuedEvents, serialized)
	}

	// Re-add all events back to the queue
	for _, pe := range tempEvents {
		heap.Push(&globalQueue, pe)
	}

	return state, nil
}

// RestoreEventState restores the event queue after copyover
func RestoreEventState(state *EventCopyoverState) error {
	if state == nil {
		return nil
	}

	qLock.Lock()
	defer qLock.Unlock()

	// Restore the order counter
	orderCounter = state.OrderCounter

	// Clear any existing events (should be empty after restart)
	globalQueue = priorityQueue{}
	heap.Init(&globalQueue)

	// Restore each event
	for _, serialized := range state.QueuedEvents {
		event, err := deserializeEvent(serialized)
		if err != nil {
			// Skip events that can't be deserialized
			continue
		}

		pe := &prioritizedEvent{
			event:    event,
			priority: serialized.Priority,
			order:    serialized.Order,
		}

		heap.Push(&globalQueue, pe)
	}

	return nil
}

// serializeEvent converts an event to a serializable format
func serializeEvent(pe *prioritizedEvent) (SerializedEvent, error) {
	serialized := SerializedEvent{
		EventType: pe.event.Type(),
		Priority:  pe.priority,
		Order:     pe.order,
		Data:      make(map[string]interface{}),
	}

	// Use reflection to extract event data
	v := reflect.ValueOf(pe.event)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip unexported fields
		if !value.CanInterface() {
			continue
		}

		// Store field data
		serialized.Data[field.Name] = value.Interface()
	}

	return serialized, nil
}

// deserializeEvent recreates an event from serialized data
func deserializeEvent(serialized SerializedEvent) (Event, error) {
	// This is where we'd need a registry of event types
	// For now, we'll handle common event types

	switch serialized.EventType {
	case "Quest":
		if userId, ok := serialized.Data["UserId"].(float64); ok {
			if questToken, ok := serialized.Data["QuestToken"].(string); ok {
				return Quest{
					UserId:     int(userId),
					QuestToken: questToken,
				}, nil
			}
		}

	case "EquipmentChange":
		evt := EquipmentChange{}
		if v, ok := serialized.Data["UserId"].(float64); ok {
			evt.UserId = int(v)
		}
		if v, ok := serialized.Data["GoldChange"].(float64); ok {
			evt.GoldChange = int(v)
		}
		if v, ok := serialized.Data["BankChange"].(float64); ok {
			evt.BankChange = int(v)
		}
		return evt, nil

	case "ItemOwnership":
		evt := ItemOwnership{}
		if v, ok := serialized.Data["UserId"].(float64); ok {
			evt.UserId = int(v)
		}
		if v, ok := serialized.Data["Gained"].(bool); ok {
			evt.Gained = v
		}
		// Note: Item reconstruction would need item system integration
		return evt, nil

		// Add more event types as needed
	}

	// For unhandled events, return a generic event
	return GenericEventImpl{
		TypeStr: serialized.EventType,
		DataMap: serialized.Data,
	}, nil
}

// GenericEventImpl is a generic event implementation for deserialization
type GenericEventImpl struct {
	TypeStr string
	DataMap map[string]interface{}
}

func (g GenericEventImpl) Type() string {
	return g.TypeStr
}

func (g GenericEventImpl) Data(name string) interface{} {
	return g.DataMap[name]
}

// SaveEventStateForCopyover saves event queue state to a file
func SaveEventStateForCopyover() error {
	state, err := GatherEventState()
	if err != nil {
		return fmt.Errorf("failed to gather event state: %w", err)
	}

	if state == nil || len(state.QueuedEvents) == 0 {
		// No events to save
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal event state: %w", err)
	}

	if err := os.WriteFile(eventStateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write event state file: %w", err)
	}

	return nil
}

// LoadEventStateFromCopyover loads and restores event queue state
func LoadEventStateFromCopyover() error {
	data, err := os.ReadFile(eventStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No event state file, that's ok
			return nil
		}
		return fmt.Errorf("failed to read event state file: %w", err)
	}

	var state EventCopyoverState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal event state: %w", err)
	}

	if err := RestoreEventState(&state); err != nil {
		return fmt.Errorf("failed to restore event state: %w", err)
	}

	// Clean up the file after successful restoration
	os.Remove(eventStateFile)

	return nil
}
