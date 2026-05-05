package netproto

// Envelope is the common wire format for all websocket messages.
type Envelope struct {
	Type      string      `json:"t"`
	Timestamp int64       `json:"ts"`
	Sequence  uint64      `json:"seq"`
	RoomID    string      `json:"room,omitempty"`
	Payload   interface{} `json:"p"`
}

const (
	MsgHello        = "hello"
	MsgHelloOK      = "hello_ok"
	MsgCreateRoom   = "create_room"
	MsgJoinRoom     = "join_room"
	MsgLeaveRoom    = "leave_room"
	MsgReady        = "ready"
	MsgInput        = "input"
	MsgAckSnapshot  = "ack_snapshot"
	MsgRoomState    = "room_state"
	MsgMatchStart   = "match_start"
	MsgSnapshot     = "snapshot"
	MsgEvent        = "event"
	MsgError        = "error"
	MsgPing         = "ping"
	MsgPong         = "pong"
)
