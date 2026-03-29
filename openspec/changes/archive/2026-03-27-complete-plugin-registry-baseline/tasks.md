## 1. Establish the remote registry control-plane seam

- [x] 1.1 Add a concrete remote registry adapter and configuration path that can satisfy `RemoteRegistryClient` for browse and download operations.
- [x] 1.2 Update the Go plugin service and handlers so remote registry browse and install routes return stable operator-facing availability and failure states instead of raw configuration errors.
- [x] 1.3 Normalize remote registry browse results into the metadata shape required by the plugin operator console, including installability and source provenance.

## 2. Route remote installs through the existing trust pipeline

- [x] 2.1 Extend remote install flow so downloaded remote artifacts enter the existing install and verification pipeline rather than bypassing digest, signature, approval, or lifecycle gates.
- [x] 2.2 Persist remote registry source provenance, selected version, and resolved artifact metadata in the installed plugin record and related DTOs.
- [x] 2.3 Add stable failure classification for remote reachability, download, manifest validity, and trust or approval blocking.

## 3. Extend the plugin operator console for remote registry entries

- [x] 3.1 Expand the plugin store with remote registry browse, install, availability, and failure state handling that maps directly to the Go control-plane responses.
- [x] 3.2 Update `app/(dashboard)/plugins/page.tsx` and related plugin components to render a distinct remote registry marketplace section with truthful install CTA, blocked reasons, and source health state.
- [x] 3.3 Show remote-source provenance and install outcomes in plugin detail surfaces without regressing existing installed, built-in, and local catalog flows.

## 4. Verify the focused Plugin Registry baseline

- [x] 4.1 Add or update Go service and handler tests for configured vs unconfigured remote registry browse, remote install success, and classified failure paths.
- [x] 4.2 Add or update frontend tests for remote marketplace rendering, availability messaging, remote install actions, and source provenance display.
- [x] 4.3 Run focused verification for the touched Go and frontend plugin-control-plane paths, then reconcile wording if implementation truth requires it.