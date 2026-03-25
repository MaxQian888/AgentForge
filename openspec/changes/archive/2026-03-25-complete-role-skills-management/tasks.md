## 1. Extend Role Skills Contracts

- [x] 1.1 Add a structured role skill reference model to `src-go/internal/model/role.go` and extend `src-go/internal/role/*` parsing, normalization, YAML round-trip, and inheritance merge logic for `capabilities.skills`.
- [x] 1.2 Update role handler/store tests and canonical sample role manifests under `roles/` so role list/get/create/update flows preserve ordered skill references and reject blank or duplicate skill paths.

## 2. Build The Role Skills Workspace

- [x] 2.1 Extend `lib/stores/role-store.ts` and `lib/roles/role-management.ts` with typed skill draft helpers, serialization, summary helpers, and blocking validation for invalid skill rows.
- [x] 2.2 Update `components/roles/role-workspace.tsx` and any shared role form seams so operators can add, remove, edit, and toggle auto-load for role skills while preserving template and inheritance carry-over.
- [x] 2.3 Update `components/roles/role-card.tsx` and the role draft summary rail to surface configured skill counts, auto-load versus on-demand split, and representative skill path cues.

## 3. Verify Focused Role Skills Coverage

- [x] 3.1 Add or refresh focused backend and frontend tests covering skill round-trip, inheritance override, UI prefill, and invalid draft blocking scenarios.
- [x] 3.2 Run scoped role-management verification and confirm the new change is implementation-ready without expanding into runtime skill injection or marketplace behavior.
