package netproto

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type ClientConn struct {
	id   string
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	client := &ClientConn{
		id:   time.Now().UTC().Format("20060102150405.000000000"),
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 64),
	}

	hub.Register(client)
	go client.writePump()
	go client.readPump()

	client.SendEnvelope(Envelope{
		Type:      MsgHelloOK,
		Timestamp: nowMilli(),
		Sequence:  0,
		Payload: map[string]string{
			"playerId": client.id,
		},
	})
}

func (c *ClientConn) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var env Envelope
		if err := json.Unmarshal(message, &env); err != nil {
			log.Printf("invalid websocket payload from %s: %v", c.id, err)
			continue
		}

		switch env.Type {
		case MsgPing:
			c.SendEnvelope(Envelope{
				Type:      MsgPong,
				Timestamp: nowMilli(),
				Sequence:  env.Sequence,
				Payload:   map[string]string{"ok": "1"},
			})
		case MsgCreateRoom:
			room, err := c.hub.rooms.CreateRoom(c)
			if err != nil {
				c.sendError(env.Sequence, err.Error())
				continue
			}
			c.hub.broadcastRoomState(room)
		case MsgJoinRoom:
			code, err := parseRoomCode(env.Payload)
			if err != nil {
				c.sendError(env.Sequence, err.Error())
				continue
			}
			room, err := c.hub.rooms.JoinRoom(c, code)
			if err != nil {
				c.sendError(env.Sequence, err.Error())
				continue
			}
			c.hub.broadcastRoomState(room)
		case MsgLeaveRoom:
			room := c.hub.rooms.LeaveRoom(c)
			if room != nil {
				c.hub.broadcastRoomState(room)
			}
			c.SendEnvelope(Envelope{
				Type:      MsgRoomState,
				Timestamp: nowMilli(),
				Sequence:  env.Sequence,
				Payload:   emptyRoomStatePayload(),
			})
		case MsgReady:
			ready, err := parseReadyState(env.Payload)
			if err != nil {
				c.sendError(env.Sequence, err.Error())
				continue
			}
			room, err := c.hub.rooms.SetReady(c, ready)
			if err != nil {
				c.sendError(env.Sequence, err.Error())
				continue
			}
			c.hub.broadcastRoomState(room)
		default:
			c.sendError(env.Sequence, "unsupported message type")
		}
	}
}

func emptyRoomStatePayload() map[string]interface{} {
	return map[string]interface{}{
		"room":       "",
		"hostId":     "",
		"maxPlayers": maxRoomPlayers,
		"players":    []string{},
		"ready":      map[string]bool{},
	}
}

func parseReadyState(payload interface{}) (bool, error) {
	body, ok := payload.(map[string]interface{})
	if !ok {
		return false, errReadyStateRequired
	}

	keys := []string{"ready", "isReady"}
	for _, key := range keys {
		if raw, found := body[key]; found {
			if ready, ok := raw.(bool); ok {
				return ready, nil
			}
		}
	}

	return false, errReadyStateRequired
}

var errReadyStateRequired = &protocolError{message: "ready state is required"}

func (c *ClientConn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *ClientConn) SendEnvelope(env Envelope) {
	b, err := json.Marshal(env)
	if err != nil {
		log.Printf("failed to marshal envelope: %v", err)
		return
	}

	select {
	case c.send <- b:
	default:
		log.Printf("send queue full for client %s", c.id)
		_ = c.conn.Close()
	}
}

func (c *ClientConn) sendError(seq uint64, reason string) {
	c.SendEnvelope(Envelope{
		Type:      MsgError,
		Timestamp: nowMilli(),
		Sequence:  seq,
		Payload: map[string]string{
			"reason": reason,
		},
	})
}

func parseRoomCode(payload interface{}) (string, error) {
	body, ok := payload.(map[string]interface{})
	if !ok {
		return "", errRoomCodeRequired
	}

	keys := []string{"room", "roomId", "code"}
	for _, key := range keys {
		if raw, found := body[key]; found {
			if roomCode, ok := raw.(string); ok && strings.TrimSpace(roomCode) != "" {
				return roomCode, nil
			}
		}
	}
	return "", errRoomCodeRequired
}

var errRoomCodeRequired = &protocolError{message: "room code is required"}

type protocolError struct {
	message string
}

func (e *protocolError) Error() string {
	return e.message
}

func nowMilli() int64 {
	return time.Now().UnixMilli()
}
