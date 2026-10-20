import { test, expect, type Page } from "@playwright/test";
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
  } = {},
) {
  page.on("pageerror", (error) => console.log("PAGE ERROR", error.message));
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
    if (path === "/api/sync/events") return json({ data: [event], count: 1 });
    if (path === "/api/sync/events/42")
      return json({
        event: applications
          ? {
              ...event,
              status: "synced",
              errorCode: "",
              errorMessage: "",
              malId: 63367,
              malTitle: event.title,
              lastAction: "replay",
            }
          : event,
        original: event,
        input: {
          ...event,
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
            ? [{ field: "episodesWatched", before: 1, after: 2 }]
            : [],
        warnings: [],
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
    page.getByText("Historical decision trace unavailable"),
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
    page.getByRole("cell", { name: "Episodes watched", exact: true }),
  ).toBeVisible();
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
  await expect(
    page.getByText("Observed MAL result", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("Confirmed by a MAL read after update", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "MAL at preview", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("columnheader", { name: "Planned change", exact: true }),
  ).toBeVisible();
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
  await page.getByRole("button", { name: "Use this correction" }).click();
  await page.getByRole("button", { name: "Preview now", exact: true }).click();
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
