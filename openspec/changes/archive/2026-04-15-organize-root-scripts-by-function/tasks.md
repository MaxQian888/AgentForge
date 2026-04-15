## 1. Script Domain Layout

- [x] 1.1 Inventory the current root `scripts/` files, helpers, fixtures, and wrappers and assign each one to a canonical function-based script family.
- [x] 1.2 Move each root script family into its domain-owned directory and update intra-script imports or path helpers so the reorganized files still resolve correctly.

## 2. Caller Migration

- [x] 2.1 Update repo-supported execution entrypoints to the canonical script paths, including root `package.json`, plugin-local `package.json` files, shell wrappers, and GitHub workflow steps.
- [x] 2.2 Update repository docs, tests, fixtures, and embedded path strings so every repo-owned caller references the new script locations instead of removed flat paths.

## 3. Verification And Cleanup

- [x] 3.1 Refresh or add focused validation for moved script families so path-sensitive workflows and tests still execute against the reorganized layout.
- [x] 3.2 Run a repo-wide stale-path audit plus focused verification commands, and resolve any remaining references to removed root script paths before declaring the migration complete.
