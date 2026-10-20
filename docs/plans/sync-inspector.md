# Sync Inspector implementation plan

**Status: implementation authorized; development verification in progress.** This document records the product and engineering contract, based on Shinkro v0.6.5 / develop commit f5c96be76bd7b6b421fdf9db52c97104bda5f95e. The [acceptance checklist](sync-inspector-acceptance.md) records release gates. Personal deployment details and credentials are kept outside the repository.

## 1. Proposed first release

A user can open a Plex event, understand its MAL destination and episode conversion, preview a correction, save an optional local rule, and safely reprocess that event. A successful sync is inspectable too: the inspector must help catch a wrong match that did not produce an error.

The first release includes:

- An event list and detailed inspector, linked from existing history screens.
- The decision recorded during processing, alongside a separately labelled evaluation using today's data.
- A preview of the exact proposed MAL field changes, with no MAL list writes.
- Manual retry and correction through a durable executor shared with normal live sync.
- Simple local mapping editors: movie → MAL, and a source season/episode range → MAL episode range.
- Versioned local rules layered over the selected mapping source, with scope and conflicts visible.

Follow-up releases add automatic retry of failed work, advanced mapping editing, and community contribution drafts. Plex library backfill, a public mapping website, PostgreSQL, additional tracker integrations, and multiple MAL accounts are separate projects.

**Decisions for your review:**

| Decision | Recommended default | Tradeoff |
| --- | --- | --- |
| First-release scope | Ship the complete inspect → preview → fix → retry flow, in small implementation PRs | More work than a details screen, but solves the actual support problem |
| Local corrections | Offer “this event only” and “save a rule”; default to this event only | Recurring rules require explicit scope selection |
| Existing custom YAML | Preserve its existing role as the selected provider base; add UI rules on top | Community maps remain the base for users without a custom file; migrating existing files to additive rules is optional later |
| Replaying old activity | Block automatic progress decreases, stale ratings, and inferred new rewatches; require a fresh explicit preview for intentional changes | Some old events need review rather than a one-click retry |
| Correcting a prior successful match | Show and apply the corrected destination; leave the previous MAL entry unchanged | Automatic undo could erase legitimate later activity on that entry |
| Automatic failed-job retries | Follow up after durable manual recovery is validated | The first release still resumes safely queued work after restart |

Approval here means agreeing on scope and behavior before implementation. The acceptance scenarios in section 10 define what must work.

## 2. User experience

### Entry points and event list

Add a “Sync Inspector” navigation item and an Inspect action to Plex Payloads, Anime Updates, and the dashboard's recent activity. Deep links use the stored Plex event ID, so the same event has one stable inspector URL.

The list has one row per received event, with the Plex title/user, source episode or rating, MAL target when known, last action, and current result. Filters cover action needed, failure reason, source provider, event type, title/ID, and time. Attempts and corrections appear inside that event, rather than multiplying list rows. If a correction fails after the original sync succeeded, show both facts.

Use plain result labels: Queued, Processing, Synced, No change needed, Needs mapping, Needs sign-in, Needs review, Failed, and Cancelled. Plex extraction success alone must not be labelled “Synced to MAL.”

### Inspector detail

Present four sections:

| Section | What the user sees |
| --- | --- |
| Received | Original Plex title, source/provider IDs, account/library, source season/episode or rating, received time, and source event time when actually supplied |
| Match | Resolution steps, selected MAL title/link, selected rule and its origin, episode arithmetic, alternatives/rejected candidates, and a useful reason if blocked |
| MAL change | Before → proposed/observed after values for every field touched: progress, status, score, rewatch flags/count, and start/finish dates |
| Attempts | Original result and each later action, with timing, sanitized errors, retry eligibility, and links between related jobs |

Use badges such as Direct MAL, Canonical AniDB, Community rule, Custom file, Local override, Ambiguous, and Source unavailable. Do not invent a confidence percentage.

“What happened” comes from a saved trace. “Preview now” creates a new evaluation against current mappings and current MAL state. It must never rewrite the historical explanation.

