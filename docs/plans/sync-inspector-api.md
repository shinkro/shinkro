# Sync Inspector API contract

All routes below are relative to `/api/sync`. JSON uses camelCase. Routes require the existing signed browser session (API keys are not accepted). GET `/capabilities` returns `{writesEnabled, csrfToken, previewTTLSeconds:300}`. For every POST/PUT/DELETE send `X-Shinkro-CSRF: <csrfToken>`. Dev deployments set `SHINKRO_SYNC_WRITES_ENABLED=false`; previews, mapping refresh and local rules remain available, but MAL apply is disabled server-side.

- GET `/events?limit=25&offset=0&search=&status=&source=&event=` returns `{data:Event[],count:number}`. Status values `synced`, `no_change`, `needs_mapping`, `needs_sign_in`, `needs_review`, `failed`, `queued`, `processing`, `received`, `cancelled`.
- GET `/events/{id}` returns `{event:Event,input:Input,attempts:Attempt[],legacy:boolean,replayEligible:boolean,replayReason:string,overrides:Override[]}`. ID is Plex payload ID.
- POST `/events/{id}/preview` body `{correction?:{malId:number,episode:number}}` returns `Preview`. No MAL writes. Errors in resolution/read appear as `ready:false`, `code`, `message`, and partial trace; HTTP 200 permits inspection.
- POST `/events/{id}/replays` body `{previewId:string}` returns `Job`; same preview deduplicates to the same job. HTTP 409 for stale/expired preview, 403 when writes disabled.
- GET `/jobs/{id}` returns `Job` (poll while queued/processing).
- GET `/mal/search?q=...` returns `{data:[{malId,title,pictureUrl,totalEpisodes,mediaType}]}`. Numeric input supports direct MAL lookup; other text searches local catalogue (therefore works offline; preview reads authoritative MAL state).
- POST `/sources/refresh` body `{}` refreshes catalogue and mapping cache; returns `{refreshedAt}`. HTTP 502 on provider failure; old validated map remains active.
- GET `/overrides` returns `{data:Override[]}`.
- POST `/overrides` body `{eventId:number,malId:number,sourceEpisodeStart:number,sourceEpisodeEnd:number,malEpisodeStart:number}`. Scope is derived from event source ID, provider, season, library, Plex account and available server identity. Movie range is 1–1. AniDB rules must be season 1. Returns saved `Override`; conflicting ranges rejected 409. Does not replay history.
- DELETE `/overrides/{id}` deactivates rule, preserving revision/history; returns 204.

`Event`: `{id,title,episodeTitle,account,library,event,mediaType,source,sourceId,season,episode,rating,receivedAt,status,errorCode,errorMessage,malId,malTitle,lastAction,legacy}`.

`Input`: `{version,eventId,title,episodeTitle,account,library,serverId,event,mediaType,source,sourceId,season,episode,rating,receivedAt}`. Original source numbers never change.

`Preview`: `{id,eventId,createdAt,expiresAt,ready,writesEnabled,code,message,resolution:Resolution,before?:MALState,after?:MALState,changes:Change[],warnings:string[],accountId:string}`.

`Resolution`: `{malId,episode,origin,title,steps:Step[],rule?:object,fingerprint,sources?:SourceSnapshot[]}`. Origin values `direct_mal`, `canonical_anidb`, `community_rule`, `custom_file`, `catalogue`, `local_override`, `event_correction`. `Step`: `{label,detail}`.

`SourceSnapshot`: `{kind,label,contentHash,entries,observedAt,lastRefreshAt?}`. `observedAt` is the first local observation of that immutable content generation, not proof of upstream freshness. `lastRefreshAt` is present only after a successful explicit source refresh. Saved traces retain their source metadata. Changes to unrelated catalogue entries or freshness timestamps do not invalidate an otherwise identical approved decision.

`MALState`: `{malId,title,pictureUrl,totalEpisodes,status,episodesWatched,score,isRewatching,rewatchCount,startDate,finishDate,updatedAt}`. `Change`: `{field,before:unknown,after:unknown}`. Preview preserves all unrelated fields. Old rating events are blocked; progress never decreases or creates an inferred rewatch.

`Attempt`: `{id,jobId,kind,status,createdAt,finishedAt,preview?:Preview,observed?:MALState,observedAt?:string,observationSource?:string,code,message}`. Historical legacy attempts may have no preview; original history remains separate from fresh preview.

`Job`: `{id,eventId,kind,status,createdAt,updatedAt,code,message,previewId}`. An uncertain remote write is `needs_review`, never blindly resubmitted.

`Override`: `{id,revision,eventId,source,sourceId,season,library,account,serverId,mediaType,sourceEpisodeStart,sourceEpisodeEnd,malEpisodeStart,malId,active,createdAt}`.

