## ADDED Requirements

### Requirement: Official built-in catalog entries expose readiness separately from installability
The system SHALL return evaluated readiness state and setup guidance for official built-in catalog and discovery entries separately from the source installability contract. A built-in entry MAY remain explicitly installable while still reporting blocked activation readiness, but the response MUST NOT imply that installability alone means the plugin is immediately runnable.

#### Scenario: Built-in entry is installable but still requires configuration
- **WHEN** an official built-in plugin can be installed through the supported built-in flow but lacks required configuration for activation
- **THEN** the catalog response keeps the explicit install path available
- **THEN** the same response marks the built-in as not ready for activation and includes setup guidance describing the missing configuration

#### Scenario: Built-in entry is blocked on the current host
- **WHEN** an official built-in plugin is unsupported on the current host or missing a required local prerequisite
- **THEN** the catalog or discovery response includes the built-in with evaluated readiness and blocking guidance
- **THEN** the response does not misrepresent that built-in as fully runnable on the current host