Source refresh and MAL lookup are separate: a preview can explain a cached match even when the provider is temporarily unavailable, while clearly withholding an apply-ready preview when required MAL state is missing.

For example:

    Plex: Example Show S02E13
      → TVDB series 123, season 2, episode 13
      → Community rule: source episodes 13–24 map to MAL 456, starting at episode 1
      → MAL 456 episode 1
      → Current progress 0; proposed progress 1 and status Watching

The example uses fictional IDs. The real Gantz fixtures remain part of the regression suite.

### Fix and preview

“Use a different MAL entry” opens a searchable title picker with MAL IDs, type, episode count, and release information. Entering a MAL ID is also supported. Selection is explicit; search results never silently replace the resolver's decision.

For episodes, ask for source season, source episode start/end, and MAL episode start. Show an episode conversion table before saving. Movie rules specify the source ID and MAL title. Existing explicit/skip rules can be inspected and previewed; their advanced editor is a later increment.

Offer two distinct actions:

- **Apply to this event:** an event-scoped correction, with no persistent mapping change.
- **Save a rule:** show the exact server/library/provider/season/episode scope, affected stored events, and conflicts. Saving changes future matching; reprocessing existing events is a separate explicit action.

The final action names the destination and effect, for example “Sync MAL 456 to episode 1.” If MAL already has the intended state, show “No change needed.” If a previous attempt updated another title, explicitly state that its MAL entry will not be reverted.

Network failures, missing credentials, expired previews, and incomplete legacy inputs each have their own explanation and next action. Disabled controls include a reason. Preserve list filters and scroll position when returning from detail; support keyboard navigation and narrow screens.

## 3. Repository findings that affect the design

| Current implementation | Consequence for this feature |
| --- | --- |
| [animeupdate service](../../internal/animeupdate/service.go) resolves IDs, mutates episode numbers, fetches MAL state, writes MAL, and stores history in one path | Extract a shared resolver and change planner; preview cannot call the current write method |
| [event subscribers](../../internal/events/subscribers.go) launch MAL updates in a goroutine | A persisted queue and one authoritative writer are required before enabling manual replay |
| [Plex repository](../../internal/database/plex.go) persists a subset of the payload, including media type, GUID, and original season/episode | New events need a versioned input projection; verify reconstruction against stored columns; older events need capability checks |
| Parent GUID, server identity, rating keys, and original live Plex metadata results are not all retained | Historical resolution cannot always be reproduced honestly from current records |
| [anime_update storage](../../internal/database/animeupdate.go) stores derived episode/result data; GetByID is unimplemented | Original Plex fields are the replay input; processed episode values must never be mapped twice; add explicit history queries |
| [mapping service](../../internal/mapping/service.go) uses a per-provider custom file instead of community data when selected, with an in-memory cache | Preserve existing base selection, add a separate override layer, and capture mapping provenance/content hashes |
| [domain mapping](../../internal/domain/mapping.go) chooses among candidate rules and falls back for missing explicit episodes | Preserve valid split-season ordering; separately review fail-closed handling for conflicting rules and missing explicit keys |
| [watch-status planning](../../internal/domain/animeupdate.go) uses the current clock and can increment rewatch counts for completed entries | Build deterministic field intents; a replay must not become another rewatch completion |
| [database migrations](../../internal/database/database.go) reject schemas newer than the binary understands | Rollback after a schema migration needs a compatible binary; a feature flag does not make v0.6.5 understand the new schema |
| [UI history details](../../web/src/components/table/ViewDetailsModal.tsx) currently show JSON; write routes for replay/overrides do not exist | Reuse existing tables, routing, API client, and query cache, while adding focused inspector screens |

The current app has one configured Plex-user/MAL connection. Keep that model. Record a stable MAL account binding so queued activity cannot be redirected to a different account after reconnection.

## 4. Shared processing architecture

    Capture immutable event input
      → Select consistent catalogue/mapping snapshots
      → Resolve target and episode, with a decision trace
      → Read current MAL state
      → Build an explicit field-change intent
      → Preview, or persist/validate/execute the intent
      → Record observed outcome and notify the existing UI