Errors use `{code,message}`. All arrays are JSON arrays, never null. No credentials/raw payloads are exposed.

Implemented additions:

- Detail includes `original: Event`, preserving the original result and destination independently of the current/latest action.
- Authenticated mutations require a signed session carrying the operator's `user_id`; users with an older session must sign in again. Previews, jobs and rule revisions record `actorId`.
- POST `/overrides/preview` validates the MAL destination and known episode bounds, derives the exact scope and returns `{scope,affectedEventIds,conflicts,ready,warnings,fingerprint}`. Saving an active rule requires `impactFingerprint` copied from this reviewed result; a changed scope or destination is rejected.
- GET `/overrides` includes inactive rules. PUT `/overrides/{id}` edits or re-enables a rule with the current `revision`, `active`, and reviewed `impactFingerprint`. DELETE `/overrides/{id}?revision=N` deactivates a rule with revision checking. Every revision is retained. Disabling a rule requires revision confirmation but does not require MAL connectivity.
- `SHINKRO_MAL_TOKEN_REFRESH_ENABLED=false` uses the current access token without refreshing it. This prevents a development copy from rotating a shared production refresh token. Expired tokens require a separate development sign-in.
- Same-event queued/processing submissions return the existing job only when the approved intent is equivalent, including an equivalent fresh preview. A different intent returns `409 JOB_IN_PROGRESS`. A replay that starts later revalidates its approved preview before sending.
- New live activity has a separate rewatch policy: a fresh episode-one event may start a rewatch, and a fresh completion may finish an already active rewatch with a persisted absolute count. Historical replay never infers a new watch cycle.

Operational safeguards:

- Distinct Plex deliveries with the same server/account/library/provider/item episode or rating identity and MAL account are suppressed for 30 seconds. A duplicate retains its own event and explanation, and does not extend the original window. The same item may sync normally after the window.
- Old activity cannot advance a newer active rewatch cycle. `STALE_WATCH_CYCLE` requires a new Plex activity event to establish current-cycle intent.
- Unused previews are pruned after 24 hours; all job-referenced intents remain available for audit and uncertain-write reconciliation. Event inputs, attempts, jobs and local rule revisions are retained until explicitly deleted; automatic age-based history pruning is a follow-up.
- MAL reads, account verification and writes are each bounded to 15 seconds. A transport timeout after an update is treated as uncertain and reconciled before another attempt.

Legacy server scope:

- `Override` includes `anyServer: boolean`, defaulting to false. A known event server remains the default exact scope.
- When the original server identity is unavailable, impact returns `ready:false`, an empty fingerprint, and a warning until the operator explicitly chooses `anyServer:true`. Active save/edit otherwise returns `SCOPE_REQUIRED`.
- An explicitly reviewed all-server rule still matches only the displayed Plex account, library, source provider/ID, media type, season and episode range. It can therefore fix future events with captured server identities without guessing a server for the old event.
- All-server and specific-server rules with intersecting ranges conflict symmetrically. Changing the server scope invalidates the prior impact fingerprint. Event-only corrections require no server-scope choice.


Observed outcomes and current eligibility:

- `Preview.before` and `Preview.after` remain the historical read and **planned** changes. They are never rewritten to pretend they are the remote outcome.
- An attempt's optional `observed` stores actual remote list state, with a local response observation time. `confirmation_read` means an authoritative MAL read after an acknowledged update; `reconciliation` means a later read recovered an uncertain outcome; `no_change_read` means a read showed no update was needed. Complete custom/test adapter responses may use `write_response`.
- The production adapter confirms a write using a complete MAL read through the same account-bound client. If that read fails, the job needs review and contains no invented observed after-state. Compatible history uses the observed result, or the last actual preflight read when no after-state is known; failed proposed progress, dates or ratings are never materialized as applied.
- No-op, duplicate-source and reconciled outcomes emit no success notification. Repeated terminal completion is idempotent: it adds no second attempt, compatible history row or notification. Direct confirmed updates may announce one logical completion; remote notification delivery is not an exactly-once transaction.
- Preview, queue admission and execution recheck the current Plex user and enabled anime libraries, including immediately before a possible MAL send. Excluded events return `EVENT_NOT_ELIGIBLE`; unavailable settings return `ELIGIBILITY_UNAVAILABLE`. Captured input remains immutable.
- Where the original server ID is known and a Plex API server is enabled/configured, `/identity` must match it. A disabled/unconfigured Plex API remains compatible with HAMA: user/library checks still apply and the preview explicitly warns that server identity was not checked. An absent historical server ID is never replaced by today's server.
- Plex extraction failures appear as Failed with sanitized preparation errors and an explicit statement that no MAL update was attempted. They are not labelled as Received or a failed MAL write.
