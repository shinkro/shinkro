# Sync Inspector UI refinement

Designer-to-engineering handoff, following an actual authenticated Chrome review of the Inspector, Plex Payloads, Anime Updates and Dashboard. This refines the presentation of the existing [design contract](sync-inspector-design.md); all API, approval, history and write safeguards remain authoritative. No new backend capabilities are required.

## Observed usability problems

The existing product language is sound: Verdana, Mantine, dark neutral surfaces over the existing pattern, bordered cards, gold outline controls in dark mode, MAL blue controls in light mode, and compact semantic badges. Keep that language. The problem is the arrangement and emphasis of information.

- The sibling history screens use a single bordered workspace and dense, aligned table. Inspector presents each event as a large separate card. At the reviewed desktop size, setup consumes most of the viewport and only one complete event is visible at the top. Repeated long source and MAL titles expand this further. Scanning a history of hundreds of events is slow.
- At 390px, the three filters remain in one row. Search and select values are clipped and labels split across lines. This is technically within the viewport but practically unusable.
- The detail screen first shows an expansive Received card and then historical explanation. The first recovery control is below the first desktop viewport and much farther down on mobile. Users must work through technical context before discovering the next useful action.
- Current preview puts the resolution trace and source catalogue diagnostics before the field changes and action. The destination, change and final decision are separated by scrolling. Several messages repeat the same write policy. History, new evaluation and editor occupy the same long vertical region with weak boundaries.
- Correction selection is functional but feels like form expansion: choose a title, choose an episode, use a correction, then separately preview. The persistent-rule editor expands inside this flow while the old current preview remains below it. Selection, one-event scope and persistent scope need clearer separation.
- Local rules has a full-width Reload rules button as its dominant control, generic identifiers as the main title and multiple equally weighted buttons. The destination, conversion, scope and revision need a consistent hierarchy.
- The development/write-policy banner uses a title, separate badge and paragraph, taking roughly 120px desktop and 160px mobile. This policy must remain persistent, but can communicate the same facts much more compactly.
- Secondary text is often too faint, particularly light-theme metadata over the patterned background. Content surfaces should carry operational content; the pattern should not compete with filters, explanations or metadata.

## Priority 1: coherent events workspace

Use the same 1200px container and page-heading scale/left edge as Plex Payloads and Anime Updates. Use one bordered Paper containing Events/Local rules tabs, filters and the list. The outer page heading has its existing one-sentence purpose and a compact secondary Refresh mapping data control. Keep refresh feedback adjacent, without a separate large permanent paragraph. A refresh is explicitly a cache update, never a retry.

At desktop, render a real table with Event, MAL outcome, Destination/source, Received and an explicit Inspect action. Use approximately 32% / 20% / 27% / 13% / 8% as a starting allocation, adjusted to actual available width. Rows should generally be 64–90px, with separators and the same restrained hover treatment as sibling histories. Titles are 14px semibold; episode/rating and library/context are secondary. Source and target are subordinate to the event name. Do not duplicate an entire long target title at full weight. Clamp long titles visually to two lines with a keyboard-accessible way to inspect the full title; event detail always displays it completely. Do not make hover the only access path.

Use plain outcome labels and secondary latest-attempt information. Errors are a short supporting line beneath the outcome, not an equally weighted paragraph spanning the card. Preserve the distinction between an original success and failed correction. Use relative received age for scanning, with exact datetime available accessibly, borrowing the existing AgeCell approach where sensible. Never infer success from receipt.

Desktop filters: search gets the most width; two narrower selects align to it. At less than roughly 700px: search spans the full width, and select controls either stack or form two usable equal columns with at least about 140px each. At 375px they must remain readable. Keep visible labels, adequate input height and predictable clear controls. Needs mapping is a selectable shortcut with a visible selected state; Clear filters should be understated and only offered when meaningful. Do not mislabel this shortcut Needs attention unless it actually filters all actionable outcomes on the server.

Below the filters show the matching count and current range/page context. Keep URL filters and Back restoration. Pagination has a contextual label and fits at 375px. Hide it for a truly empty result. Do not add unsupported counts, totals by status or charts.

At narrow widths use compact event cards/list rows within the same workspace. Show title/episode, outcome, destination/source and age. Inspect is a full, easy-to-reach action rather than a button that steals half of the title row. Avoid forcing users to horizontally scroll core content.

## Priority 1: detail built around the next decision