Proposed boundaries:

- **Input capture:** a versioned, allowlisted projection of the incoming event, original season/episode/rating, agent identifiers, server/library/item identity when supplied, received time, and destination binding. Retain extracted provider metadata as a separate captured observation.
- **Resolver:** returns a target, converted episode, typed outcome, candidates, and trace. It never writes MAL, updates history, or publishes events. Plex metadata reads belong to an adapter and return a captured result with its observation time.
- **Snapshot registry:** supplies immutable, versioned catalogue and map data. Persist a content hash and retrieval time for community/custom maps and the catalogue generation. Record the actual selected rule in each trace. A snapshot must remain consistent during a resolution. “Current mapping” means the active validated snapshot, with its observation time visible; an explicit source refresh checks for newer data without silently changing a historical trace.
- **Change planner:** computes a typed, explicit patch from input, resolution, MAL state, policy, and an injected clock. It is independent of SDK option serialization. Preview and execution use this same planner.
- **Executor:** owns MAL list writes, validates approval/freshness/order, persists intent before sending, records outcome, and materializes compatible history.

Preserve the default resolver paths: direct MAL IDs; direct canonical AniDB for season one; AniDB-to-TVDB conversion and season/episode rules for later seasons; existing movie and season-one fallback behavior where unambiguous. These defaults continue to work without a custom mapping.

An explicitly saved local override may precede the automatic route only within its exact scope, and the trace must say so. AniDB overrides are restricted to an explicit season-one scope in the initial editor. Direct MAL inputs remain direct by default; a deliberate event-only correction can override one event. Personal AniDB rules are not automatically exportable as community TVDB rules.

Treat “not found,” “ambiguous,” “source unavailable,” and “invalid rule/input” as different outcomes. Cache/download failures must not masquerade as a missing rule and silently trigger a different match. Preserve a last-known-good validated map, show its age, and fail clearly when no valid snapshot is available.

Characterize current behavior first. The following intentional safety changes need their own fixtures and review: conflicting equally applicable rules block a write; a missing explicit episode does not silently become a range mapping; invalid episode conversions block a write; duplicate completion cannot increment rewatch count again. Valid split-cour rules with different episode starts remain deterministic.

## 5. Persistence and legacy history

Keep SQLite and the current single-instance deployment. No external queue or database service is needed for this scope.

The logical records below are a schema proposal; PR 1 should finalize column names and indexes:

| Record | Purpose |
| --- | --- |
| Event input snapshot, linked to plex_payload | Original normalized inputs and capture version; preserve original versus observation/attempt timestamps |
| Sync job | Live/replay/correction action, original event/parent action, destination account binding, state, intent identity, ordering information, claim/lease, and next eligible time |
| Sync attempt | Append-only finalized decision trace, snapshot references, proposed patch, before/observed-after MAL fields, safe provider status, and outcome |
| Preview | Opaque server-side preview ID, event/draft, destination binding, relevant revision hashes, expected MAL fields, expiry, and actor |
| Mapping override revisions | Immutable versions of a scoped rule, with enable/disable/delete history and optimistic revision checks |
| Snapshot/destination metadata | Catalogue/map content revisions and the stable configured MAL account identity; no copied OAuth secrets |

Index event chronology, job state/eligibility, event-to-job linkage, attempt order, destination+target ordering, and active override lookup. Constrain a single active equivalent replay; persist an idempotency key for each UI submission. Legacy history adapters must select the latest attempt deterministically by time and ID, avoiding duplicate join rows or an older attempt winning accidentally.

Store each accepted live event and its initial job atomically before acknowledging acceptance on the new execution path. Preserve the existing webhook response contract unless a separately documented compatibility change is necessary. The old subscriber must stop issuing writes before the new worker becomes authoritative.

Keep SQLite write transactions short: intake, claim, and finalization are separate transactions; never hold a database write transaction while waiting on Plex or MAL.

