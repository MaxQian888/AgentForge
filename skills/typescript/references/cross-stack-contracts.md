# Cross-Stack Contract Touchpoints

Use this reference when a field or payload crosses more than one implementation surface.

## Frontend

- `lib/stores/` for API-backed state and normalization
- `lib/roles/` for role-authoring serialization and resolution helpers
- `components/roles/` and related UI consumers for rendered contract usage

## TS Bridge

- `src-bridge/src/types.ts` for runtime-facing TypeScript contracts
- `src-bridge/src/schemas.ts` for Zod validation
- `src-bridge/src/role/injector.ts` for prompt projection

## Go

- `src-go/internal/model/role.go` for API-facing role models
- `src-go/internal/role/` for skill parsing, catalogs, and execution profile projection
- `src-go/internal/bridge/client.go` for bridge request payload shapes

## Verification

- Re-run the package-local typecheck for every touched surface.
- Re-run the narrowest schema or serialization tests that prove the contract change is real.
