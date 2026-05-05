package netproto

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const maxRoomPlayers = 4

type Room struct {
	code    string
	hostID  string
	players map[*ClientConn]struct{}
}

type RoomRegistry struct {
	mu           sync.Mutex
	rooms        map[string]*Room
	clientToRoom map[*ClientConn]string
}

func NewRoomRegistry() *RoomRegistry {
	return &RoomRegistry{
		rooms:        make(map[string]*Room),
		clientToRoom: make(map[*ClientConn]string),
	}
}

func (rr *RoomRegistry) CreateRoom(client *ClientConn) (*Room, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if _, ok := rr.clientToRoom[client]; ok {
		return nil, fmt.Errorf("already in a room")
	}

	code, err := rr.generateRoomCodeLocked()
	if err != nil {
		return nil, err
	}

	room := &Room{
		code:    code,
		hostID:  client.id,
		players: map[*ClientConn]struct{}{client: {}},
	}
	rr.rooms[code] = room
	rr.clientToRoom[client] = code

	return room, nil
}

func (rr *RoomRegistry) JoinRoom(client *ClientConn, code string) (*Room, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	if _, ok := rr.clientToRoom[client]; ok {
		return nil, fmt.Errorf("already in a room")
	}

	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, fmt.Errorf("room code is required")
	}

	room, ok := rr.rooms[code]
	if !ok {
		return nil, fmt.Errorf("room not found")
	}
	if len(room.players) >= maxRoomPlayers {
		return nil, fmt.Errorf("room is full")
	}

	room.players[client] = struct{}{}
	rr.clientToRoom[client] = code
	return room, nil
}

// LeaveRoom removes the client from their room. It returns the updated room,
// or nil if the room was deleted (empty) or the client wasn't in a room.
func (rr *RoomRegistry) LeaveRoom(client *ClientConn) *Room {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.leaveRoomLocked(client)
}

func (rr *RoomRegistry) CurrentRoom(client *ClientConn) *Room {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	code, ok := rr.clientToRoom[client]
	if !ok {
		return nil
	}
	return rr.rooms[code]
}

func (rr *RoomRegistry) RoomClients(room *Room) []*ClientConn {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	clients := make([]*ClientConn, 0, len(room.players))
	for c := range room.players {
		clients = append(clients, c)
	}
	return clients
}

func (rr *RoomRegistry) RoomStatePayload(room *Room) map[string]interface{} {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	players := make([]string, 0, len(room.players))
	for c := range room.players {
		players = append(players, c.id)
	}
	sort.Strings(players)

	return map[string]interface{}{
		"room":       room.code,
		"hostId":     room.hostID,
		"maxPlayers": maxRoomPlayers,
		"players":    players,
	}
}

func (rr *RoomRegistry) leaveRoomLocked(client *ClientConn) *Room {
	code, ok := rr.clientToRoom[client]
	if !ok {
		return nil
	}

	room, ok := rr.rooms[code]
	if !ok {
		delete(rr.clientToRoom, client)
		return nil
	}

	delete(room.players, client)
	delete(rr.clientToRoom, client)

	if len(room.players) == 0 {
		delete(rr.rooms, code)
		return nil
	}

	if room.hostID == client.id {
		hostCandidates := make([]string, 0, len(room.players))
		for c := range room.players {
			hostCandidates = append(hostCandidates, c.id)
		}
		sort.Strings(hostCandidates)
		room.hostID = hostCandidates[0]
	}

	return room
}

func (rr *RoomRegistry) generateRoomCodeLocked() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLen = 6

	for attempt := 0; attempt < 20; attempt++ {
		buf := make([]byte, codeLen)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("generate room code: %w", err)
		}

		for i := range buf {
			buf[i] = alphabet[int(buf[i])%len(alphabet)]
		}
		code := string(buf)

		if _, exists := rr.rooms[code]; !exists {
			return code, nil
		}
	}

	return "", fmt.Errorf("failed to allocate unique room code")
}

