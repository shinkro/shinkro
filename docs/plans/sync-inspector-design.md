# Sync Inspector design contract

Product design for Sync Inspector. This complements [the implementation plan](sync-inspector.md); backend capabilities remain authoritative. Read-only instances must clearly distinguish inspection and preview from external MAL writes.

## Product journey

The primary journey is **find a failed sync → understand the match → preview a repair → explicitly apply**. Successful syncs are equally inspectable. Receipt by Plex is a separate fact from success on MAL.

The Beerus acceptance journey is concrete: select the original AniDB 19859 / S1E2 failure; see the historical `ANIME_NOT_IN_DB` result; refresh catalogue data; preview again; see MAL 63367 and episode 2; inspect current versus proposed progress; explicitly apply if writes are enabled. If the catalogue still lacks the link, choose MAL 63367 as an event-only correction and preview that choice. Historical failure remains visible after successful recovery.

## Existing visual language

Use the current Mantine theme, Verdana, outline buttons, Plex gold accents in dark mode and MAL blue in light mode. Keep the existing AppShell and mobile navigation. No new design system, large illustrations, confidence scores, or JSON-first details.

Use a `Container size={1200}`, `Stack gap="md"`, and bordered `Paper p="md"` cards, as existing history screens do. Use real headings, muted metadata and restrained semantic badges. A text label always accompanies a result icon; color and hover tooltips cannot carry essential meaning. Show IDs as selectable text and external provider links with an accessible label.

## Navigation and event list

Add **Sync Inspector** beside Anime Updates. The main screen contains **Events** and **Local rules** tabs. A stable event detail URL uses the original stored Plex event ID. Existing Plex Payloads and Anime Updates receive a clearly labelled Inspect link; dashboard recent activity should link to the same detail route.

The Events header has the page title, one sentence of purpose, and **Refresh catalogue**. Describe refresh as updating locally cached mapping/catalogue data; never imply it repairs events or updates MAL. Show the active source's last successful refresh when the backend supplies it; an unknown time is “Refresh time unavailable,” not the event's creation time.

Filters: title or provider/MAL ID search; result; source provider. More filters may include event type and time when supported. The default is all events, newest received first, with a visible **Needs attention** preset. Filters and pagination belong in URL search parameters so Back restores context. Reset pagination after a filter changes.

Columns:

| Column | Content |
| --- | --- |
| Event | Plex title, S1 E2 or Movie or Rating 8/10, library/user as secondary text |
| Result | Plain MAL outcome; latest corrective attempt is distinguished from original outcome |
| Match | MAL title/ID when known, otherwise source provider and ID |
| Received | Relative time with exact timestamp accessible on focus/hover |
| Action | Explicit Inspect link, usable without clicking the row |

One row represents one event, not one attempt. Original success plus failed correction is labelled “Correction failed” with “Original sync succeeded” as secondary text. When no MAL attempt is known, use “Not recorded”; never infer success from Plex receipt. An empty list says “No events match these filters” and offers Clear filters. An empty database says “No sync events yet” and explains where events will appear. Loading uses skeleton rows; failures show a local retry action without erasing filter inputs.

On narrow screens stack filters full width and show compact event cards with title, episode, result, received time and Inspect. A horizontal table scroll fallback is acceptable initially if text remains readable and the action is reachable. Do not make core detail content horizontally scroll.

## Event detail

Header: Back to events, event title and episode, current outcome badge, event ID in secondary text. Four cards follow in reading order. On wide screens Received metadata may use two columns; the whole journey still works as a single vertical stack.

### 1. Received

Show original provider/ID, source season and episode, event type, user/library, received time, and source event time only when supplied. Distinguish received time from playback time. Show “Plex event received” separately from the MAL outcome. Advanced persisted inputs may be collapsed under “Technical details”; do not expose tokens or full raw payloads.

### 2. What happened

This is historical: the original attempt and any subsequent applied attempts. Display the saved trace as a short ordered list: source extraction → selected mapping and origin → converted episode → MAL result. Each step includes actual IDs and arithmetic when known. Selected-rule badges include Direct MAL, Canonical AniDB, Community rule, Custom file, or Local override.

For pre-inspector events say: **“This event predates detailed sync traces. The saved result is shown below; a new preview uses current data and settings.”** Show all genuinely persisted error facts. Never fabricate historical trace steps by running today's resolver.

If the old target differs from a correction, show it and explain: **“Applying this correction leaves MAL [old ID] unchanged.”** Do not hide that notice inside a tooltip.

### 3. Preview with current data

Initial state shows **Preview now** and secondary **Choose a different MAL entry**. Preview is a read operation with no MAL list write. Refreshing catalogue or editing a correction invalidates the visible approval; require a fresh preview.

The completed preview contains:

