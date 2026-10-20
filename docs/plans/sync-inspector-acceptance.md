# Sync Inspector release acceptance

This checklist makes the [implementation plan](sync-inspector.md) reviewable as a customer journey. The first release must deliver inspect → refresh or correct → preview → retry, including successful events whose destination was wrong. Automatic retries, advanced mapping editors, community PR submission, library backfill, and PostgreSQL remain follow-ups.

## Product contract

- **One event, one inspector:** every received Plex event has a stable detail URL and an attempt timeline. The list summarizes the actual MAL outcome, separately from successful webhook receipt.
- **Explain the decision:** show original provider, source season/episode, MAL destination and converted episode, mapping origin, useful failure reason, and the next available action. Show source freshness when known. Do not invent confidence percentages or historical facts.
- **A preview makes no MAL list changes:** it names the destination and every changed field. Apply requires a fresh, server-issued approval token and an explicit button naming the effect. Reopening or refreshing a page cannot submit a write.
- **Fixes are personal by default:** an entered MAL ID applies to this event unless the operator explicitly saves a scoped local rule. Saving a rule does not replay earlier events or publish anything.
- **Recovery is conservative:** no silent progress decrease, duplicated rewatch completion, stale rating overwrite, or automatic undo of another MAL entry.
- **The development copy is isolated:** copied history/settings are preserved, while webhook ingestion and MAL writes are disabled until deliberately enabled. Visiting the dev site must not mutate the production MAL account or receive production webhooks.

## Beerus release walkthrough

Use a representative missing-catalogue fixture: Dragon Ball Super: Beerus, AniDB `19859`, season 1, source episodes 1 and 2, MAL `63367`, error `ANIME_NOT_IN_DB`. Do not apply production events to a real MAL account as a release test.

1. Open Sync Inspector and select **Needs mapping**. Find the title by text or source ID. Each episode appears once. The card/row identifies its MAL sync failure even if Plex receipt was successful.
2. Open S1E2. Received information says AniDB `19859`, season 1, episode 2. Existing failure remains visible. For a legacy event show **Historical decision trace unavailable**, rather than presenting today's resolution as the old result.
3. Select **Refresh mapping data**. Explain whether the catalogue changed and when it was last checked. A refresh failure preserves the last valid catalogue and has an actionable explanation.
4. Select **Preview now**. With the corrected catalogue, show **Canonical AniDB → MAL 63367**, episode 2, without requiring a personal mapping. Show the actual current MAL values and exact proposed values. If MAL is already beyond episode 2 in the same watch cycle, show **No change needed**.
5. With a fixture catalogue still missing that link, select **Use another MAL entry**, enter `63367`, and verify the title before previewing. The default is **This event only**. The preview must clearly identify the correction's origin.
6. In a fake-MAL test, explicitly apply the preview. The same event gains a new attempt and a terminal result, with the original failure retained. A repeated submission returns the same logical job and performs no duplicate write.
7. Separately save an AniDB season-one rule, scoped to the available stable context. The impact preview shows matching stored events; nothing is retried automatically. An unrelated season/title/library retains its normal routing. Disabling the rule restores the selected automatic/community/custom-file behavior.

## Release-blocking acceptance matrix

Tests should assert remote calls and persisted facts as well as UI text. A test using real credentials is not required for any destructive or mutating scenario.