Use the same content width as the list; the current narrower 1000px detail is unnecessarily detached from sibling alignment. Header: Back to events, complete event title, episode/activity and stable event number, outcome. Provide a concise source-to-destination summary when actually known. Keep outcome distinct from Plex receipt.

Immediately beneath the header, show a compact **Recorded result** summary: original outcome, original target if any, readable error and error code when applicable, received age/time and source/episode. Legacy trace limitations appear as one quiet sentence, not a large alert dominating the result. Preserve genuinely recorded facts. This is the original result even after a corrective attempt advances the event summary.

Next comes the main **Preview with current data** workspace, visible at the top of the default recovery flow. Before preview, give one concise instruction and Preview now as the main outline action. Choose a different MAL entry is secondary; Refresh mapping data is tertiary. All three cannot appear as competing primary controls. On mobile stack full-width useful actions and keep about 40px targets.

After preview, show these in this order:

1. A labelled Current evaluation timestamp and status (ready, blocked, no change, expired or state changed), plus the selected destination/link and mapping-origin badge.
2. A compact conversion summary: source season/episode → target MAL episode. Movies and rating-only actions use their real semantics.
3. The complete changed-field review, Current on MAL → Proposed. Use normal sentence case for enum values. Unknown/unavailable is not zero. On mobile use stacked current/proposed values or a deliberately fitted two-value layout; no horizontal scrolling.
4. Applicable warning(s), expiry, visible write-disabled or blocked reason, and the result-specific apply action together. The reason must be readable even when its disabled button cannot receive focus. No redundant policy warning above and below the same action. No-change and blocked states should provide their useful next action rather than an inert primary-looking button.
5. **How this match was found** disclosure for resolution steps, data used and source diagnostics. Preserve refresh versus local observation truth. Technical detail is discoverable and fully keyboard accessible, but does not delay the review decision. An account binding disclosure remains available.

Received metadata and full historical history follow this workspace, or may sit in a desktop side column only if reading order still puts the result and action first. Metadata should use a compact definition grid, not six rows of oversized spacing. Attempts form a clear compact chronological history with labelled original/retry/correction, result, time and destination. Expand recorded traces, planned diffs and provider observations on demand. Keep the exact labels MAL at preview / Planned change / Observed on MAL and observation time; never pass a saved plan off as a new remote read.

An open correction editor must visibly replace or supersede the apply-ready decision region. Any retained older preview is marked as previous/reference and cannot masquerade as the current draft. Preview requests and save success should update a polite live region and bring the new result into view deliberately, without surprising jumps for background updates. Do not auto-run previews or writes on open/navigation.

## Priority 1: correction and rules as explicit choices

The correction editor needs a clearly bounded surface, title, contextual source episode and a visible **This event only** scope. Explain in one sentence that future mappings stay unchanged. Use a small visual progression such as 1 Choose destination / 2 Review correction; this is presentation only, not a mandatory wizard with extra clicks.

Search has its full label, usable width and loading/no-results/failure states. Results show verified title, MAL ID, type and known episode count; missing metadata remains honestly unavailable. Selected entry has a visible Selected state and is restated beside the episode input, without repeating the whole result list indefinitely. Use this correction should be labelled **Use for this event** with adjacent text explaining that a fresh preview is next, or become **Preview correction** only if the existing API/state flow can safely make the next read directly. Never imply that this action syncs MAL.

Saving a persistent rule is a secondary explicit path, visually separated from one-event confirmation. When opened, give it its own heading and a Back to event correction/Cancel affordance. Keep target identity and source scope visible. Group fields under Source range and Destination conversion; on mobile stack them. Show first/last and current-event conversion in a concise readable summary. Server scope consent remains explicit and unchecked for missing historical server identity. Distinguish scope warning from the informational future-event impact text; use one restrained warning, not multiple large colored blocks.

Rule review must visually progress from edit → Review rule impact → reviewed summary → Save local rule. Display affected historical count/events and conflicts before save, explaining that none are retried. A changed field invalidates review. Save enabled state, revision conflicts and discarded-draft confirmation remain as implemented. Success offers Preview this event, with no automatic retry. Saving does not publish community mappings.

Local rules tab uses the same bordered workspace. Header has concise explanation, a compact Reload rules utility and a count derived only from fetched data. Each rule presents source → MAL destination/range first, state badge nearby, then context/server scope and revision/time as secondary metadata. Edit is the main row action; enable/disable is a secondary state-specific action; source-event navigation is a text link. Keep disabled rules fully readable. Edit/re-enable/disable require the existing impact/revision confirmations, with a meaningful summary and clearly distinguished Cancel/Confirm actions. No unsolicited persistent changes during designer review.