A job's approved intent is immutable. Retries append attempts; a changed target or materially changed intent is a new linked correction action. A successful API write and its local result/history are recorded in one local transaction when possible; the external API cannot participate in that transaction.

For legacy events:

- Display existing facts and mark the historical trace unavailable; never reconstruct an old explanation from current mappings and label it original.
- Restore genuinely stored fields correctly. A fresh preview can use a validated reconstruction of original input, clearly labelled with current settings/data.
- Enable replay only if all required inputs can be recovered. Show exactly which fields are missing otherwise. Fetching today's Plex metadata is an explicit fresh observation, not recovery of historical truth.
- Never use the already converted anime_update episode as source input.
- Do not automatically enqueue legacy records during migration or guess that a missing result means no MAL write happened.

Use additive, transactional migrations, tested on both a fresh database and a v0.6.5 fixture. Preserve existing settings and history, handle legacy unlinked rows without silently deleting them, and verify foreign-key behavior on every pooled connection before relying on cascading deletes. Deleting an event must cancel pending work and remove associated details once quiescent; reject deletion during an in-flight write. A deleted event must never execute later in the background.

Retain compact event/action history with the existing history records. Proposed detailed-trace retention is 90 days, configurable; never prune active jobs or data needed to reconcile an uncertain write. Expire unused previews promptly. Document retention and deletion behavior in the UI.

## 6. Replay and execution rules

### Fresh preview and account binding

A preview is generated server-side, expires after a proposed five minutes, and contains the complete relevant diff. Preserve existing MAL dates unless a deliberate reviewed change is required. Newly proposed watch dates use a trustworthy source event date, or the labelled original received date as fallback, in the configured timezone; pin those values in the intent rather than replacing them with the retry date. It performs no MAL list writes and sends no sync notifications. Token refresh may still occur through the existing authentication service.

When applying, the worker rechecks the configured account, event eligibility, rule/catalogue revisions, relevant MAL fields, and the preview expiry. A change to the target, episode conversion, intended fields, account, or relevant remote state requires a new preview. Revision changes trigger revalidation; an unrelated mapping change alone need not invalidate an otherwise identical approved intent. A merely refreshed OAuth token for the same account does not create a new destination. Recheck the configured Plex user/library eligibility as well. Any fresh Plex lookup must use the captured server/item binding; never send an old item key to a newly configured server silently.

Current state must be fetched just before a write. This reduces races with direct MAL edits; it does not promise an atomic compare-and-set against another external client. Confirm the provider's current conditional/idempotency capabilities during the initial API spike rather than assuming support.

### Ordering, duplicates, and rewatches

Start with one writer per configured MAL destination. Live events and manual actions go through the same coordinator. Do not permit the original goroutine writer to race a replay worker.

- Scope duplicate detection to the source server/user/item and event identity. Prefer a provider event ID/time when supplied; use a documented bounded duplicate window otherwise. Do not deduplicate every future watch of the same episode forever.
- Preserve source event time where present, plus received time and stable receipt sequence. If source order is unknown, label it rather than treating receipt time as proof of playback order.
- Within a known watch cycle, older progress cannot move progress backwards. A replay cannot infer the start of a new rewatch from an old low episode number.
- Completing a rewatch changes its count once for the logical completion. Persist the intended absolute count; never recalculate “current count + 1” on each attempt.
- Rating and progress actions have separate ordering rules. An old rating replay cannot overwrite a newer known rating. If remote edits or watch-cycle identity make the decision uncertain, require review.
- Intentional progress resets or a new rewatch require a separate, clearly named, freshly previewed action; the normal Retry button cannot authorize them implicitly.
- Retrying an already satisfied intent is a recorded no-op, without another MAL write or duplicate success notification.

### Failure and crash handling

Record an intent before sending a write. Use bounded network timeouts and recoverable job claims. On restart, unstarted queued work can resume; a job that may have sent a write must be reconciled first.

