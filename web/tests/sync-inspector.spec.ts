import { test, expect, type Locator, type Page } from "@playwright/test";
const timestamp = "2026-10-19T12:00:00Z";
const event = {
  id: 42,
  title: "Dragon Ball Super: Beerus",
  episodeTitle: "Episode 2",
  account: "sample-user",
  library: "Anime",
  event: "media.scrobble",
  mediaType: "episode",
  source: "anidb",
  sourceId: 19859,
  season: 1,
  episode: 2,
  rating: 0,
  receivedAt: timestamp,
  status: "needs_mapping",
  errorCode: "ANIME_NOT_IN_DB",
  errorMessage: "No MAL entry is linked to AniDB 19859.",
  malId: 0,
  malTitle: "",
  lastAction: "",
  legacy: true,
};
async function fixture(
  page: Page,
  options: {
    writes?: boolean;
    stale?: boolean;
    noChange?: boolean;
    expired?: boolean;
    serverMissing?: boolean;
    warnings?: string[];
    statusChange?: boolean;
    movie?: boolean;
  } = {},
) {
  page.on("pageerror", (error) => console.log("PAGE ERROR", error.message));
  const eventData = options.movie
    ? { ...event, mediaType: "movie", episode: 1 }
    : event;
  let refreshed = false;
  let applications = 0;
  let previewCount = 0;
  let savedRule: unknown;
  let lastPreview: Record<string, unknown> | undefined;
  const bodies: unknown[] = [];
  await page.addInitScript(() => {
    localStorage.setItem(
      "shinkro_user_auth",
      JSON.stringify({ username: "operator", isLoggedIn: true }),
    );
  });
  await page.route(/\/api\/[^/]+(?:\/|\?|$)/, async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const method = route.request().method();
    if (!path.startsWith("/api/")) return route.continue();
    const json = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    if (path === "/api/auth/validate") return json({});
    if (path === "/api/config")
      return json({ version: "develop", check_for_updates: false });
    if (path === "/api/updates/latest") return json({ tag_name: "v0.6.5" });
    if (path === "/api/sync/capabilities")
      return json({
        writesEnabled: options.writes ?? true,
        webhooksEnabled: false,
        csrfToken: "fixture-csrf",
        previewTTLSeconds: 300,
      });
    if (path === "/api/sync/events")
      return json({ data: [eventData], count: 1 });
    if (path === "/api/sync/events/42")
      return json({
        event: applications
          ? {
              ...eventData,
              status: "synced",
              errorCode: "",
              errorMessage: "",
              malId: 63367,
              malTitle: event.title,
              lastAction: "replay",
            }
          : eventData,
        original: eventData,
        input: {
          ...eventData,
          eventId: 42,
          version: 1,
          serverId: options.serverMissing ? "" : "fixture-server",
        },
        attempts: applications
          ? [
              {
                id: "attempt-1",
                jobId: "job-1",
                kind: "replay",
                status: "synced",
                createdAt: timestamp,
                finishedAt: timestamp,
                message: "MAL progress updated.",
                preview: lastPreview,
                observed: {
                  malId: 63367,
                  title: event.title,
                  totalEpisodes: 12,
                  episodesWatched: 2,
                  status: "watching",
                  score: 0,
                  isRewatching: false,
                  rewatchCount: 0,
                  startDate: "",
                  finishDate: "",
                  updatedAt: timestamp,
                },
                observedAt: timestamp,
                observationSource: "confirmation_read",
              },
            ]
          : [],
        legacy: true,
        replayEligible: true,
        replayReason: "",
        overrides: [],
      });
    if (method !== "GET" && path.startsWith("/api/sync/"))
      expect(route.request().headers()["x-shinkro-csrf"]).toBe("fixture-csrf");
    if (path === "/api/sync/sources/refresh") {
      refreshed = true;
      return json({ refreshedAt: timestamp });
    }
    if (path === "/api/sync/mal/search")
      return json({
        data: [
          {
            malId: 63367,
            title: event.title,
            totalEpisodes: 12,
            mediaType: "TV",
          },
        ],
      });
    if (path === "/api/sync/overrides" && method === "GET")
      return json({ data: [] });
    if (path === "/api/sync/overrides/preview") {
      const body = route.request().postDataJSON();
      const scopeReady = !options.serverMissing || body.anyServer;
      return json({
        scope: {
          ...body,
          serverId: options.serverMissing ? "" : "fixture-server",
        },
        affectedEventIds: scopeReady ? [42] : [],
        conflicts: [],
        ready: !!scopeReady,
        warnings: scopeReady
          ? []
          : [
              "Choose an explicit server scope before saving this historical rule.",
            ],
        fingerprint: scopeReady
          ? body.anyServer
            ? "fixture-rule-all-servers"
            : "fixture-rule"
          : "",
      });
    }
    if (path === "/api/sync/overrides" && method === "POST") {
      savedRule = route.request().postDataJSON();
      return json({
        id: "rule-1",
        revision: 1,
        active: true,
        ...(savedRule as object),
      });
    }
    if (path === "/api/sync/events/42/preview") {
      const body = route.request().postDataJSON();
      bodies.push(body);
      previewCount++;
      const ready = refreshed || !!body.correction;
      const preview = {
        id: `preview-${previewCount}`,
        eventId: 42,
        createdAt: timestamp,
        expiresAt: new Date(
          Date.now() + (options.expired ? -1000 : 300000),
        ).toISOString(),
        ready,
        writesEnabled: options.writes ?? true,
        code: ready ? "" : "ANIME_NOT_IN_DB",
        message: ready
          ? ""
          : "No matching MAL entry. Refresh mapping data or choose a MAL entry.",
        resolution: {
          malId: ready ? 63367 : 0,
          episode: ready ? 2 : 0,
          title: ready ? event.title : "",
          origin: body.correction ? "event_correction" : "canonical_anidb",
          sources: [
            {
              kind: "catalogue",
              label: "Anime catalogue",
              contentHash: "a".repeat(64),
              entries: 30617,
              observedAt: timestamp,
              ...(refreshed ? { lastRefreshAt: timestamp } : {}),
            },
            {
              kind: "community_tv",
              label: "Community TV mappings",
              contentHash: "b".repeat(64),
              entries: 5775,
              observedAt: timestamp,
            },
          ],
          steps: [
            {
              label: "Original source",
              detail: "AniDB 19859 · season 1 · episode 2",
            },
            {
              label: "Canonical AniDB",
              detail: ready ? "MAL 63367 · episode 2" : "No catalogue link",
            },
          ],
        },
        before: ready ? { episodesWatched: 1 } : undefined,
        after: ready ? { episodesWatched: 2 } : undefined,
        changes:
          ready && !options.noChange
            ? [
                { field: "episodesWatched", before: 1, after: 2 },
                ...(options.statusChange
                  ? [
                      {
                        field: "status",
                        before: "plan_to_watch",
                        after: "watching",
                      },
                    ]
                  : []),
              ]
            : [],
        warnings: options.warnings || [],
        accountId: "fixture-account",
      };
      lastPreview = preview;
      return json(preview);
    }
    if (path === "/api/sync/events/42/replays") {
      if (options.stale)
        return json(
          {
            code: "PREVIEW_STALE",
            message: "MAL state changed. Preview again.",
          },
          409,
        );
      applications++;
      expect(route.request().postDataJSON()).toEqual({
        previewId: `preview-${previewCount}`,
      });
      return json({
        id: "job-1",
        eventId: 42,
        status: "queued",
        message: "Queued for review",
        previewId: `preview-${previewCount}`,
      });
    }
    if (path === "/api/sync/jobs/job-1")
      return json({
        id: "job-1",
        eventId: 42,
        status: "synced",
        message: "MAL progress updated.",
        previewId: `preview-${previewCount}`,
      });
    return json({});
  });
  return {
    get applications() {
      return applications;
    },
    get bodies() {
      return bodies;
    },
    get savedRule() {
      return savedRule;
    },
  };
}
test("Beerus recovery retains history and applies only after explicit review", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto("/sync?status=needs_mapping&q=19859");
  await expect(
    page.getByText("Dragon Ball Super: Beerus", { exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Inspect", exact: true }).click();
  await expect(
    page.getByText("Historical decision trace unavailable.", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByText("ANIME_NOT_IN_DB", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(page.getByText("Cannot apply this preview")).toBeVisible();
  expect(state.applications).toBe(0);
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Preview again", exact: true })
    .click();
  await expect(
    page.getByText("Episodes watched", { exact: true }),
  ).toBeVisible();
  await page.getByText("How this match was found", { exact: true }).click();
  await expect(page.getByText("Data used", { exact: true })).toBeVisible();
  await expect(
    page.getByText("Last successful refresh", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Local observation", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Upstream freshness is unknown.", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("a".repeat(64), { exact: true })).toBeHidden();
  await expect(
    page.getByText("Canonical AniDB", { exact: true }).first(),
  ).toBeVisible();
  expect(state.applications).toBe(0);
  await page.screenshot({
    path: `test-results/beerus-preview-${test.info().project.name}.png`,
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Sync MAL 63367 to episode 2", exact: true })
    .click();
  await expect(page.getByText("MAL progress updated.").first()).toBeVisible();
  expect(state.applications).toBe(1);
  await page
    .getByText("View saved plan and MAL observation", { exact: true })
    .click();
  await expect(
    page.getByText("Observed on MAL", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Confirmed by a MAL read after update", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("MAL at preview", { exact: true })).toBeVisible();
  await expect(page.getByText("Planned change", { exact: true })).toBeVisible();
  await expect(
    page.getByText("These values are not a fresh MAL read.", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByText("ANIME_NOT_IN_DB", { exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "← Back to events" }).click();
  await expect(page.getByLabel("Search events")).toHaveValue("19859");
  await expect(page.getByRole("combobox", { name: "MAL outcome" })).toHaveValue(
    "Needs mapping",
  );
});
test("event correction remains personal; scoped rule save never applies MAL", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Choose a different MAL entry" })
    .click();
  await page.getByLabel("MAL title or numeric ID").fill("63367");
  await page.getByRole("button", { name: "Search MAL entries" }).click();
  await page.getByRole("button", { name: "Select entry" }).click();
  await page.getByRole("button", { name: "Save as a local rule…" }).click();
  await page.getByLabel("Source episode end").fill("12");
  await page.screenshot({
    path: `test-results/beerus-rule-${test.info().project.name}.png`,
    fullPage: true,
  });
  await page.getByRole("button", { name: "Review rule impact" }).click();
  await expect(
    page.getByText("1 stored events match this scope.", { exact: false }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Save local rule", exact: true })
    .click();
  await expect(
    page.getByText("Local rule saved.", { exact: false }),
  ).toBeVisible();
  expect(state.applications).toBe(0);
  expect(state.savedRule).toEqual({
    impactFingerprint: "fixture-rule",
    eventId: 42,
    malId: 63367,
    sourceEpisodeStart: 1,
    sourceEpisodeEnd: 12,
    malEpisodeStart: 1,
  });
  await page
    .getByRole("button", { name: "Preview correction", exact: true })
    .click();
  await expect(
    page.getByText("This event only", { exact: true }),
  ).toBeVisible();
  expect(state.bodies).toEqual([{ correction: { malId: 63367, episode: 2 } }]);
  expect(state.applications).toBe(0);
});
test("stale approval is retained but disabled until a new preview", async ({
  page,
}) => {
  const state = await fixture(page, { stale: true });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await page
    .getByRole("button", { name: "Sync MAL 63367 to episode 2" })
    .click();
  await expect(page.getByText("State changed — preview again")).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sync MAL 63367 to episode 2" }),
  ).toBeDisabled();
  expect(state.applications).toBe(0);
});
test("dev mode shows persistent isolation and cannot apply; narrow layout fits", async ({
  page,
}) => {
  const state = await fixture(page, { writes: false });
  await page.goto("/sync/events/42");
  await expect(
    page.getByText("Development build", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("MAL writes disabled on this instance"),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Sync MAL 63367 to episode 2" }),
  ).toBeDisabled();
  expect(state.applications).toBe(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
test("operator session failure is actionable without submitting a write", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.route("**/api/sync/events/42/preview", (route) =>
    route.fulfill({
      status: 403,
      contentType: "application/json",
      body: JSON.stringify({
        code: "SESSION_REQUIRED",
        message: "An operator session is required. Sign in again, then retry.",
      }),
    }),
  );
  await page.goto("/sync/events/42");
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(
    page.getByRole("alert", { name: "Unable to complete this action" }),
  ).toContainText("Sign in again");
  expect(state.applications).toBe(0);
});
test("a correction draft disables an earlier approval until the editor closes", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  const apply = page.getByRole("button", {
    name: "Sync MAL 63367 to episode 2",
  });
  await expect(apply).toBeEnabled();
  await page
    .getByRole("button", { name: "Choose a different MAL entry" })
    .click();
  await expect(apply).toBeDisabled();
  await expect(
    page.getByText("Previous preview", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Ready to review", { exact: true })).toHaveCount(
    0,
  );
  await expect(
    page.getByText("Close the correction editor or select a correction", {
      exact: false,
    }),
  ).toBeVisible();
  expect(state.applications).toBe(0);
});
test("rule lifecycle requires reviewed scope and current revision", async ({
  page,
}) => {
  const state = await fixture(page);
  let rule = {
    id: "rule-1",
    revision: 3,
    eventId: 42,
    source: "anidb",
    sourceId: 19859,
    season: 1,
    library: "Anime",
    account: "sample-user",
    serverId: "fixture-server",
    mediaType: "episode",
    sourceEpisodeStart: 1,
    sourceEpisodeEnd: 12,
    malEpisodeStart: 1,
    malId: 63367,
    active: true,
    createdAt: timestamp,
  };
  let edits = 0;
  await page.route("**/api/sync/overrides**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const json = (body: unknown, status = 200) =>
      route.fulfill({
        status,
        contentType: "application/json",
        body: JSON.stringify(body),
      });
    if (url.pathname === "/api/sync/overrides" && request.method() === "GET")
      return json({ data: [rule] });
    if (url.pathname === "/api/sync/overrides/preview")
      return json({
        scope: request.postDataJSON(),
        affectedEventIds: [42],
        conflicts: [],
        ready: true,
        warnings: [],
        fingerprint: "reviewed-edit",
      });
    if (request.method() === "PUT") {
      const body = request.postDataJSON();
      expect(body.revision).toBe(rule.revision);
      expect(body.impactFingerprint).toBe("reviewed-edit");
      rule = { ...body, revision: rule.revision + 1 };
      edits++;
      return json(rule);
    }
    if (request.method() === "DELETE") {
      expect(url.searchParams.get("revision")).toBe(String(rule.revision));
      rule = { ...rule, active: false, revision: rule.revision + 1 };
      return route.fulfill({ status: 204 });
    }
    return route.fallback();
  });
  await page.goto("/sync?tab=rules");
  await page.getByRole("button", { name: "Edit rule", exact: true }).click();
  await page.getByLabel("Source episode end").fill("11");
  await page
    .getByRole("button", { name: "Discard edits", exact: true })
    .click();
  await expect(
    page.getByText("Discard these unsaved rule edits?", { exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel("Source episode end")).toHaveValue("11");
  await page.getByRole("button", { name: "Keep editing", exact: true }).click();
  await expect(page.getByLabel("Source episode end")).toHaveValue("11");
  expect(edits).toBe(0);
  await page
    .getByRole("button", { name: "Discard edits", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Discard rule edits", exact: true })
    .click();
  await expect(page.getByLabel("Source episode end")).toHaveCount(0);
  await expect(page.getByText("Revision 3", { exact: true })).toBeVisible();
  expect(edits).toBe(0);
  await page.getByRole("button", { name: "Edit rule", exact: true }).click();
  await expect(page.getByLabel("Source episode end")).toHaveValue("12");
  await page.getByLabel("Source episode end").fill("10");
  await expect(
    page.getByRole("button", { name: "Save rule revision" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Review edited rule" }).click();
  await page.getByRole("button", { name: "Save rule revision" }).click();
  await expect(page.getByText("Revision 4", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Disable rule", exact: true }).click();
  await page.getByRole("button", { name: "Confirm disable" }).click();
  await expect(
    page.getByRole("button", { name: "Enable rule", exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Enable rule", exact: true }).click();
  await page.getByRole("button", { name: "Review edited rule" }).click();
  await page.getByRole("button", { name: "Save rule revision" }).click();
  await expect(page.getByText("Revision 6", { exact: true })).toBeVisible();
  expect(edits).toBe(2);
  expect(state.applications).toBe(0);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({
    path: `test-results/rules-${test.info().project.name}.png`,
    fullPage: true,
  });
});
test("expired previews and already satisfied progress never offer a write", async ({
  page,
}) => {
  const state = await fixture(page, { expired: true });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(
    page.getByText("Preview expired", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sync MAL 63367 to episode 2" }),
  ).toBeDisabled();
  expect(state.applications).toBe(0);
});
test("an already satisfied preview shows no change needed", async ({
  page,
}) => {
  const state = await fixture(page, { noChange: true });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(
    page.getByText("No change needed", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Sync MAL 63367 to episode 2" }),
  ).toHaveCount(0);
  expect(state.applications).toBe(0);
});
test("conflicting local ranges cannot be saved", async ({ page }) => {
  const state = await fixture(page);
  await page.route("**/api/sync/overrides/preview", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ready: false,
        scope: {},
        affectedEventIds: [42],
        warnings: [],
        conflicts: [
          {
            id: "existing-rule",
            revision: 2,
            source: "anidb",
            sourceId: 19859,
            sourceEpisodeStart: 1,
            sourceEpisodeEnd: 12,
            malId: 63367,
          },
        ],
        fingerprint: "conflict",
      }),
    }),
  );
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Choose a different MAL entry" })
    .click();
  await page.getByLabel("MAL title or numeric ID").fill("63367");
  await page.getByRole("button", { name: "Search MAL entries" }).click();
  await page.getByRole("button", { name: "Select entry" }).click();
  await page.getByRole("button", { name: "Save as a local rule…" }).click();
  await page.getByRole("button", { name: "Review rule impact" }).click();
  await expect(
    page.getByText("Conflict with revision 2:", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Save local rule", exact: true }),
  ).toBeDisabled();
  expect(state.savedRule).toBeUndefined();
  expect(state.applications).toBe(0);
});
test("an unavailable event list keeps filters and offers retry", async ({
  page,
}) => {
  await fixture(page);
  await page.route("**/api/sync/events?**", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({
        code: "SOURCE_UNAVAILABLE",
        message: "The event store is temporarily unavailable. Retry shortly.",
      }),
    }),
  );
  await page.goto("/sync?q=Beerus");
  await expect(
    page.getByRole("button", { name: "Retry loading events" }),
  ).toBeVisible({ timeout: 15000 });
  await expect(page.getByLabel("Search events")).toHaveValue("Beerus");
});

test("a legacy rule requires explicit all-server scope and a new impact review", async ({
  page,
}) => {
  const state = await fixture(page, { serverMissing: true });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Choose a different MAL entry" })
    .click();
  await page.getByLabel("MAL title or numeric ID").fill("63367");
  await page.getByRole("button", { name: "Search MAL entries" }).click();
  await page.getByRole("button", { name: "Select entry" }).click();
  await expect(
    page.getByRole("checkbox", {
      name: "All servers in this library and Plex account",
    }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Save as a local rule…" }).click();
  const allServers = page.getByRole("checkbox", {
    name: "All servers in this library and Plex account",
  });
  await expect(allServers).not.toBeChecked();
  await page.getByRole("button", { name: "Review rule impact" }).click();
  await expect(
    page.getByRole("button", { name: "Save local rule", exact: true }),
  ).toBeDisabled();
  expect(state.savedRule).toBeUndefined();
  await allServers.check();
  await expect(
    page.getByRole("button", { name: "Save local rule", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Review rule impact" }).click();
  await expect(
    page.getByText("Server scope: All servers · sample-user / Anime", {
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Save local rule", exact: true })
    .click();
  await expect(
    page.getByText("Local rule saved.", { exact: false }),
  ).toBeVisible();
  expect(state.savedRule).toEqual({
    eventId: 42,
    malId: 63367,
    sourceEpisodeStart: 1,
    sourceEpisodeEnd: 2,
    malEpisodeStart: 1,
    anyServer: true,
    impactFingerprint: "fixture-rule-all-servers",
  });
  expect(state.applications).toBe(0);
});

test("events remain readable at mobile, tablet-sidebar and desktop widths", async ({
  page,
}) => {
  await fixture(page, { writes: false });
  const events = Array.from({ length: 8 }, (_, index) => ({
    ...event,
    id: index + 1,
    title: `Example anime ${index + 1}: A very long descriptive title for a readable event history`,
    status: index % 2 ? "synced" : "needs_mapping",
    errorMessage: index % 2 ? "" : event.errorMessage,
    malId: index % 2 ? 63367 : 0,
    malTitle:
      index % 2
        ? "Matched destination title: A long alternate title with a season subtitle and additional catalogue information"
        : "",
  }));
  await page.route("**/api/sync/events?**", (route) =>
    route.fulfill({ json: { data: events, count: 8 } }),
  );
  for (const width of [375, 390, 820, 900, 1440]) {
    await page.setViewportSize({ width, height: 800 });
    await page.goto("/sync?status=needs_mapping&q=19859");
    await expect(
      page.getByRole("link", { name: "Inspect", exact: true }),
    ).toHaveCount(8);
    for (const label of ["Search events", "MAL outcome", "Source provider"]) {
      const control = page.getByRole(
        label === "Search events" ? "textbox" : "combobox",
        { name: label, exact: true },
      );
      await expect(control).toBeVisible();
      const bounds = await control.boundingBox();
      expect(
        bounds!.width,
        `${label} usable at viewport ${width}`,
      ).toBeGreaterThanOrEqual(140);
    }
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await expect(
      page.getByRole("button", { name: "Needs mapping", exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator('[aria-label="Event pages"]')).toBeVisible();
    await expect(
      page.getByText("Matched destination title:", { exact: false }).first(),
    ).toBeVisible();
    if (width <= 390) {
      for (const name of [
        "Needs mapping",
        "Clear filters",
        "Refresh mapping data",
      ]) {
        const bounds = await page
          .getByRole("button", { name, exact: true })
          .boundingBox();
        expect(
          bounds!.height,
          `${name} touch target at ${width}`,
        ).toBeGreaterThanOrEqual(40);
      }
    }
    if (width === 1440) {
      await expect(
        page.getByRole("table", { name: "Sync events" }),
      ).toBeVisible();
      const row = page
        .getByRole("table", { name: "Sync events", exact: true })
        .getByRole("row")
        .nth(4);
      const bounds = await row.boundingBox();
      expect(bounds!.y + bounds!.height).toBeLessThan(800);
    } else {
      await expect(
        page.getByRole("table", { name: "Sync events" }),
      ).toHaveCount(0);
    }
  }
});

test("recorded result and preview precede metadata and match diagnostics", async ({
  page,
}) => {
  await fixture(page, { writes: false });
  await page.setViewportSize({ width: 1440, height: 800 });
  await page.goto("/sync/events/42");
  const preview = page.getByRole("button", {
    name: "Preview now",
    exact: true,
  });
  await expect(preview).toBeVisible();
  const bounds = await preview.boundingBox();
  expect(bounds!.y + bounds!.height).toBeLessThan(800);
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await preview.click();
  await expect(
    page.getByRole("table", { name: "Proposed MAL changes" }),
  ).toBeVisible();
  await expect(page.getByText("Data used", { exact: true })).toBeHidden();
  const apply = page.getByRole("button", {
    name: "Sync MAL 63367 to episode 2",
    exact: true,
  });
  await expect(apply).toBeDisabled();
  const applyBounds = await apply.boundingBox();
  const disclosureBounds = await page
    .getByText("How this match was found", { exact: true })
    .boundingBox();
  expect(applyBounds!.y).toBeLessThan(disclosureBounds!.y);
  await page.getByText("How this match was found", { exact: true }).click();
  await expect(page.getByText("Data used", { exact: true })).toBeVisible();
});

test("empty event history differs from filtered no-results and has no pagination", async ({
  page,
}) => {
  await fixture(page);
  await page.route("**/api/sync/events?**", (route) =>
    route.fulfill({ json: { data: [], count: 0 } }),
  );
  await page.goto("/sync");
  await expect(
    page.getByText("No sync events yet", { exact: true }),
  ).toBeVisible();
  await expect(page.locator('[aria-label="Event pages"]')).toHaveCount(0);
  await page.getByLabel("Search events", { exact: true }).fill("No match");
  await expect(
    page.getByText("No events match these filters", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Clear filters", exact: true })
    .last()
    .click();
  await expect(
    page.getByText("No sync events yet", { exact: true }),
  ).toBeVisible();
});

test("disabled write policy is concise while distinct preview warnings remain visible", async ({
  page,
}) => {
  await fixture(page, {
    writes: false,
    warnings: [
      "MAL writes are disabled on this instance. You can inspect, preview, and save local rules.",
      "Original server identity was not saved. Confirm this event belongs to your server.",
    ],
  });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  await expect(
    page.getByText(
      "MAL writes are disabled on this instance. You can inspect, preview, and save local rules.",
      { exact: true },
    ),
  ).toHaveCount(0);
  await expect(
    page.getByText(
      "Original server identity was not saved. Confirm this event belongs to your server.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(
    page.getByText(
      "Applying is disabled on this instance. This preview has not changed MAL.",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: "Sync MAL 63367 to episode 2",
      exact: true,
    }),
  ).toBeDisabled();
});

test("detail read error retries locally and preserves event-list context", async ({
  page,
}) => {
  await fixture(page);
  let recovered = false;
  await page.route("**/api/sync/events/42", (route) =>
    recovered
      ? route.fallback()
      : route.fulfill({
          status: 503,
          json: {
            code: "UNAVAILABLE",
            message: "Event temporarily unavailable.",
          },
        }),
  );
  await page.goto("/sync/events/42?q=19859&status=needs_mapping");
  const retry = page.getByRole("button", {
    name: "Retry loading event",
    exact: true,
  });
  await expect(retry).toBeVisible({ timeout: 15000 });
  recovered = true;
  await retry.click();
  await expect(
    page.getByRole("button", { name: "Preview now", exact: true }),
  ).toBeEnabled();
  await page
    .getByRole("link", { name: "← Back to events", exact: true })
    .click();
  await expect(page.getByLabel("Search events", { exact: true })).toHaveValue(
    "19859",
  );
  await expect(
    page.getByRole("combobox", { name: "MAL outcome", exact: true }),
  ).toHaveValue("Needs mapping");
});

test("essential metadata, amber status and server-scope captions retain readable contrast", async ({
  page,
}) => {
  await fixture(page, { writes: false, serverMissing: true });
  await page.setViewportSize({ width: 375, height: 844 });
  await page.goto("/sync/events/42");
  for (const theme of ["light", "dark"]) {
    if (
      (await page.locator("html").getAttribute("data-mantine-color-scheme")) !==
      theme
    )
      await page
        .getByRole("button", { name: "Theme Switch", exact: true })
        .click();
    await expect(page.locator("html")).toHaveAttribute(
      "data-mantine-color-scheme",
      theme,
    );
    const samples = [
      [
        "episode metadata",
        page.getByText(/Season 1 · Episode 2 · Watched · Event #42/),
      ],
      [
        "disabled banner heading",
        page.getByText("MAL writes disabled on this instance", { exact: true }),
      ],
      [
        "mapping outcome",
        page.getByText("Needs mapping", { exact: true }).first(),
      ],
      [
        "preview instruction",
        page.getByText(
          "Preview reads the current mapping and MAL list state.",
          { exact: false },
        ),
      ],
    ] as const;
    for (const [name, locator] of samples) {
      const sample = await measureContrast(locator);
      console.log(`${theme} ${name}: ${JSON.stringify(sample)}`);
      expect(sample.ratio, `${theme} ${name}`).toBeGreaterThanOrEqual(4.5);
    }
  }
  await page
    .getByRole("button", { name: "Choose a different MAL entry", exact: true })
    .click();
  await page
    .getByLabel("MAL title or numeric ID", { exact: true })
    .fill("63367");
  await page
    .getByRole("button", { name: "Search MAL entries", exact: true })
    .click();
  await page.getByRole("button", { name: "Select entry", exact: true }).click();
  await page
    .getByRole("button", { name: "Save as a local rule…", exact: true })
    .click();
  const caption = page.locator(".mantine-Checkbox-description");
  const scopeSample = await measureContrast(caption);
  console.log(`dark server scope caption: ${JSON.stringify(scopeSample)}`);
  expect(scopeSample.ratio).toBeGreaterThanOrEqual(4.5);
});

async function measureContrast(locator: Locator) {
  return locator.evaluate((element) => {
    const rgba = (value: string) => {
      const values = value.match(/[\d.]+/g)!.map(Number);
      return value.startsWith("color(srgb")
        ? [
            ...values.slice(0, 3).map((channel) => channel * 255),
            values[3] ?? 1,
          ]
        : values;
    };
    const over = (front: number[], back: number[]) =>
      front
        .slice(0, 3)
        .map((v, i) => v * (front[3] ?? 1) + back[i] * (1 - (front[3] ?? 1)));
    const chain: Element[] = [];
    for (let node: Element | null = element; node; node = node.parentElement)
      chain.unshift(node);
    let background = [255, 255, 255];
    for (const node of chain)
      background = over(
        rgba(getComputedStyle(node).backgroundColor),
        background,
      );
    const style = getComputedStyle(element);
    const color = rgba(style.color);
    color[3] = (color[3] ?? 1) * Number(style.opacity);
    const foreground = over(color, background);
    const luminance = (rgb: number[]) =>
      rgb
        .map((c) => c / 255)
        .map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
        .reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
    const a = luminance(foreground),
      b = luminance(background);
    return {
      foreground: foreground.map(Math.round),
      background: background.map(Math.round),
      ratio: (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05),
    };
  });
}

test("narrow preview uses labelled current and proposed values without splitting watch status", async ({
  page,
}) => {
  await fixture(page, { writes: false, statusChange: true });
  await page.setViewportSize({ width: 375, height: 844 });
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Refresh mapping data", exact: true })
    .click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
  const changes = page.locator('[aria-label="Proposed MAL changes"]');
  await expect(changes).toBeVisible();
  await expect(
    page.getByRole("table", { name: "Proposed MAL changes", exact: true }),
  ).toHaveCount(0);
  await expect(
    changes.getByText("Watch status", { exact: true }),
  ).toBeVisible();
  await expect(
    changes.getByText("Plan to watch", { exact: true }),
  ).toBeVisible();
  const watching = changes.getByText("Watching", { exact: true });
  await expect(watching).toBeVisible();
  expect((await watching.boundingBox())!.height).toBeLessThan(30);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

test("small semantic outcome labels meet light-theme text contrast", async ({
  page,
}) => {
  await fixture(page, { writes: false });
  const statuses = [
    "synced",
    "no_change",
    "needs_mapping",
    "needs_sign_in",
    "needs_review",
    "failed",
    "queued",
    "processing",
    "received",
    "cancelled",
  ];
  await page.route("**/api/sync/events?**", (route) =>
    route.fulfill({
      json: {
        data: statuses.map((status, index) => ({
          ...event,
          id: index + 1,
          status,
          errorMessage: "",
        })),
        count: statuses.length,
      },
    }),
  );
  await page.goto("/sync");
  if (
    (await page.locator("html").getAttribute("data-mantine-color-scheme")) !==
    "light"
  )
    await page
      .getByRole("button", { name: "Theme Switch", exact: true })
      .click();
  for (const label of [
    "Synced",
    "No change needed",
    "Needs mapping",
    "Needs sign-in",
    "Needs review",
    "Failed",
    "Queued",
    "Processing",
    "Not recorded",
    "Cancelled",
  ]) {
    const outcome = page
      .locator(".mantine-Badge-root")
      .getByText(label, { exact: true });
    await expect(outcome).toBeVisible();
    const sample = await measureContrast(outcome);
    console.log(`light ${label}: ${JSON.stringify(sample)}`);
    expect(sample.ratio, label).toBeGreaterThanOrEqual(4.5);
  }
});

test("persistent rule edits survive attempted tab and source navigation until explicit discard", async ({
  page,
}) => {
  const state = await fixture(page);
  await page.route("**/api/sync/overrides", (route) =>
    route.request().method() === "GET"
      ? route.fulfill({
          json: {
            data: [
              {
                id: "draft-rule",
                revision: 1,
                eventId: 42,
                source: "anidb",
                sourceId: 19859,
                season: 1,
                library: "Anime",
                account: "sample-user",
                serverId: "fixture-server",
                mediaType: "episode",
                sourceEpisodeStart: 1,
                sourceEpisodeEnd: 12,
                malEpisodeStart: 1,
                malId: 63367,
                active: true,
                createdAt: timestamp,
              },
            ],
          },
        })
      : route.fallback(),
  );
  await page.goto("/sync/events/42");
  await page
    .getByRole("link", { name: "← Back to events", exact: true })
    .click();
  await page.getByRole("tab", { name: "Local rules", exact: true }).click();
  await page.getByRole("button", { name: "Edit rule", exact: true }).click();
  await page.getByLabel("Source episode end", { exact: true }).fill("11");
  await page.getByRole("tab", { name: "Events", exact: true }).click();
  const confirmation = page.getByRole("dialog", {
    name: "Discard unsaved rule changes?",
    exact: true,
  });
  await expect(confirmation).toBeVisible();
  await confirmation
    .getByRole("button", { name: "Keep editing", exact: true })
    .click();
  await page.evaluate(() => window.history.back());
  await expect(confirmation).toBeVisible();
  await confirmation
    .getByRole("button", { name: "Keep editing", exact: true })
    .click();

  await expect(
    page.getByLabel("Source episode end", { exact: true }),
  ).toHaveValue("11");
  await page
    .getByRole("link", { name: "Inspect source event", exact: true })
    .click();
  await expect(confirmation).toBeVisible();
  await confirmation
    .getByRole("button", { name: "Keep editing", exact: true })
    .click();
  await expect(
    page.getByLabel("Source episode end", { exact: true }),
  ).toHaveValue("11");
  await page
    .getByRole("link", { name: "Inspect source event", exact: true })
    .click();
  await confirmation
    .getByRole("button", { name: "Discard and continue", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Preview now", exact: true }),
  ).toBeVisible();
  expect(state.applications).toBe(0);
  expect(state.savedRule).toBeUndefined();
});

test("new persistent drafts guard Back navigation while event-only lookups stay free", async ({
  page,
}) => {
  await fixture(page);
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Choose a different MAL entry", exact: true })
    .click();
  await page
    .getByLabel("MAL title or numeric ID", { exact: true })
    .fill("63367");
  await page
    .getByRole("button", { name: "Search MAL entries", exact: true })
    .click();
  await page.getByRole("button", { name: "Select entry", exact: true }).click();
  await page
    .getByRole("button", { name: "Save as a local rule…", exact: true })
    .click();
  await page
    .getByRole("link", { name: "← Back to events", exact: true })
    .click();
  const confirmation = page.getByRole("dialog", {
    name: "Discard unsaved rule changes?",
    exact: true,
  });
  await expect(confirmation).toBeVisible();
  await confirmation
    .getByRole("button", { name: "Keep editing", exact: true })
    .click();
  await expect(
    page.getByLabel("Source episode end", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "← Back to events", exact: true })
    .click();
  await confirmation
    .getByRole("button", { name: "Discard and continue", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Sync Inspector", exact: true }),
  ).toBeVisible();
  await page.goto("/sync/events/42");
  await page
    .getByRole("button", { name: "Choose a different MAL entry", exact: true })
    .click();
  await page
    .getByLabel("MAL title or numeric ID", { exact: true })
    .fill("Lookup draft");
  await page
    .getByRole("link", { name: "← Back to events", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Sync Inspector", exact: true }),
  ).toBeVisible();
  await expect(confirmation).toHaveCount(0);
});

for (const movie of [false, true]) {
  test(`keyboard destination selection focuses ${movie ? "movie preview" : "episode review"} and remains read-only`, async ({
    page,
  }) => {
    const state = await fixture(page, { writes: false, movie });
    await page.goto("/sync/events/42");
    await page
      .getByRole("button", {
        name: "Choose a different MAL entry",
        exact: true,
      })
      .click();
    await page
      .getByLabel("MAL title or numeric ID", { exact: true })
      .fill("63367");
    await page
      .getByRole("button", { name: "Search MAL entries", exact: true })
      .click();
    const select = page.getByRole("button", {
      name: "Select entry",
      exact: true,
    });
    await select.focus();
    await page.keyboard.press("Enter");
    const preview = page.getByRole("button", {
      name: "Preview correction",
      exact: true,
    });
    const target = page.getByLabel("Target MAL episode", { exact: true });
    await expect(movie ? preview : target).toBeFocused();
    await expect(
      page.getByText("Selected destination", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("link", {
        name: "Dragon Ball Super: Beerus · MAL 63367",
        exact: true,
      }),
    ).toBeVisible();
    expect(state.bodies).toEqual([]);
    expect(state.applications).toBe(0);
    await page
      .getByRole("button", { name: "Change entry", exact: true })
      .focus();
    await page.keyboard.press("Enter");
    const searchInput = page.getByLabel("MAL title or numeric ID", {
      exact: true,
    });
    await expect(searchInput).toBeFocused();
    await select.focus();
    await page.keyboard.press("Enter");
    await expect(movie ? preview : target).toBeFocused();
    await page
      .getByRole("button", { name: "Save as a local rule…", exact: true })
      .focus();
    await page.keyboard.press("Enter");
    await expect(
      page.getByRole("heading", {
        name: "Create a persistent local rule",
        exact: true,
      }),
    ).toBeFocused();
    await page
      .getByRole("button", { name: "← Back to event correction", exact: true })
      .focus();
    await page.keyboard.press("Enter");
    await page
      .getByRole("button", { name: "Discard rule draft", exact: true })
      .focus();
    await page.keyboard.press("Enter");
    await expect(movie ? preview : target).toBeFocused();
    if (!movie) {
      await target.fill("3");
      await page.keyboard.press("Tab");
      await expect(preview).toBeFocused();
    }
    await page.keyboard.press("Enter");
    await expect(
      page.getByText("This event only", { exact: true }),
    ).toBeVisible();
    expect(state.bodies).toEqual([
      { correction: { malId: 63367, episode: movie ? 1 : 3 } },
    ]);
    expect(state.applications).toBe(0);
    expect(state.savedRule).toBeUndefined();
  });
}
