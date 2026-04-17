## ADDED Requirements

### Requirement: DingTalk and WeCom SHALL declare full_native_lifecycle with explicit mutable update methods

The DingTalk provider SHALL advertise `ReadinessTier=full_native_lifecycle` with `MutableUpdateMethod=openapi_only` so operators know mutable updates apply only to cards originally sent via DingTalk OpenAPI (webhook-origin cards are not mutable). The WeCom provider SHALL advertise `ReadinessTier=full_native_lifecycle` with `MutableUpdateMethod=template_card_update` so operators know mutable updates flow through the template-card API.

#### Scenario: DingTalk reports openapi_only mutable update method
- **WHEN** Bridge registers with the control plane using the DingTalk live transport
- **THEN** the registration payload carries `readiness_tier=full_native_lifecycle` and `capability_matrix.mutableUpdateMethod=openapi_only`

#### Scenario: WeCom reports template_card_update mutable update method
- **WHEN** Bridge registers with the control plane using the WeCom live transport
- **THEN** the registration payload carries `readiness_tier=full_native_lifecycle` and `capability_matrix.mutableUpdateMethod=template_card_update`

### Requirement: QQ Bot SHALL declare native_send_with_fallback tier with OpenAPI PATCH mutability

The QQ Bot provider SHALL advertise `ReadinessTier=native_send_with_fallback` with `MutableUpdateMethod=openapi_patch` to reflect that markdown messages dispatched via OpenAPI can be updated through the `PATCH /messages/{id}` endpoint. Mutable update requests targeting webhook-origin messages MUST degrade with `fallback_reason`.

#### Scenario: QQ Bot registration advertises openapi_patch mutability
- **WHEN** Bridge registers with the QQ Bot live transport
- **THEN** the registration payload carries `readiness_tier=native_send_with_fallback` and `capability_matrix.mutableUpdateMethod=openapi_patch`

### Requirement: QQ (OneBot) SHALL declare simulated mutable update truthfully

The QQ provider SHALL advertise `MutableUpdateMethod=simulated` so operators and backend consumers know that mutable updates are implemented as "delete old + send new + preserve thread context" rather than a native edit API. The readiness tier SHALL remain `text_first` so no richer-delivery pretence enters the catalog.

#### Scenario: QQ registration advertises simulated mutable update
- **WHEN** Bridge registers with the QQ OneBot live transport
- **THEN** the registration payload carries `capability_matrix.mutableUpdateMethod=simulated`
