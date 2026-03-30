# enriched-structured-message Specification

## Purpose
Define the enriched shared structured-message contract for AgentForge IM Bridge so richer section-based content can be rendered across providers without forcing provider-native payloads.

## Requirements
### Requirement: StructuredMessage SHALL support typed sections for richer content expression
The `StructuredMessage` struct SHALL include an optional `Sections []StructuredSection` field. When `Sections` is non-empty, platform renderers SHALL prefer it over the legacy `Title`/`Body`/`Fields`/`Actions` fields. Each `StructuredSection` SHALL have a `Type` string and typed content pointer for the section variant.

#### Scenario: Structured message with sections overrides legacy fields for rendering
- **WHEN** a `StructuredMessage` has both legacy `Title`/`Body` fields and non-empty `Sections`
- **THEN** platform renderers use `Sections` for output construction
- **AND** legacy fields are ignored for rendering but remain available for `FallbackText()`

#### Scenario: Structured message without sections uses legacy rendering
- **WHEN** a `StructuredMessage` has legacy `Title`/`Body`/`Fields`/`Actions` but empty `Sections`
- **THEN** platform renderers use the legacy field-based rendering path unchanged

### Requirement: Section types SHALL include text, image, divider, context, fields, and actions
The system SHALL define the following section types: `text` (markdown or plain body), `image` (URL + alt text), `divider` (visual separator), `context` (small auxiliary text, e.g., timestamps or authors), `fields` (multi-column key-value grid), and `actions` (button grid with optional layout hints like buttons-per-row).

#### Scenario: Text section carries formatted body content
- **WHEN** a `StructuredSection` has `Type: "text"` with a `TextSection` containing body content
- **THEN** platform renderers output the body using the platform's supported text format
- **AND** `FallbackText()` returns the raw body content

#### Scenario: Image section carries URL and alt text
- **WHEN** a `StructuredSection` has `Type: "image"` with an `ImageSection` containing `URL` and `AltText`
- **THEN** platform renderers that support images render them inline
- **AND** platforms without image support include the alt text and URL in fallback text

#### Scenario: Divider section produces visual separation
- **WHEN** a `StructuredSection` has `Type: "divider"`
- **THEN** platform renderers output a platform-native divider (Slack divider block, Discord empty field, Telegram `---`)
- **AND** `FallbackText()` returns a `---` separator line

#### Scenario: Context section renders auxiliary metadata
- **WHEN** a `StructuredSection` has `Type: "context"` with a `ContextSection` containing elements
- **THEN** platform renderers output small/muted text appropriate to the platform
- **AND** `FallbackText()` returns context elements joined with ` | `

#### Scenario: Fields section renders multi-column key-value pairs
- **WHEN** a `StructuredSection` has `Type: "fields"` with a `FieldsSection` containing labeled values
- **THEN** platform renderers output a multi-column layout where supported (Slack fields, Discord embed fields)
- **AND** platforms without column support render fields as `label: value` lines

#### Scenario: Actions section renders a button grid with layout hints
- **WHEN** a `StructuredSection` has `Type: "actions"` with an `ActionsSection` containing buttons and `ButtonsPerRow: 3`
- **THEN** platform renderers that support button grids arrange buttons in rows of 3
- **AND** platforms without grid support render buttons as a flat list

### Requirement: Per-platform structured section renderers SHALL convert sections to native output
Each platform with a structured surface SHALL implement a section renderer that converts `[]StructuredSection` to platform-native output. Unsupported section types SHALL be skipped with their fallback text appended to the output.

#### Scenario: Slack renders sections as Block Kit blocks
- **WHEN** a `StructuredMessage` with `Sections` targets Slack
- **THEN** text sections become `section` blocks, images become `image` blocks, dividers become `divider` blocks, context becomes `context` blocks, fields become `section` blocks with field objects, actions become `actions` blocks with buttons

#### Scenario: Discord renders sections as embed fields and components
- **WHEN** a `StructuredMessage` with `Sections` targets Discord
- **THEN** text sections become embed description segments, images become embed thumbnails or image URLs, fields become embed fields with inline layout, actions become component action rows

#### Scenario: Telegram renders sections as formatted text with keyboard
- **WHEN** a `StructuredMessage` with `Sections` targets Telegram
- **THEN** text sections become message text segments, images become text links (Telegram doesn't support inline images in text messages), fields become formatted lines, actions become inline keyboard button rows

#### Scenario: DingTalk renders sections as ActionCard markdown
- **WHEN** a `StructuredMessage` with `Sections` targets DingTalk
- **THEN** text sections become markdown body segments, images become markdown image links, fields become markdown formatted lines, actions become ActionCard buttons

#### Scenario: Unsupported section type degrades to fallback text
- **WHEN** a platform renderer encounters a section type it does not support
- **THEN** the renderer includes the section's `FallbackText()` output in the platform-native message
- **AND** rendering continues with subsequent sections without error

### Requirement: FallbackText SHALL incorporate sections when present
When `Sections` is non-empty, `StructuredMessage.FallbackText()` SHALL generate text from sections instead of legacy fields. Each section's `FallbackText()` SHALL be joined with newlines to produce the complete fallback output.

#### Scenario: Sections-based fallback text
- **WHEN** a `StructuredMessage` has sections: text("Hello"), divider, fields([{Status: Active}]), actions([{Label: Approve}])
- **THEN** `FallbackText()` returns `"Hello\n---\nStatus: Active\nApprove"`