| Condition | First-release behavior |
| --- | --- |
| Missing/ambiguous mapping or invalid episode | Needs mapping/review; no write |
| Credentials unavailable or bound account changed | Needs sign-in/review; no rerouting to another account |
| Read timeout, 429, or provider failure before a write | Record retryable failure and any provider cooldown; expose a safe manual retry |
| Write timeout, lost response, or crash after sending | Read remote state; if the pinned intended state is observed, record reconciliation; conflicting/uncertain state needs review |
| MAL accepted a write but local history persistence failed | Keep the action recoverable; reconcile instead of issuing a newly calculated update |
| Already satisfied or superseded action | No-op/superseded, with the reason visible |

Exactly-once remote delivery is not a guarantee. Persisted identities, explicit absolute patches, serialization, read-back, and conservative handling of uncertainty prevent blind repeated writes. Process leases protect local state; they cannot fence an already in-flight request at MAL.

Existing notifications are emitted from committed logical outcomes, with local duplicate suppression. The inspector's database records are authoritative even if notification or UI event delivery fails. Provider response headers/status must be retained through a small adapter instead of discarded by the service.

## 7. Local mapping rules

New UI-created rules are additive. Precedence is an applicable operator override, then the existing automatic route and selected mapping base. Without an override, the season-one AniDB behavior stays unchanged. Explicitly overlapping active rules for the same source/season/episode scope are rejected rather than ordered silently.

Scopes use stable server/library/provider identifiers where available; titles are display labels, not identity keys. Require an explicit scope if old metadata is insufficient. Source ordering is recorded when known; unknown ordering is labelled. DVD/absolute-order conversion is not newly introduced in this release.

For TVDB season ranges, support split cours by non-overlapping source episode ranges with separate MAL targets. A missing end bound must be explicit and visible. Validate positive destination episodes and MAL episode limits when known. Saving a rule shows the affected stored events without modifying them automatically.

Keep custom-file imports compatible. New UI rules do not rewrite the user's YAML files, and disabling a UI rule reveals the existing base behavior. Every applied trace includes the exact rule version. Updating/removing a rule requires its current revision, preventing a second tab from overwriting a newer edit.

## 8. Proposed API and frontend surface

Route names are provisional; preserve the configured base-path behavior.

| Endpoint | Purpose |
| --- | --- |
| GET /api/sync/events | Paginated, filtered event summaries |
| GET /api/sync/events/{id} | Captured input, original traces, action/attempt history, and supported actions |
| POST /api/sync/events/{id}/preview | Fresh evaluation, optional draft correction, exact field diff, and expiring preview ID |
| POST /api/sync/events/{id}/replays | Submit the approved preview plus idempotency key; return 202 and a durable job ID, or the existing job for a repeated submission |
| GET /api/sync/jobs/{id} | Job state, outcome, and retry/review requirements |
| POST /api/sync/sources/refresh | Refresh validated local source snapshots; retain last-known-good data on failure; no MAL list write |
| GET /api/sync/anime/search | Bounded/debounced MAL search and direct-ID validation through the configured client |
| POST /api/sync/overrides/preview | Validate rule scope, conflicts, and effect on the selected/relevant stored events |
| GET/POST/PATCH/DELETE /api/sync/overrides | List/create/version/disable rules with revision checks |

Return structured reason codes and human-readable messages. Relevant conflicts return 409 with a reason such as preview expired, remote state changed, rule changed, or already processing. MAL outage during preview can still expose the resolution trace, but no apply-ready token is issued without the required remote state.

New inspector actions require an authenticated operator session, a recorded actor, and CSRF protection. A webhook API key must not automatically gain inspector-write access through the current broad authentication middleware. Existing webhook authentication remains compatible. Future scoped automation access can be added deliberately.

Reuse React, Mantine, TanStack Router/Query/Table, the API client, and existing source/status/link components. Add event detail routes, typed trace/preview/job models, a resolver-step view, MAL field diff, rule editor, and attempt timeline. Poll only while a job is active; do not poll MAL on every render. Cancel obsolete search/preview requests. A stale browser response must not replace a newer draft.

Capture only useful metadata. MAL snapshots also use an allowlist of relevant list fields; personal comments/tags are not needed for a sync diff. Never persist/export Plex tokens, OAuth secrets, API keys, session cookies, file paths, or player network addresses in inspector snapshots or errors. Community drafts omit personal account/server identifiers by default.