1. Evaluation time and current catalogue/rule revision if available.
2. Resolution trace and selected MAL title/ID, linked to MAL.
3. Source-to-target episode calculation; for example `S1 E2 → MAL 63367 · E2`.
4. Every field the backend proposes to change, in a compact table **Field | Current on MAL | Proposed**. Progress, status, score, rewatch state/count and dates appear when touched; do not infer missing fields or replace unknown values with zero. Empty values are “Not set”; an unavailable read is “Unavailable.”
5. Account name or binding when backend supplies it, expiry, warnings, and one primary action.

Use a result-specific action such as **Sync MAL 63367 to episode 2**. For rating-only changes use **Update score on MAL 63367**. A preview already satisfied shows **No change needed** with no write action. A blocked preview displays the reason and one useful next action. During submission show “Submitting…”; after acceptance show the durable job status, not premature success. Poll the accepted job while processing, then invalidate detail/list/history queries after the final result. Navigating away must not cancel accepted server work.

An expired or stale preview is retained visibly for reference, labelled **Preview expired** or **State changed**. Its apply button is disabled and the primary action becomes **Preview again**. A fresh preview always requires renewed approval. A failed preview does not replace the historical attempt.

### 4. Attempts

Use a compact timeline, newest first, with original/live/retry/correction labels, started/finished times, target and episode, outcome, and readable error. Expand an item for its saved trace and field changes. Historical comparisons say **MAL at preview** and **Planned change**, with their evaluation time; they are never labelled as a fresh current read. A planned after-value is not proof of a remote result. If a saved provider observation is available, label its separate observed values and observation time explicitly. A queued or uncertain attempt is plainly distinguishable from confirmed success. Errors are sanitized. Do not dump OAuth data, raw HTTP headers or stack traces into customer-facing content.

## Correction editor

Open an in-page bordered editor or a large Modal with a clear title **Choose a MAL entry**. Keep event context visible. Default scope is **This event only**. Explain: “Changes this event's destination. Future events keep their current mappings.”

Support title search and direct numeric MAL ID. Use explicit **Search** or debounced search with loading state; show title, MAL ID, type and episode count where available. A result is selected only by a deliberate user action. Numeric IDs require server validation/lookup before an apply-ready preview. Preserve typed input on provider failure. Do not silently choose the first search result.

For a one-event episode correction, display the source episode and editable target MAL episode, initially equal to the actual original source episode only when valid. Selecting a new title clears the old preview. The editor's primary action is **Preview correction**, not Sync. Cancel restores the previous event view without saving a rule.

**Save a local rule** is a separate explicit path after target selection. The initial editor supports a movie target or source season/episode range. Fields are provider/ID (captured from event), server/library scope when available, season, source episode start/end and target MAL episode start. AniDB rules are limited to explicit season one. Missing required account/library identity disables persistent save with a visible explanation. A legacy event without server identity can offer an explicit choice of all servers within the displayed account and library; show that broader scope during review and never select it silently. Event-only correction remains available. Do not invent an unrestricted global scope.

Show a human-readable scope summary before save, e.g. “AniDB 19859 · season 1 · source episodes 1–12 → MAL 63367 · episodes 1–12 · this library.” Show first/last conversion and the current event conversion; a longer table may expand. Backend validates ranges, known MAL episode bounds and conflicts. Inline field errors stay beside the relevant fields. For overlap/conflict show the conflicting existing rule and require the user to narrow or edit scope; do not choose a winner in the browser.

The review states **“Saving changes matching for future events. Existing events are not retried.”** Action: **Save local rule**. A successful save offers **Preview this event**; it does not run a MAL update automatically. Existing custom YAML remains the chosen base and is identified separately from these additive rules.

## Local rules management

List provider/ID, source scope, MAL destination/range, enabled state and latest revision/time. Expose supported create, inspect/edit and enable/disable actions. Changes require backend revision checks and an explicit impact review; editing fields invalidates that review. Deactivation uses a specific confirmation naming scope and destination, explains future matching returns to the selected base, and does not undo previous MAL updates. An inactive rule remains visible and labelled. Never imply a local save is published to community mappings.

On revision conflict show “This rule changed elsewhere. Reload it before saving.” Final lifecycle endpoint details must be reflected in the [API contract](sync-inspector-api.md) before release. Do not render placeholder controls for any unavailable capability.

## Outcome and action contract

Backend typed outcomes and capability reasons drive labels and actions. The UI never treats an unknown status as success or interprets an unrecognized error string as safe to replay.

