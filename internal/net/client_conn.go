package netproto

import (
	"encoding/json"
	"log"
	"net/http"
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
		Timestamp: time.Now().UnixMilli(),
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

		if env.Type == MsgPing {
			c.SendEnvelope(Envelope{
				Type:      MsgPong,
				Timestamp: time.Now().UnixMilli(),
				Sequence:  env.Sequence,
				Payload:   map[string]string{"ok": "1"},
			})
		}
	}
}

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
		c.hub.Unregister(c)
	}
}

