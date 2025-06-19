package rooms

// GetAllRooms returns all loaded rooms (used in copyover.go)
func GetAllRooms() []*Room {
	rooms := make([]*Room, 0, len(roomManager.rooms))
	for _, room := range roomManager.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}
