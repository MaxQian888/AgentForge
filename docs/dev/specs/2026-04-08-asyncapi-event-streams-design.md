# AsyncAPI Event Streams Design

## Goal

Document the current Go backend realtime and event-driven contracts in a machine-readable AsyncAPI document, focused on the websocket and IM/bridge message flows that OpenAPI does not fully model.

## Scope

- `/ws` frontend realtime push channel
- `/ws/bridge` TS bridge -> Go runtime event ingress
- `/ws/im-bridge` Go -> IM bridge delivery stream plus websocket ack flow
- Companion HTTP control-plane paths are referenced in descriptions when they are required to make sense of the websocket flow

## Current Repo Truth

- All websocket handshake endpoints are registered in `src-go/internal/server/routes.go`
- Frontend websocket event envelopes are defined in `src-go/internal/ws/events.go` and emitted via `src-go/internal/ws/hub.go`
- Bridge runtime ingress messages are defined by `BridgeAgentEvent` in `src-go/internal/ws/events.go` and consumed by `src-go/internal/ws/bridge_handler.go`
- IM bridge control deliveries and websocket acks are defined in `src-go/internal/model/im.go` and wired by `src-go/internal/ws/im_control_handler.go`

## Design

1. Use AsyncAPI 3.0.0 with a websocket-first model.
2. Keep one canonical file at `docs/api/asyncapi.yaml`.
3. Reuse stable REST DTOs from `docs/api/openapi.json` via external JSON references where that improves consistency.
4. Keep dynamic plugin/runtime payloads schema-loose instead of inventing false fixed contracts.
5. Update `docs/api/README.md` to point to the AsyncAPI document and to clarify the division of responsibility between OpenAPI and AsyncAPI.

## Validation

- YAML must parse successfully.
- The document must expose channels for `/ws`, `/ws/bridge`, and `/ws/im-bridge`.
- Event enums must match the current constants in `src-go/internal/ws/events.go`.
