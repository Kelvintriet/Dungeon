# Co-op Backend Plan (Go + WebSocket, Max 4 Players)

## 1) Goal and Constraints

Build a simple but solid co-op backend so up to **4 players** can join the same run and play together smoothly (movement, shooting, enemies, damage, loot, room flow).

- Transport: **WebSocket** (no WebRTC for now)
- Server model: **authoritative game server**
- Target feel: **60 FPS client render**, **60 Hz simulation tick** on server
- Match size: **1-4 players per room/session**
- Keep game logic deterministic enough for consistent sync

## 2) High-Level Architecture

1. **Go server** hosts lobby + game rooms.
2. Each room runs one fixed-step simulation loop (`16.666ms`).
3. Clients send input commands (move, aim, fire, interact), not final positions.
4. Server simulates world state and broadcasts periodic snapshots.
5. Clients render at 60 FPS with interpolation/prediction to hide latency.

## 3) Suggested Repository Structure

```text
game-backend/
  cmd/
    server/
      main.go
  internal/
    net/
      ws_hub.go             # websocket upgrade, connection lifecycle
      client_conn.go        # per-client send queue, ping/pong, backpressure
      protocol.go           # message envelope + type registry
      codec_json.go         # start with JSON; optional binary later
    lobby/
      lobby_service.go      # create/join/leave room, invite code
      room_registry.go      # room lookup and lifecycle
    room/
      room.go               # room state + player slots (max 4)
      room_loop.go          # fixed tick loop, time budget metrics
      room_events.go        # in-room event queue
    simulation/
      world_state.go        # players, enemies, bullets, pickups, dungeon seed
      systems_movement.go
      systems_combat.go
      systems_enemy_ai.go
      systems_collision.go
      systems_pickups.go
      systems_rooms.go
      rng.go                # seeded RNG for reproducibility
    snapshot/
      snapshot_builder.go   # delta/full snapshot assembly
      quantize.go           # position/velocity packing strategy
    game/
      weapons.go            # mirrored weapon data used server-side
      enemies.go
      balance.go
    storage/
      memory_store.go       # in-memory for first phase
    observability/
      metrics.go            # tick ms, room count, msg/sec, drop count
      logger.go
    config/
      config.go             # env config (port, tick rate, limits)
  web/
    game.html               # current client; networking hooks added here
    js/
      net_client.js         # ws client, reconnect, protocol dispatch
      net_prediction.js     # client prediction + reconciliation
      net_interpolation.js  # remote players/enemies smoothing
      net_schema.js         # client protocol constants
  docs/
    protocol.md             # message contract and examples
    runbook.md              # local run, tuning, troubleshooting
```

## 4) Networking Protocol (MVP)

Use one envelope for every message:

```json
{
  "t": "message_type",
  "ts": 1710000000000,
  "seq": 120,
  "room": "ABCD12",
  "p": {}
}
```

### Client -> Server

- `hello` (client version, player name)
- `create_room` / `join_room` / `leave_room`
- `ready`
- `input` (frame number + movement axes + aim + buttons)
- `ack_snapshot` (last snapshot tick received)
- `ping`

### Server -> Client

- `hello_ok` (playerId, server time)
- `room_state` (players in room, host, ready states)
- `match_start` (seed, initial room/world state)
- `snapshot` (tick, state delta/full)
- `event` (weapon fired, hit, pickup, room cleared, revive, etc.)
- `error` (reason/code)
- `pong`

## 5) Authority and Simulation Rules

1. **Server is source of truth** for:
   - player positions/HP/armor/energy
   - bullets/projectiles
   - enemy AI and damage
   - drops/coins/interaction results
   - room lock/unlock progression
2. Client can do local feel improvements (prediction/effects), but reconciles to server.
3. Any invalid or late input is clamped/rejected (anti-cheat baseline).

## 6) Tick, Snapshot, and Smoothness Targets

- **Server tick:** 60 Hz fixed-step (`dt = 1/60`)
- **Input rate:** client sends 30-60 Hz (compressed command stream)
- **Snapshot broadcast:** start at 20-30 Hz, tune upward if bandwidth allows
- **Client render:** keep `requestAnimationFrame` at 60 FPS
- **Smoothing:** interpolate remote entities by ~100ms buffer

