# Sync Inspector

Open **Sync Inspector** to see whether a received Plex event updated MyAnimeList. You can also select **Inspect** from Plex Payloads, Anime Updates, or recent dashboard activity.

## Understand a result

Filter events by outcome or search for a title or source ID. Open an event to see its original provider, season and episode, saved result, and later attempts. Successful Plex receipt means the webhook arrived; the separate sync result tells you what happened on MAL.

Older events may lack a historical decision trace. The inspector shows the original facts it can recover and labels unavailable history. A new preview evaluates the event using today's mapping data and MAL state.

The **Recorded result** preserves the original outcome. Saved attempts can be expanded to review their historical plan and, when available, a separate **Observed on MAL** result with its observation time. Under **How this match was found → Data used**, a local observation records when this instance saw a mapping snapshot; it does not establish how recently the upstream source was updated.

## Preview and correct an event

1. Select **Refresh mapping data** if the catalogue or community mapping may have changed.
2. Select **Preview now**. Check the MAL title, converted episode, proposed field changes, and any warnings. Previewing makes no MAL list changes.
3. If the destination is wrong, select **Choose a different MAL entry**. Search for a title or enter its numeric MAL ID, select the verified entry, check its target episode, and select **Preview correction**. This reads a fresh preview for **This event only**; it does not apply a MAL update.
4. When writes are enabled, explicitly apply the reviewed preview. The inspector records the new attempt and its result. Previews expire after five minutes; changed mapping, account or relevant MAL state requires another preview.

An already satisfied event needs no update. A correction to a different MAL entry does not undo changes previously made to the old entry. Old watch activity and ratings can require review to protect newer list progress.

## Save a local rule

For a repeated mapping problem, choose a local rule instead of an event-only correction. Review its account, library, server, provider, title, season, episode range and conversion, together with matching historical events and conflicts, before saving.

Some older events have no recorded server identity. Such events require an explicit **All servers in this library and Plex account** choice to create a rule for future events. Other scope limits still apply. Rules with known server identity default to that server.

Saving a rule does not retry old events or submit a community contribution. Use **Local rules** to edit, disable or re-enable rules, with a fresh impact review when required. Existing custom YAML mapping files remain available.

## Review a development copy safely

Set `SHINKRO_SYNC_WRITES_ENABLED=false` to disable both live and manual MAL updates on an isolated instance. Previews and local rules remain available. The interface displays the server's write policy.

If the instance uses copied MAL credentials, also set `SHINKRO_MAL_TOKEN_REFRESH_ENABLED=false` to prevent refresh-token rotation. Reads use the existing valid access token; an expired token requires separate sign-in. Keep the database, encryption key and other credentials private, and prevent production webhooks from reaching the copy.

Automatic retry policies, library backfill and community mapping submission are separate future features.