## Priority 1: states, accessibility and cohesion

Compact the global write-policy banner using a single heading/status line and one short supporting sentence. Retain Development build where supplied and the unambiguous statement that MAL writes are disabled. Never hide capability failure or infer enabled writes from missing data. The policy is still visible outside detail. Changing its typography/spacing is allowed; removing server safeguards is not.

Use theme variables and Mantine components, existing radius/borders and existing outline-button vocabulary. No gradients, new font, oversized hero, special sidebar, animated progress decoration or invented success statistics. Use primary for actions, semantic colors for outcomes. Improve essential label/body contrast in both themes; do not set the whole screen to tiny dimmed text. Aim for WCAG AA text contrast (4.5:1 body, 3:1 large text/UI boundaries) and clearly visible focus against light and dark surfaces.

Use semantic headings, labels and buttons/disclosures; maintain correct focus return and keyboard selection. Status changes use live regions. Loading skeletons match the eventual table/card density; errors preserve filters/drafts and offer a local retry; separate No sync events yet from No events match these filters with Clear filters; rule empty state explains creation from an event and links to Events. Loading must not show a false zero count or premature empty state. Stale/expired previews stay visible as reference with disabled apply and Preview again. Capability/source/session errors remain actionable and local to the affected operation.

## Suggested implementation boundaries

Keep existing query and mutation ownership. Extract presentation helpers such as EventList/EventRow, EventFilters, RecordedResult, PreviewWorkspace, MatchDetails, AttemptHistory, CorrectionEditor and RuleSummary where this makes the JSX understandable. A shared scoped CSS module can define responsive grids, metadata, filter controls and touch sizing. Reuse existing table/time components when compatible; do not impose a new global theme or refactor unrelated screens.

## Designer acceptance and evidence

Designer must review the rendered implementation after engineering, not just the diff. Review at default desktop size, 390px and 375px, dark and light. Private screenshots from the copied personal database stay outside Git. Use public-safe fixtures for checked-in tests.

- At approximately 1440×800 desktop, the list header, functional filters and at least four complete ordinary event rows are visible without scrolling; long titles do not turn ordinary rows into giant cards. It clearly belongs beside the existing history tables.
- At 390×844, search and select values are readable, no three squeezed columns remain, and an event's Inspect action is discoverable and comfortably tappable. Page width stays within viewport at 375px as well.
- The failed-event initial desktop view shows recorded failure and Preview now together. At mobile size the preview action is reachable within the initial screen or a short natural scroll, rather than after the entire Received/history stack.
- A resolved preview puts destination, episode conversion, changed-field review and action/disabled reason in one coherent compact region. Diagnostics no longer separate the decision from its action. Current evaluation and historical result cannot be confused.
- Opening correction makes its scope/selected target/next step evident; persistent-rule review is clearly separate and progressive. No enabled apply survives an edited/open correction.
- Rule lifecycle summaries are legible on mobile, review invalidation/discard protection remain correct, and enabling/disabling is explicit.
- Exercise successful no-op, failed mapping, legacy input gaps, no results, truly empty data, loading/error, stale/expired preview, disabled writes, long titles and rule conflicts. Existing fake-provider approval/history/observation tests continue to pass after adapting disclosure interactions.
- Keyboard can reach Inspect, preview, correction search/selection, every disclosure and rule controls. Focus is visible and essential error/disabled explanations require no hover.

Approval is contingent on these rendered criteria and resolution of concrete review findings. A passing build or browser suite alone is not design approval.

## First implementation review

The first engineering implementation substantially improves the information architecture: a coherent table workspace, usable narrow filters, recorded result followed immediately by recovery, grouped current evaluation, and explicitly scoped correction/rule progression. Actual Chrome inspection also confirmed the content-based card breakpoint works with the persistent sidebar at 820px and 900px viewport widths.

Designer requested a further implementation round after reviewing desktop, 390px and 375px in both themes:

- Restore the available human-readable MAL destination title in list rows/cards rather than showing only an opaque MAL ID.
- Give narrow event headings their full width; the outcome badge was taking enough width to turn an ordinary title into three lines and delay the first action.
- Replace the cramped narrow three-column diff with labelled current/proposed values that do not split a word such as Watching across lines. Preserve saved-history labels in the same responsive treatment.
- Correct light-theme amber/status contrast and essential secondary metadata contrast, plus the dark server-scope consent description on an amber surface. Use sentence case for readable enum values.
- Mark a retained editor-open preview as Previous preview rather than retaining a green Ready to review badge.
- Remove only the exact repeated disabled-write policy warning when its concise visible inline reason is already present. Preserve all distinct backend warnings and the global server-capability banner.