| Condition | User-facing state | Next action |
| --- | --- | --- |
| No mapping/catalogue link | Needs mapping; show original provider/ID | Refresh catalogue; Preview now; Choose MAL entry |
| Equally valid candidates | Needs review; show alternatives and source | Explicit event correction or edit conflicting local rule |
| Required provider unavailable | Source unavailable; last successful snapshot age if known | Retry fetch; keep cached explanation labelled |
| MAL OAuth missing/expired | Needs sign-in | Open existing MAL settings; preview again afterward |
| MAL state unavailable | Cannot preview MAL changes | Retry preview; apply unavailable |
| Preview expired | Preview expired | Preview again |
| Relevant MAL/account/rule state changed | State changed; summarize safe reason | Preview again; never auto-approve |
| Legacy fields incomplete | Preview/retry unavailable; list missing fields | Inspect recorded facts; no guessed replay |
| Known higher progress or stale rating | Needs review or No change needed, as backend decides | Explain preserved state; no generic force button |
| Write outcome uncertain | Checking MAL result / Needs review | Follow server reconciliation; no blind retry |
| Dev writes disabled | Persistent development banner and inline disabled reason | Inspection, lookup and preview remain available |

Use green for Synced/No change needed, blue for Queued/Processing, amber for Needs mapping/Needs review, red for Failed/Needs sign-in, gray for Cancelled/Not recorded. Source unavailable is amber when a validated cache remains usable and red when required data is unavailable; the text explains which applies. State never relies on color alone.

## Read-only instance safeguards

Persistent banner: **“MAL writes are disabled on this instance.”** Show this outside detail so read-only capability is unambiguous. Actual write capability comes from the server; hiding a button is not a security boundary. Apply controls remain disabled with visible text when writes are disabled, while preview and local rule functionality follow server capabilities. Onboarding must distinguish missing authentication from intentionally disabled writes.

A source refresh modifies only the current instance's catalogue. Local rule mutations modify only the current instance's database. Do not emit sync notifications during preview. If writes are enabled later, the banner must accurately reflect current capability; the frontend does not override it from a query parameter.

## Accessibility and implementation acceptance

- Semantic headings and form labels; no placeholder-only labels. All buttons and provider links have accessible names. Status and error updates use a polite live region; async focus does not jump unexpectedly.
- Keyboard opens detail and correction editor, selects a result and reaches preview/apply. Modal traps focus and returns it to the trigger. Critical disabled reasons remain visible on touch devices.
- Controls wrap at 375px, long titles/IDs wrap without pushing the viewport, and diffs use stacked Current/Proposed values if a table is too wide. Keep practical touch targets of about 40px.
- Preserve correction input and historical trace on search/preview failure. Discarding an edited but unsaved persistent rule requires confirmation; simple browsing does not.
- Loading, error, empty, preview blocked, successful no-op, stale preview, legacy missing input and dev-disabled views must be exercised alongside the happy path.
- Browser review includes a recoverable legacy missing-link event, an inspectable successful event, a range correction with offset, and narrow-screen list/detail. API fixtures cover states not reproducible from the available test data.

## Implementation handoff

Use the existing API client and TanStack Query cache; namespace new query keys for event lists, event detail, previews, jobs and rules. Keep API response types in the API module and render declared fields rather than guesses. Mutations must invalidate exactly the affected records. Final endpoint and DTO names are coordinated with backend; this document describes the customer contract rather than prescribing incompatible wire names.

Preferred component boundaries: `SyncInspector` list/tabs; `SyncEventDetail`; `SyncOutcomeBadge`; `ResolutionTrace`; `MalChangeDiff`; `MappingCorrectionEditor`; `LocalRules`. A drawer is unnecessary for the main detail because the journey needs a deep link and enough space to review a write.

Do not ship inert primary actions. If a backend capability is unavailable, show a clear explanation and actual supported recovery action. Reach the complete inspect → preview → correct → retry journey before claiming the feature complete; the read-only dev deployment is a separately labelled review stage.

## First-build visual review

Reviewed actual Playwright renders of the event list, Beerus current preview, correction picker and range/impact editor at desktop and narrow widths. Additional 1440px and 390px fixtures exercised the visible development banner, disabled apply control and inline disabled reason. The 390px page remained within the viewport; the source/target episode, three-column MAL progress diff, range summary and stored-event impact were readable. The historical missing-link explanation and new canonical AniDB evaluation remained visually distinct.

Two review findings were handed to engineering: disable an existing approval while its correction editor is open, and render the original attempt in the historical card even after the backend's current event summary advances to a successful retry. Subsequent source review confirms both behaviors are implemented. Fixture recovery tests must advance the latest event summary to catch the latter. Saved full-page screenshots can place a fixed AppShell header partway down the image when captured after auto-scroll; viewport captures at the top provide clearer handover evidence.

Observed-result rendering uses separate persisted evidence: `observed`, `observedAt` and `observationSource`. Customer labels distinguish a MAL update response, a later reconciliation read, and a read showing no change was needed. An unknown result has no observed-state card. The immutable planned after-state is never presented as provider evidence. Source review confirmed this separation; live integration and visual review remain separate checks.

This review verifies the implemented layout and fixture interactions; integration validation still needs an actual backend event, the final lifecycle contract and the final browser suite. Visual review alone never authorizes an external MAL write from a read-only instance.