## 9. Implementation sequence and release gates

Each row is a reviewable implementation slice; split a large row into additional PRs if needed.

| PR | Work | Gate before proceeding |
| --- | --- | --- |
| 1 — Resolver and change planner | Characterization fixtures; typed immutable input/resolution/intent; extract shared planning; inject clock; MAL adapter spike; pin proposed behavior changes separately | Existing successful routing and episode conversion match fixtures; intentional safety differences are enumerated; planning performs zero MAL list writes |
| 2 — Capture and trace persistence | Add migrations, input snapshots, map/catalogue generations, historical traces, legacy capability assessment, and deterministic history queries | Fresh/upgrade migrations preserve existing records; stored source episodes remain immutable; historical and fresh evaluations cannot be confused |
| 3 — Read-only inspector | Event/detail APIs, preview API, list/detail UI, old-screen deep links, target/rule provenance, field diff, error actions | End-to-end read-only walkthrough works with fake Plex/MAL; repeated preview never writes a list or changes processing history |
| 4 — Durable execution and manual retry | Atomic intake/job creation, one writer/coordinator, request deduplication, account binding, intent persistence, crash reconciliation, manual action API/UI | Prove live/replay concurrency, duplicate submissions, stale approval, restart, and ambiguous-write scenarios before enabling repair |
| 5 — Corrections and local rules | Event-only correction; movie/season-range editor; additive override revisions; impact preview; conflict handling; successful-event correction flow | Ordinary season-one AniDB remains automatic; scoped fixes work without replacing community maps; previewed and executed targets agree |
| 6 — Integration and rollout | Full UI/error-state testing, compatibility/retention/deletion checks, operator docs, staging rehearsal, and controlled production enablement | All acceptance scenarios pass; backup and compatible rollback procedure is rehearsed; repair enablement is reviewed |

The durable executor and replay policy are the largest correctness risk; local rule editing is the next largest slice. Do not treat this as a modal-only frontend change.

The first read-only inspector can be reviewed in staging after PR 3. The first complete product release is after PR 6. Automatic failed-job retry and community contribution are follow-ups, not hidden dependencies of the first release.

For automatic retries, later add error classification, bounded exponential backoff with jitter, provider Retry-After handling, attempt/elapsed-time limits, account-wide cooldown, and cancellation. Authentication and mapping failures remain blocked until corrected; uncertain write outcomes always reconcile before another write.

For contribution, later generate a previewable TVDB/TMDB patch from a representable rule, with validation fixtures and source context. Keep an export/copy path that needs no GitHub login. Submitting a PR is an explicit user action; a local save never publishes automatically. Personal numbering and AniDB-only corrections require a different report/export path when not representable in community schemas.

## 10. Acceptance and testing

Use fake Plex/MAL adapters and fixture data. Tests must assert remote calls and durable state, not just rendered labels.

