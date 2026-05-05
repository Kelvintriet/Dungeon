package netproto

// Hub tracks active websocket clients.
type Hub struct {
	register   chan *ClientConn
	unregister chan *ClientConn
	clients    map[*ClientConn]struct{}
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *ClientConn),
		unregister: make(chan *ClientConn),
		clients:    make(map[*ClientConn]struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
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

