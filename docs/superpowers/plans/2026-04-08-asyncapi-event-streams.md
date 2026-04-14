# AsyncAPI Event Streams Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a machine-readable AsyncAPI document for the current Go backend websocket and bridge/IM event flows.

**Architecture:** Build one AsyncAPI 3.0.0 document grounded in `src-go/internal/ws`, `internal/model/im.go`, and the live route wiring. Reuse stable OpenAPI DTOs via external refs and keep plugin/runtime-defined payloads intentionally loose where the backend contract is dynamic.

**Tech Stack:** AsyncAPI 3.0.0, YAML, current Go backend DTOs and websocket handlers.

---

## Chunk 1: Map realtime truth

**Files:**
- Read: `src-go/internal/server/routes.go`
- Read: `src-go/internal/ws/events.go`
- Read: `src-go/internal/ws/handler.go`
- Read: `src-go/internal/ws/bridge_handler.go`
- Read: `src-go/internal/ws/im_control_handler.go`
- Read: `src-go/internal/model/im.go`

- [ ] Confirm the three websocket handshake surfaces and their directionality.
- [ ] Extract the frontend event enum and bridge event enum.
- [ ] Identify the stable IM control delivery and ack schemas.

## Chunk 2: Author AsyncAPI

**Files:**
- Create: `docs/api/asyncapi.yaml`
- Modify: `docs/api/README.md`

- [ ] Define websocket servers and channels for `/ws`, `/ws/bridge`, and `/ws/im-bridge`.
- [ ] Add operations for frontend event send, bridge event receive, IM control delivery send, and IM delivery ack receive.
- [ ] Reuse stable DTOs from `docs/api/openapi.json` via external refs where appropriate.
- [ ] Add examples for frontend events, bridge runtime events, and IM control deliveries.

## Chunk 3: Verify

**Files:**
- Verify: `docs/api/asyncapi.yaml`

- [ ] Parse AsyncAPI YAML successfully.
- [ ] Re-check that websocket channel addresses match the live routes.
- [ ] Re-check that frontend and bridge event enums match `src-go/internal/ws/events.go`.