| Scenario | Required result |
| --- | --- |
| Gantz AniDB 2400, season 1 | MAL 384 directly, without a custom rule |
| Gantz TVDB 78916, season 2 episode 1 | MAL 395 with the correct mapped episode |
| Main ONA versus mini-anime | AniDB 19908 → 63509, 20040 → 64131, 19889 → 63468; preserve Pikachin's 13514 → 36552 behavior |
| Direct MAL/movie/Tautulli/Plex-agent events | Correct target or explicit typed failure across supported inputs; record existing extraction gaps separately rather than inventing an ID |
| Split cour, range offset, skipped/explicit episodes, season zero | Explain the actual conversion; block undefined/conflicting or invalid results rather than guessing |
| Ratings, dates, and unknown episode totals | Exact score conversion/zero score are shown; unrelated MAL fields and existing dates are preserved; episode bounds use known totals without rejecting an ongoing unknown total |
| Repeated preview | Zero MAL list writes, sync-result mutations, or notifications |
| Correction saved on one season | Correct source-to-MAL episode table; unrelated seasons, shows, and libraries unchanged |
| Double-click/repeated HTTP submission | Same logical job; no duplicate external update |
| Live episode 8 and replayed episode 6 overlap | No regression to episode 6 in the same known watch cycle |
| Repeated completion/crash after a rewatch write | No second rewatch increment; reconcile the stored absolute intent |
| MAL state/rule/target/account changes after preview | Reject stale approval and request a new preview; never switch accounts silently |
| MAL write succeeds, response/local store fails | Recovery observes remote state and avoids blind reapplication |
| Legacy inputs lack parent GUID/server context | Show available facts; enable replay only when required input is recoverable; otherwise explain the missing capability |
| Original episode was converted before failure | Retry derives from original Plex episode, not the already mapped episode |
| Wrong title was previously updated successfully | Show both targets; correct the intended target only, with no automatic undo of the old title |
| Provider map download fails | Clear unavailable/stale-snapshot state; no false “not found” fallback |
| Event deleted or app restarted | No hidden action after deletion; safely queued work survives restart; uncertain sends reconcile first |
| Existing YAML, subfolder install, dashboard/history, notifications | Compatibility maintained; new overrides are distinguishable from the selected base |

Run focused domain/SQLite tests, provider-adapter fault injection, handler authorization tests, and worker race/restart tests. Run the existing Go unit/integration suites, targeted race detection, frontend type/build checks, and real browser journeys for inspect → edit → preview → apply, including stale-preview and narrow-screen states. A small browser-test setup will be needed; the current frontend has no test script.

The existing webhook integration race discovered during the previous release is a known baseline. Replace sleep-based asynchronous test coordination with deterministic completion signals where touched, and do not treat an existing race as evidence that new concurrency tests passed.

## 11. Rollout and operational limits

Begin on a production-like database copy and fixture services. Enable read-only tracing/inspection before manual repair. There must be no period with both old and new MAL writers active, and no automatic replay of the entire historical database.

Separate UI/repair enablement from worker ownership. Disabling repair stops new operator actions; it does not hand pending jobs back to the old goroutine path. A writer cutover or rollback must explicitly drain/cancel unsent jobs and reconcile possibly sent attempts.

Before schema migration, create and verify a consistent SQLite backup. After migration, the primary rollback is disabling new repair actions while running a schema-compatible binary, or deploying a compatible forward fix. The old v0.6.5 binary rejects a newer schema. Restoring its database backup after live traffic resumes could discard new history, so it is not a lossless rollback strategy. Rehearse this distinction before production enablement.

Observe queue age, queued/processing/needs-review counts, provider failures/cooldowns, duplicate/no-op counts, and uncertain-write recovery. Use compact operator-visible status and logs before adding a new metrics service. Rate-limit search/preview and bound payload/trace sizes and network waits.

Production acceptance is a small, explicitly chosen set of events, with the resulting MAL state and trace checked after apply. Avoid destructive progress resets or automatic bulk repair as a deployment test.

## 12. Review outcome to record

Implementation of the first-release scope is authorized: additive overrides, conservative replay/rewatch behavior, and leaving a previously wrong MAL entry unchanged. Automatic failed-job retries and community contribution remain follow-ups. A development release must pass the acceptance gates and live visual review before it is described as complete.

Useful implementation references: [current resolver/write flow](../../internal/animeupdate/service.go), [watch/rewatch policy](../../internal/domain/animeupdate.go), [current mapping selection](../../internal/mapping/service.go), [Plex input model](../../internal/domain/plex.go), and [current migrations](../../internal/database/migrate.go).

The pinned [MAL SDK list-state/update implementation](https://github.com/nstratos/go-myanimelist/blob/v0.9.5/mal/useranimelist.go) exposes the list fields needed for a complete diff, including rewatch flags/count and dates. Its [HTTP response/error wrapper](https://github.com/nstratos/go-myanimelist/blob/v0.9.5/mal/mal.go) exposes provider status and headers for classification. These are implementation references, not a claim of a provider-wide transaction or exactly-once guarantee; validate current provider semantics in the API spike.