| Area | Scenario | Observable pass condition |
| --- | --- | --- |
| Discovery | Failed MAL sync with successful Plex processing | Needs mapping/Failed is visible; the event is never labelled Synced solely from the Plex success flag |
| Discovery | Search, pagination, return from detail | Search finds title/source ID; attempts do not duplicate rows; returning preserves list context |
| Detail | Existing successful event | Target and result remain inspectable; correction is possible through a new explicit preview |
| History | Legacy event with stored GUID, media type, source season/episode | Original facts are shown; current evaluation is labelled separately; the mapped anime-update episode is never reused as source input |
| History | Required original input is missing | Replay is disabled with the missing field/capability named; no guessed target or hidden Plex lookup |
| Matching | Beerus with corrected catalogue | AniDB 19859 S1E2 resolves to MAL 63367 E2 automatically |
| Matching | Gantz and mixed TV/ONA fixtures | AniDB 2400 S1 → MAL 384; TVDB 78916 S2 → MAL 395; 19908 → 63509; 20040 → 64131; 19889 → 63468; 13514 → 36552 |
| Matching | Direct MAL, movie, Tautulli and Plex-agent input | Supported paths preserve existing behavior; unavailable extraction produces a specific explanation rather than a guessed source ID |
| Matching | Split cour/range offset/explicit/skip rules | Actual arithmetic is shown; equally applicable conflicts, undefined explicit episodes, nonpositive or out-of-bounds conversions cannot write |
| Sources | Refresh fails or provider is unavailable | Existing valid data remains active; stale/unavailable is distinct from missing mapping; refresh never writes to MAL |
| Preview | Preview twice or change a draft | Zero MAL update calls, sync notifications or historical result rewrites; obsolete responses cannot replace the current draft |
| Preview | MAL title/state unavailable | Resolution can be displayed, but no apply-ready token is issued without the required remote state |
| Preview | Token expired or target/rule/account/relevant MAL state changes | Apply returns a clear conflict; a new preview is required; an OAuth token refresh for the same account alone does not redirect the job |
| Correction | Event-only correction | Selected MAL ID/episode are used once; persistent rules and future events remain unchanged |
| Rules | Save movie or season-range rule | Scope, range conversion, affected historical events and conflicts are visible before save; matching future events use the versioned rule |
| Rules | Existing custom YAML | Existing provider-base selection is preserved; local rules layer on top and never rewrite the YAML file |
| Rules | Concurrent edit or overlapping active scope | Revision conflict/overlap is rejected explicitly; no silent last-writer-wins result |
| Execution | Double-click or repeat the same request | One durable action, one logical completion; no second external update or success notification |
| Execution | Live episode 8 overlaps replay episode 6 | Shared writer prevents progress regressing to 6 in the same known watch cycle |
| Execution | Replay completion/old rating | No extra rewatch increment; old activity does not silently overwrite newer progress/rating |
| Execution | Previously wrong successful target | Both destinations are visible; apply changes only the reviewed destination and does not undo the old entry |
| Execution | Crash before send | Durable queued work can resume without needing the browser to remain open |
| Execution | Timeout/crash after send or failed local result storage | Reconcile the persisted absolute intent against remote state before any resend; uncertainty requires review |
| Execution | Already satisfied intent | Record No change needed without an external write or duplicate notification |
| Lifecycle | Delete event with queued/in-flight work | Queued work cannot execute afterward; in-flight deletion is blocked until safe |
| Security | Webhook API key attempts inspector write | Denied; operator session and CSRF protection govern inspector actions |
| Privacy | Trace/error/input snapshot | No Plex tokens, OAuth secrets, API keys, cookies, file paths, or player IP addresses are persisted/exposed |
| Migration | Fresh install and production-like v0.6.5 database copy | Existing settings, MAL auth and history are preserved; migration does not enqueue or replay historical events |
| Deployment | Copied live database starts on dev | No automatic MAL writes or production webhook ingestion; UI clearly identifies the dev environment and write policy |
| Compatibility | Existing screens and subfolder installation | Dashboard/history deep links work; configured base path is respected; existing webhook integration remains compatible |
| Accessibility | Keyboard and narrow viewport | Inputs/actions have accessible names, focus is visible, pending/errors are announced, the detail flow works without a wide table |

## API and UI handoff

Use the existing plan's `/api/sync` route family. Endpoints may differ internally if the frontend and backend agree, but the contract must preserve:

- Event summaries with stable `id`, display title, source provider/ID, source season/episode, actual MAL outcome, latest attempt time, and next-action capability.
- Detail with immutable known input, explicit legacy/historical-trace capability, original results, ordered attempts, and current-evaluation data in a separate structure.
- Preview with a typed reason, resolved target and episode, trace/provenance, complete field diff, expiry, and opaque approval ID only when safe to apply.
- Replay with a client idempotency key, durable job identity, and polling only while active. Return actionable conflicts instead of generic failures.
- Rules with explicit source/season/episode/context scope, revision, conflicts and impact; MAL-ID validation/search separate from saving.
- Refresh with last-checked/updated facts and a result the UI can describe honestly. A failed refresh must not erase the active catalogue.

Use familiar text: **Needs mapping**, **Needs sign-in**, **Needs review**, **Synced**, **No change needed**, **Queued**, **Processing**, and **Failed**. Preserve a successful original result even when a later correction fails. Disabled buttons must explain what is needed next.

## Development deployment acceptance

The private deployment report should give the develop image digest/commit, dev URL, service name, backup/rollback location, isolation controls and verification results. Use a consistent SQLite backup rather than a live database file copy. Preserve the production container, database, reverse-proxy routes, certificate renewal and settings. Personal deployment details and credentials belong outside the repository.

Review the copied Beerus history through the deployed UI, and use fake-MAL tests for apply/replay semantics. Do not enable a production-account write merely to demonstrate deployment. Document any acceptance gap as a gap; a reachable inspector alone is not the completed feature.

## Implementation contract review

Initial API/design review identified these concrete release gates for engineering. They are requirements to verify, not claims that implementation is complete.