Performance budgets (per room, rough target):

- Tick compute: <= 4ms average, <= 8ms p95
- Outbound snapshot payload: <= 20-40 KB/s per client (MVP JSON)
- No unbounded queues; drop or coalesce stale snapshots

## 7) Data Model (Core)

- `PlayerState`: id, name, x/z, dir, hp, armor, energy, weapon, alive, inputSeq
- `EnemyState`: id, type, x/z, hp, status effects, ai state
- `BulletState`: id, ownerId, x/z, vx/vz, ttl, damage, flags
- `RoomState`: id, seed, floor/subLevel, entities, status, maxPlayers=4
- `WorldState`: timestamp, tick, collections for players/enemies/bullets/items

## 8) Client Integration Plan (`game.html`)

1. Add a **network mode toggle** (solo/local vs online/co-op).
2. Extract local gameplay calls into system entry points:
   - local simulation path (existing behavior)
   - networked path (server snapshots + command send)
3. Replace direct local authority actions in co-op mode:
   - movement: send input commands
   - fire/interact: send command, show immediate local FX
4. Add remote-player render objects (up to 3 teammates).
5. Reconcile local player state against authoritative snapshots.
6. Keep HUD local but source gameplay stats from synced state in co-op mode.

## 9) Step-by-Step Delivery Plan

## Phase 0 - Baseline and refactor prep

1. Split `main.go` into `cmd/server/main.go`.
2. Move static file serving + health endpoint into server bootstrap.
3. Extract current client update flow in `game.html` into clearer modules/hooks.
4. Define protocol constants in one place on both server and client.

## Phase 1 - Room/lobby foundation

1. Implement WebSocket hub and connection lifecycle.
2. Add lobby flows: create room, join by code, leave room.
3. Enforce hard room cap of 4 players.
4. Broadcast room state changes to connected players.

## Phase 2 - Authoritative movement sync

1. Implement room fixed-tick loop (60 Hz).
2. Accept input packets and queue by tick/seq.
3. Simulate only movement + collision on server.
4. Broadcast snapshots and render remote teammates.
5. Add client interpolation and local reconciliation.

## Phase 3 - Combat sync (core gameplay)

1. Move bullet spawn/update/hit logic to server simulation.
2. Move enemy AI + damage evaluation to server.
3. Broadcast gameplay events (fire/hit/kill/drop).
4. Ensure local FX remain responsive while reconciling outcomes.

## Phase 4 - Room progression + interactions

1. Sync room lock/unlock and enemy clear conditions.
2. Move interactions (shop/chest/portal/pickups) to server authority.
3. Sync inventory/weapon changes and resource economy.

## Phase 5 - Stability + performance tuning

1. Add heartbeat + timeout + graceful disconnect handling.
2. Add reconnect window (short session resume) where possible.
3. Add metrics for tick time, queue size, payload size, dropped messages.
4. Tune snapshot frequency and payload compaction.

## Phase 6 - Hardening and release

1. Add protocol versioning and compatibility checks.
2. Add server config via env vars (port, tickrate, max room count).
3. Add deployment profile (single binary + reverse proxy websocket support).
4. Add runbook and protocol docs.

## 10) Testing Strategy

- **Unit tests (Go):**
  - movement/collision correctness
  - bullet hit + damage calculations
  - room capacity and lobby transitions
  - deterministic behavior with fixed seed and fixed inputs
- **Integration tests:**
  - 4 simulated clients in one room
  - join/leave under load
  - reconnect behavior
- **Soak tests:**
  - long-running room simulation for memory/tick drift

## 11) Risks and Mitigations

- **Desync risk:** use server authority + input seq + reconciliation.
- **Lag spikes:** interpolation buffer + message coalescing.
- **CPU growth per room:** enforce entity limits and optimize broad-phase collision.
- **Feature coupling with giant `game.html`:** progressively extract networking and gameplay adapters into small JS modules.

## 12) Milestone Definition of Done

**M1:** 4 players can create/join room and see each other move smoothly.  
**M2:** Shooting, enemy damage, and deaths are server-authoritative and synced.  
**M3:** Full room progression (clear room, loot, portal) works in co-op with stable performance.

