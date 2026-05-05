package netproto

// Hub tracks active websocket clients.
type Hub struct {
	register   chan *ClientConn
	unregister chan *ClientConn
	clients    map[*ClientConn]struct{}
	rooms      *RoomRegistry
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		clients:    make(map[*ClientConn]struct{}),
		rooms:      NewRoomRegistry(),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				room := h.rooms.LeaveRoom(c)
				if room != nil {
					h.broadcastRoomState(room)
				}
				delete(h.clients, c)
				close(c.send)
			}
		}
	}
}

func (h *Hub) Register(c *ClientConn) {
	h.register <- c
}

func (h *Hub) Unregister(c *ClientConn) {
	h.unregister <- c
}

func (h *Hub) broadcastRoomState(room *Room) {
	payload := h.rooms.RoomStatePayload(room)
	clients := h.rooms.RoomClients(room)
	for _, client := range clients {
		client.SendEnvelope(Envelope{
			Type:      MsgRoomState,
			Timestamp: nowMilli(),
			Sequence:  0,
			RoomID:    room.code,
			Payload:   payload,
		})
	}
}