Rule lookup, explicit selection, persistent-scope review, affected-event explanation and discard protection were exercised without saving a rule or applying a MAL update. These observations do not constitute final designer approval; the amended render requires another review.

## Second implementation review

Actual Chrome re-review confirmed the first-round corrections: readable subordinate MAL titles, full-width narrow headings, labelled narrow current/proposed change cards, sentence-case watch statuses, legible operational metadata and amber states in light mode, readable server-scope consent in dark mode, and a gray Previous preview badge while editing. The event-only correction action was exercised through its fresh scoped preview. Exact duplicate policy text is removed while distinct server-identity and previous-target notices remain visible. No external update or rule save was performed.

Two final refinements remain from this rendered review: real long anime names in both event and destination columns still allow only three complete rows at the agreed desktop target, and the light success badge measured 3.81:1 contrast at its small text size. Engineering was asked to compact subordinate destination/cell spacing and strengthen long-title density fixtures, plus deepen semantic text colors and check success/error/processing states. Approval remains pending the final render of these corrections.

## Final designer acceptance

The final implementation is approved against this UI-refinement brief after three rendered review rounds and targeted revisions. Actual authenticated Chrome review compared the Inspector with the existing history pages, then exercised the list, recorded failure, current preview, destination lookup/selection, event-only correction preview, persistent-rule scope/impact presentation and existing-rule editor. Dark and light themes were inspected at desktop, 375px and 390px; the sidebar/content behavior at 820px and 900px was also reviewed. The final result retains shinkro's visual language while making scanning, diagnosis and the next decision substantially clearer.

- At 1440×800, all four initial real event rows, including ordinary long source and destination names, fit completely; the fourth row's measured lower edge was approximately 781px. Available destination names remain visible as subordinate text alongside MAL IDs, with full titles accessible from detail.
- Narrow search/select controls are readable, the event heading uses its full width, and Preview now is visible at the bottom of the initial 375×844 failed-event view. Core mobile actions and the quick-filter, clear-filter and header refresh utilities have practical 40px targets.
- Current destination, conversion, complete changed-field review, applicable warning, expiry, disabled reason and action form one coherent region. At 375px the labelled current/proposed cards keep values such as Watching intact. Matching diagnostics remain available through a keyboard-operable disclosure; Enter was exercised in Chrome.
- Light amber and success labels and essential metadata are visibly readable. The formerly faint light success label was deepened; engineering's targeted computed checks cover success, amber, error, processing and neutral states at 4.5:1 or above. Actual Chrome style inspection confirmed the revised success color and surface. These samples are targeted verification, not a complete accessibility certification.
- A retained old evaluation is gray and explicitly Previous preview while a correction is open. A fresh event-only correction preview remains labelled This event only. Exact duplicated disabled-write copy is removed while distinct server-identity and previous-destination warnings remain. The server write policy remains visible globally and beside the disabled action.
- Persistent-rule navigation protection was exercised through Events, the source-event link, Back to events and browser Back. Keep editing preserved the actual changed/selected values; Discard and continue completed navigation without saving. Opening only the event correction lookup does not block navigation. Confirmation is legible on mobile and returns to the appropriate workflow.

No rule was saved or enabled, and no MAL update was applied during designer review. Screenshots from personal data are retained privately outside the repository. Loading/error, stale/expired, empty-history, semantic contrast and navigation cases also have browser fixtures; engineering reported all 44 fixtures plus build and lint passing before this final review. Independent final QA and release/deployment verification remain separate from design acceptance.

### Keyboard-selection follow-up

Independent final QA found that selecting a correction result removed the focused Select entry button and left focus on the document body. Engineering added explicit focus restoration for deliberate correction transitions, and designer re-review verified the real episode flow at desktop and 390px: Enter on Select entry focuses the visible Target MAL episode input; natural Tab reaches Preview correction; Change entry focuses the MAL lookup; entering the persistent-rule subview focuses its heading; Keep editing retains that context; Back followed by explicit discard restores the episode input with its edited value intact. Ordinary typing retained input focus. No rule save or MAL update was performed. The movie/disabled-episode action fallback is covered by public-safe browser fixtures; engineering reported all 48 browser tests, build and lint passing. This focused follow-up is approved and closes the discovered keyboard transition gap.
