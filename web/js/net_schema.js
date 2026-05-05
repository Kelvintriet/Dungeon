"use strict";

window.NET_SCHEMA = Object.freeze({
  VERSION: "v1",
  TICK_RATE: 60,
  MAX_PLAYERS: 4,
  MESSAGE_TYPES: Object.freeze({
    HELLO: "hello",
    HELLO_OK: "hello_ok",
    CREATE_ROOM: "create_room",
    JOIN_ROOM: "join_room",
    LEAVE_ROOM: "leave_room",
    READY: "ready",
    INPUT: "input",
    ACK_SNAPSHOT: "ack_snapshot",
    ROOM_STATE: "room_state",
    MATCH_START: "match_start",
    SNAPSHOT: "snapshot",
    EVENT: "event",
    ERROR: "error",
    PING: "ping",
    PONG: "pong",
  }),
});