| Priority | Gate | Required evidence |
| --- | --- | --- |
| P0 | One durable writer owns live and manual sync | Fake-provider test submits both concurrently and proves no bypass writer; disabling dev writes blocks both paths |
| P0 | Deletion is coordinated with work | Existing single/bulk Plex-delete routes cancel unsent jobs and serialize with or reject deletion during a possible remote write; restart cannot resurrect deleted work |
| P0 | Legacy recovery uses real original inputs | Copied Beerus detail can preview AniDB 19859 S1E2; converted anime-update numbers are never a reconstruction source |
| P0 | Approval and uncertain outcomes are safe | Tests cover stale account/state/rules, preview expiry, duplicate submit, crash before send and uncertain send reconciliation |
| P0 | Scoped local rules are reviewable and maintainable | Scope/conflict/affected-event preview before saving; revision-checked edit, disable and re-enable; an absent server identity does not silently grant global scope |
| P1 | Existing users can discover the inspector | Inspect links from Plex Payloads, Anime Updates and dashboard recent activity resolve to original Plex event ID |
| P1 | Historic outcome and current attempt stay distinct | Original success plus failed correction is displayed honestly; a fresh preview cannot overwrite saved historical explanation |

The initial API draft's create/deactivate-only rule lifecycle and lack of an impact-preview response were gaps against the approved first-release plan. The current implementation adds impact fingerprints, revision-checked edit/re-enable/disable, and an explicit all-server scope choice for legacy events. Endpoint names may change without changing the underlying product behavior.

### Source audit after implementation handoff

This is a source review, not deployed acceptance or a statement that all tests passed:

- Live intake and replays now use a shared durable coordinator. Deletion excludes the writer while source rows and queued work are removed. Original Plex season/episode fields are reconstructed separately from processed anime-update episode values.
- UI-created rule impact, revision lifecycle, and explicit legacy server-scope choice are implemented. Browser validation remains required.
- Historical attempt previews contain **planned** before/after values. The UI now labels these “MAL at preview” and “Planned change” and explains that they are not a fresh read. A separate observed-result card requires persisted provider evidence and an observation time.
- The initial planned-versus-observed outcome gap is corrected in source. Attempts retain actual observed state separately. The production adapter confirms an acknowledged update with a complete MAL read through the same bound client; a failed confirmation leaves the outcome uncertain without invented after-values. Compatible failed history uses the actual preflight state rather than proposed progress, score or dates.
- No-change and recovered outcomes now suppress success publication, and repeated terminal completion is idempotent. These paths have targeted backend tests; delivery of a remote notification is still not an exactly-once transaction.
- Current Plex user/library eligibility is now rechecked during preview, admission and execution, including before a possible send. A known captured server must match the configured server when API verification is enabled. Disabled API/HAMA use remains supported with an explicit warning that server identity was not compared.
- New inspector input/error/provenance structures are allowlisted and legacy error text is sanitized. No raw credentials were found in this source review. Deployment identity text must stay generic and must not reveal private account or infrastructure details.

### Local browser QA — 2026-10-20

Chrome access is now working. An authenticated walkthrough used the isolated copied database, with MAL writes and token refresh disabled, and the backend rebuilt from the feature revision.

- Beerus search by AniDB ID found exactly its two failed events. Detail preserved the original season/episode and missing-catalogue result, separately from the new canonical AniDB evaluation resolving S1E2 to MAL 63367 E2.
- Catalogue refresh invalidated the previous approval. Numeric MAL lookup, explicit selection and an event-only correction produced a separately labelled preview; Apply remained disabled with its reason visible.
- Legacy local-rule creation required explicit server scope and impact review. The actual impact identified the two matching stored events. Save, revision edit, disable and reviewed re-enable worked; the temporary QA rule was disabled afterward.
- QA found that the saved-rule editor discarded changes immediately. The editor now asks for confirmation, offers Keep editing, and preserves the saved revision when edits are discarded. Desktop and mobile browser coverage verifies both choices and the subsequent revision lifecycle.
- List pagination, filter preservation on return, provider filtering, empty-search recovery, keyboard activation, and Inspect links from both history screens were exercised. An existing successful event retained its historical success and returned No change needed from its current preview.
- At 390px, the actual list, detail, correction and rule editor stayed within the viewport. Screenshots and private database comparisons are retained outside Git. Original Plex and anime-update history stayed unchanged; no new sync jobs or attempts were created.

Validation passed: all 24 desktop/mobile browser fixtures, frontend production build and lint, full Go tagged integration, race checks for the affected backend packages, and Go vet. The browser fixtures cover explicit fake-provider apply, stale/expired previews, conflict and session errors, observed-versus-planned results, and disabled writes. PR CI for the initial feature revision passed, including browser journeys, integration, unit tests and image build; subsequent changes require their own CI result.

### Remaining release verification

- Complete the development deployment and review the final image through its deployed UI. Record the image revision/digest, migration, isolation controls, production health and compatible rollback in the private deployment report.
- This local walkthrough did not write to a real MAL account. Apply/replay behavior is exercised with fake-provider tests; the read-only copied-account walkthrough must not be represented as real-account write acceptance.
- Local QA does not claim full manual accessibility certification or a live subfolder deployment review. Keep personal deployment details, screenshots and credentials out of the public PR.
