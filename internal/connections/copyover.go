package connections

import (
	"net"
	"os"
)

// GetRawConnection returns the underlying net.Conn for copyover purposes
// This is needed to preserve connections across server restarts
func (cd *ConnectionDetails) GetRawConnection() net.Conn {
	if cd.wsConn != nil {
		return nil // WebSocket connections can't be preserved
	}
	return cd.conn
}

// GetFileDescriptor gets the file descriptor from a connection for copyover
func GetFileDescriptor(connId ConnectionId) (*os.File, error) {
	cd := Get(connId)
	if cd == nil {
		return nil, nil
	}
	
	conn := cd.GetRawConnection()
	if conn == nil {
		return nil, nil
	}
	
	// Type assert to get file
	switch c := conn.(type) {
	case *net.TCPConn:
		return c.File()
	case *net.UnixConn:
		return c.File()
	default:
		return nil, nil
	}
}

