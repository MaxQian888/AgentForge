## ADDED Requirements

### Requirement: Native interaction matrix SHALL include thread policy and reaction delivery

The native interaction matrix SHALL record per-provider thread-policy support and reaction-delivery capability so operators can see which providers honor `reuse/open/isolate` natively and which degrade. The matrix MUST surface both the unified thread policy list and the supported reaction emoji set.

#### Scenario: Operator inspects /im/health for thread support
- **WHEN** an operator checks `/im/health`
- **THEN** the response includes `supports_threads`, `threadPolicySupport`, and the list of supported reaction codes
- **AND** the values match what the bridge actually honors at delivery time
