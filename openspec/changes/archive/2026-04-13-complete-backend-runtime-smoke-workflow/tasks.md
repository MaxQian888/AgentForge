## 1. Backend verify command surface

- [x] 1.1 Add the supported root-level `dev:backend:verify` entrypoint in `package.json` and implement the verify runner by reusing the existing `scripts/dev-all.js` backend profile instead of creating a parallel startup stack.
- [x] 1.2 Plumb verify-time runtime metadata, reuse semantics, and default keep-running behavior so `dev:backend:status`, `dev:backend:logs`, and `dev:backend:stop` continue to describe the same managed backend stack after verification.

## 2. Cross-platform IM stub smoke proof

- [x] 2.1 Add a cross-platform smoke helper that reuses the existing `src-im-bridge` stub fixtures and test endpoints to inject one repo-supported Bridge-backed IM command and capture replies without external provider credentials.
- [x] 2.2 Wire the backend verify runner to execute the required smoke stages: Go health, TS Bridge health, IM Bridge health, stub command injection, and reply capture, with the managed default IM platform/ports and optional overrides kept consistent.

## 3. Stage diagnostics and documentation truth

- [x] 3.1 Implement stage-based smoke output so failures explicitly identify the broken hop (`startup`, `go-health`, `bridge-health`, `im-health`, `stub-command`, `reply-capture`) and print the relevant endpoint, state file, or repo-local log path.
- [x] 3.2 Update `README.md`, `README_zh.md`, and `TESTING.md` so backend-only startup, verify, diagnostics, and cleanup instructions all describe the same supported workflow and zero-credential stub assumptions.

## 4. Focused verification coverage

- [x] 4.1 Add or update focused tests for the backend verify runner and cross-platform stub smoke helper, covering clean-start, healthy-reuse, stage failure reporting, and keep-running behavior.
- [x] 4.2 Run the targeted verification needed for this change and record the exact backend smoke / script test commands that prove the new workflow behaves as specified.
