import { syncKeys } from "@components/sync/values";
import { useEffect, useRef, useState } from "react";
import {
  Alert,
  Badge,
  Button,
  Container,
  Divider,
  Group,
  NumberInput,
  Paper,
  SimpleGrid,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { SyncEventRoute } from "@app/routes";
import {
  syncAPI,
  SyncError,
  type EventDetail,
  type Job,
  type MALResult,
  type Preview,
} from "@api/sync";
import {
  ChangeDiff,
  ErrorNotice,
  MALLink,
  Outcome,
  ObservedResult,
  ReadableTime,
  ResolutionTrace,
  RuleServerScope,
  Section,
  SignInAction,
} from "@components/sync/shared";
import { EventAge } from "@components/sync/EventList";
import { RuleDraftGuard } from "@components/sync/RuleDraftGuard";
import classes from "@components/sync/SyncInspector.module.css";

export function SyncEventDetail() {
  const { eventId } = SyncEventRoute.useParams();
  return <SyncEventContent key={eventId} />;
}

function SyncEventContent() {
  const { eventId } = SyncEventRoute.useParams();
  const id = Number(eventId);
  const search = SyncEventRoute.useSearch();
  const queryClient = useQueryClient();
  const [preview, setPreview] = useState<Preview | null>(null);
  const [correction, setCorrection] = useState<
    { malId: number; episode: number } | undefined
  >();
  const [editor, setEditor] = useState(false);
  const [job, setJob] = useState<Job | null>(null);
  const [stale, setStale] = useState(false);
  const [clock, setClock] = useState(Date.now());
  const draftVersion = useRef(0);
  const evaluationRef = useRef<HTMLDivElement>(null);
  const correctionButtonRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (preview) {
      evaluationRef.current?.focus({ preventScroll: true });
      evaluationRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
      });
    }
  }, [preview]);
  const cap = useQuery({
    queryKey: syncKeys.capabilities,
    queryFn: syncAPI.capabilities,
  });
  const detail = useQuery({
    queryKey: syncKeys.detail(id),
    queryFn: () => syncAPI.detail(id),
    enabled: Number.isSafeInteger(id) && id > 0,
  });
  useEffect(() => {
    const timer = setInterval(() => setClock(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: syncKeys.all });
    for (const key of [
      "plex",
      "animeUpdate",
      "animeUpdateCount",
      "recentAnimeUpdates",
    ])
      void queryClient.invalidateQueries({ queryKey: [key] });
  };
  const previewMutation = useMutation({
    mutationFn: async (selectedCorrection?: {
      malId: number;
      episode: number;
    }) => {
      const version = draftVersion.current;
      const value = await syncAPI.preview(
        id,
        cap.data!.csrfToken,
        selectedCorrection || correction,
      );
      return { version, value };
    },
    onSuccess: ({ version, value }) => {
      if (version === draftVersion.current) {
        setPreview(value);
        setStale(false);
        setJob(null);
      }
    },
  });
  const refresh = useMutation({
    mutationFn: () => syncAPI.refresh(cap.data!.csrfToken),
    onSuccess: () => {
      draftVersion.current++;
      setStale(true);
      invalidate();
    },
  });
  const apply = useMutation({
    mutationFn: () => syncAPI.replay(id, preview!.id, cap.data!.csrfToken),
    onSuccess: (value) => {
      setJob(value);
      invalidate();
    },
    onError: (error) => {
      if (error instanceof SyncError && error.status === 409) setStale(true);
    },
  });
  const polling = useQuery({
    queryKey: ["sync", "job", job?.id],
    queryFn: () => syncAPI.job(job!.id),
    enabled: !!job && ["queued", "processing"].includes(job.status),
    refetchInterval: (query) =>
      query.state.data &&
      !["queued", "processing"].includes(query.state.data.status)
        ? false
        : 1000,
  });
  useEffect(() => {
    if (
      polling.data &&
      !["queued", "processing"].includes(polling.data.status)
    ) {
      setJob(polling.data);
      void queryClient.invalidateQueries({ queryKey: syncKeys.all });
      for (const key of [
        "plex",
        "animeUpdate",
        "animeUpdateCount",
        "recentAnimeUpdates",
      ])
        void queryClient.invalidateQueries({ queryKey: [key] });
    }
  }, [polling.data, queryClient]);
  const expired =
    !!preview?.expiresAt && new Date(preview.expiresAt).getTime() <= clock;
  const busy = !!job && ["queued", "processing"].includes(job.status);
  const shownJob = polling.data || job;
  if (!Number.isSafeInteger(id) || id < 1)
    return (
      <Container py="lg">
        <Alert color="yellow" title="Invalid event ID">
          Open an event from Sync Inspector to inspect its recorded source.
        </Alert>
        <Link to="/sync" search={search}>
          Back to events
        </Link>
      </Container>
    );
  if (detail.isPending)
    return (
      <Container py="lg">
        <Text role="status">Loading sync event…</Text>
      </Container>
    );
  if (detail.error || !detail.data)
    return (
      <Container py="lg">
        <Stack>
          <ErrorNotice error={detail.error || new Error("Event not found.")} />
          <Group>
            <Button onClick={() => void detail.refetch()}>
              Retry loading event
            </Button>
            <Button
              renderRoot={(props) => (
                <Link {...props} to="/sync" search={search} />
              )}
            >
              Back to events
            </Button>
          </Group>
        </Stack>
      </Container>
    );
  const data = detail.data;
  const event = data.event;
  const original = data.original || event;
  const input = data.input;
  const changeCorrection = (
    value: { malId: number; episode: number } | undefined,
  ) => {
    draftVersion.current++;
    setCorrection(value);
    setPreview(null);
    setJob(null);
    setStale(false);
  };
  const applyLabel =
    preview?.changes.some((c) => c.field === "score") &&
    !preview.changes.some((c) => c.field.toLowerCase().includes("episode"))
      ? `Update score on MAL ${preview?.resolution.malId}`
      : `Sync MAL ${preview?.resolution.malId} to episode ${preview?.resolution.episode}`;
  return (
    <Container size={1200} px="md" component="main" className={classes.page}>
      <Stack gap="md" mt="md" pb="lg">
        <Link to="/sync" search={search}>
          ← Back to events
        </Link>
        <Group
          justify="space-between"
          align="start"
          className={classes.detailHeader}
        >
          <div style={{ minWidth: 0, flex: 1 }}>
            <Title order={2} style={{ overflowWrap: "anywhere" }}>
              {event.title || "Untitled event"}
            </Title>
            <Text c="dimmed">
              {event.mediaType === "movie"
                ? "Movie"
                : `Season ${event.season} · Episode ${event.episode}`}{" "}
              ·{" "}
              {event.event.includes("rate")
                ? `Rating ${event.rating}/10`
                : "Watched"}{" "}
              · Event #{id}
            </Text>
          </div>
          <Outcome status={event.status} />
        </Group>
        {cap.error && (
          <ErrorNotice
            error={cap.error}
            onRetry={() => void cap.refetch()}
            retryLabel="Retry loading sync permissions"
          />
        )}
        <Section title="Recorded result">
          <Group justify="space-between" gap="xs" align="start">
            <Group gap="sm">
              <Outcome status={original.status} />
              {original.malId > 0 && (
                <MALLink id={original.malId} title={original.malTitle} />
              )}
            </Group>
            <EventAge value={input.receivedAt} />
          </Group>
          {original.errorMessage && (
            <Text size="sm">{original.errorMessage}</Text>
          )}
          <Group gap="xs">
            <Text size="xs" className={classes.meta}>
              {input.source?.toUpperCase() || "Unknown source"}{" "}
              {input.sourceId || "ID unavailable"} ·{" "}
              {input.mediaType === "movie"
                ? "Movie"
                : `S${input.season} · E${input.episode}`}
            </Text>
            {original.errorCode && (
              <Text component="code" size="xs">
                {original.errorCode}
              </Text>
            )}
          </Group>
          {data.legacy && (
            <Text size="xs" className={classes.meta}>
              Historical decision trace unavailable. This event predates
              detailed traces; the recorded result is preserved.
            </Text>
          )}
          {!data.legacy &&
            data.attempts.find((a) => a.kind === "live")?.preview
              ?.resolution && (
              <details className={classes.disclosure}>
                <summary>Recorded matching details</summary>
                <ResolutionTrace
                  resolution={
                    data.attempts.find((a) => a.kind === "live")!.preview!
                      .resolution
                  }
                />
              </details>
            )}
        </Section>
        <div className={classes.detailGrid}>
          <Section title="Preview with current data">
            <Text size="sm" c="dimmed">
              Preview reads the current mapping and MAL list state. It makes no
              changes to your MAL list.
            </Text>
            {!data.replayEligible && (
              <Alert color="yellow" title="Preview or replay unavailable">
                {data.replayReason ||
                  "The original source information required to replay this event was not saved."}
              </Alert>
            )}
            <Group gap="sm" className={classes.actions}>
              <Button
                loading={previewMutation.isPending}
                disabled={!cap.data || !data.replayEligible || busy || editor}
                onClick={() => previewMutation.mutate(undefined)}
              >
                {preview ? "Preview again" : "Preview now"}
              </Button>
              <Button
                variant="subtle"
                ref={correctionButtonRef}
                disabled={!data.replayEligible || busy || editor}
                onClick={() => setEditor(true)}
              >
                Choose a different MAL entry
              </Button>
              <Button
                variant="subtle"
                loading={refresh.isPending}
                disabled={!cap.data || busy}
                onClick={() => refresh.mutate()}
              >
                Refresh mapping data
              </Button>
            </Group>
            {correction && (
              <Alert color="blue" title="Correction: this event only">
                <Stack gap="xs">
                  <Text size="sm">
                    MAL {correction.malId} · episode {correction.episode}.
                    Future events keep their current mappings.
                  </Text>
                  <Button
                    size="xs"
                    variant="subtle"
                    onClick={() => changeCorrection(undefined)}
                  >
                    Use automatic matching
                  </Button>
                </Stack>
              </Alert>
            )}
            {editor && (
              <CorrectionEditor
                data={data}
                csrf={cap.data?.csrfToken}
                onSelect={(value) => {
                  changeCorrection(value);
                  setEditor(false);
                  previewMutation.mutate(value);
                }}
                onSaved={() => {
                  setStale(true);
                  draftVersion.current++;
                  invalidate();
                }}
                onCancel={() => {
                  setEditor(false);
                  requestAnimationFrame(() =>
                    correctionButtonRef.current?.focus(),
                  );
                }}
              />
            )}
            <div aria-live="polite">
              {previewMutation.isPending && (
                <Text size="sm" role="status">
                  Reading current matching and MAL list state…
                </Text>
              )}
              {refresh.isSuccess && (
                <Alert color="green">
                  Mapping data refreshed. Preview again to evaluate it; no
                  events were retried.
                </Alert>
              )}
              {refresh.error && <ErrorNotice error={refresh.error} />}{" "}
              {previewMutation.error && (
                <ErrorNotice error={previewMutation.error} />
              )}
            </div>
            {preview && (
              <Stack
                gap="sm"
                ref={evaluationRef}
                tabIndex={-1}
                aria-label="Current evaluation"
              >
                <Divider />
                <Group justify="space-between">
                  <Group gap="xs">
                    <Text fw={600}>
                      {editor
                        ? "Previous evaluation · reference only"
                        : "Current evaluation"}
                    </Text>
                    <Badge
                      className={
                        editor
                          ? undefined
                          : stale || expired || !preview.ready
                            ? classes.attentionBadge
                            : classes.successBadge
                      }
                      color={
                        editor
                          ? "gray"
                          : stale || expired
                            ? "yellow"
                            : !preview.ready
                              ? "yellow"
                              : "green"
                      }
                      variant="light"
                    >
                      {editor
                        ? "Previous preview"
                        : stale
                          ? "State changed"
                          : expired
                            ? "Expired"
                            : !preview.ready
                              ? "Blocked"
                              : preview.changes?.length
                                ? "Ready to review"
                                : "No change"}
                    </Badge>
                  </Group>
                  <ReadableTime value={preview.createdAt} />
                </Group>
                {(stale || expired) && (
                  <Alert
                    color="yellow"
                    title={
                      stale
                        ? "State changed — preview again"
                        : "Preview expired"
                    }
                  >
                    This preview is retained for reference. Create a new preview
                    and review it before applying.
                  </Alert>
                )}
                {preview.resolution && (
                  <ResolutionTrace
                    summaryOnly
                    resolution={{
                      ...preview.resolution,
                      title: preview.before?.title || preview.resolution.title,
                    }}
                  />
                )}
                <Text size="sm" className={classes.target}>
                  {input.mediaType === "movie"
                    ? "Source movie"
                    : `Source S${input.season} E${input.episode}`}{" "}
                  →{" "}
                  {preview.resolution?.malId
                    ? `MAL ${preview.resolution.malId}${input.event?.includes("rate") ? ` · Rating ${input.rating}/10` : input.mediaType === "movie" ? " · Movie" : ` · E${preview.resolution.episode}`}`
                    : "No destination resolved"}
                </Text>
                <ChangeDiff preview={preview} />
                {original.malId > 0 &&
                  preview.resolution?.malId > 0 &&
                  original.malId !== preview.resolution.malId && (
                    <Alert color="yellow">
                      Applying this correction leaves MAL {original.malId}{" "}
                      unchanged.
                    </Alert>
                  )}
                {!preview.ready && (
                  <Alert color="yellow" title="Cannot apply this preview">
                    {preview.message || preview.code}
                    <SignInAction code={preview.code} />
                  </Alert>
                )}
                {preview.before && (
                  <Text size="sm" c="dimmed">
                    MAL progress at preview:{" "}
                    {preview.before.episodesWatched ?? "unknown"} /{" "}
                    {preview.before.totalEpisodes || "unknown"} episodes ·{" "}
                    {preview.before.status
                      ? preview.before.status
                          .replaceAll("_", " ")
                          .replace(/^./, (char) => char.toUpperCase())
                      : "Status not set"}{" "}
                    · score {preview.before.score || "not set"}
                  </Text>
                )}

                {preview.warnings
                  ?.filter(
                    (warning) =>
                      !(
                        warning ===
                          "MAL writes are disabled on this instance. You can inspect, preview, and save local rules." &&
                        preview.ready &&
                        preview.changes?.length > 0 &&
                        (!cap.data?.writesEnabled || !preview.writesEnabled)
                      ),
                  )
                  .map((warning, index) => (
                    <Alert key={index} color="yellow">
                      {warning}
                    </Alert>
                  ))}
                <Text size="xs" c="dimmed">
                  Approval expires{" "}
                  {preview.expiresAt
                    ? new Date(preview.expiresAt).toLocaleTimeString()
                    : "—"}
                </Text>
                {preview.ready && preview.changes?.length > 0 && (
                  <Stack gap="xs">
                    <Button
                      variant="filled"
                      disabled={
                        !cap.data?.writesEnabled ||
                        !preview.writesEnabled ||
                        stale ||
                        expired ||
                        busy ||
                        !!job ||
                        editor ||
                        previewMutation.isPending ||
                        refresh.isPending
                      }
                      loading={apply.isPending}
                      onClick={() => apply.mutate()}
                    >
                      {applyLabel}
                    </Button>
                    {editor && (
                      <Text size="sm" c="dimmed">
                        Close the correction editor or select a correction, then
                        review a fresh preview before applying.
                      </Text>
                    )}
                    {(!cap.data?.writesEnabled || !preview.writesEnabled) && (
                      <Text size="sm" c="dimmed">
                        Applying is disabled on this instance. This preview has
                        not changed MAL.
                      </Text>
                    )}
                  </Stack>
                )}
                {preview.resolution && (
                  <details className={classes.disclosure}>
                    <summary>How this match was found</summary>
                    <Stack mt="sm">
                      <ResolutionTrace
                        detailsOnly
                        resolution={preview.resolution}
                      />
                    </Stack>
                  </details>
                )}
                {preview.accountId && (
                  <details>
                    <summary>MAL account binding</summary>
                    <Text
                      size="xs"
                      c="dimmed"
                      style={{ overflowWrap: "anywhere" }}
                    >
                      {preview.accountId}
                    </Text>
                  </details>
                )}
              </Stack>
            )}
            {apply.error && <ErrorNotice error={apply.error} />}{" "}
            {polling.error && <ErrorNotice error={polling.error} />}{" "}
            {shownJob && (
              <Alert
                color={
                  shownJob.status === "synced" ||
                  shownJob.status === "no_change"
                    ? "green"
                    : "blue"
                }
                title="Recovery attempt"
                role="status"
              >
                <Stack gap="xs">
                  <Outcome status={shownJob.status} />
                  <Text size="sm">
                    {shownJob.message ||
                      (["queued", "processing"].includes(shownJob.status)
                        ? "The server is processing this durable job. You can leave this page safely."
                        : "The result has been saved in this event’s history.")}
                  </Text>
                  <Text size="xs">Job {shownJob.id}</Text>
                </Stack>
              </Alert>
            )}
          </Section>
          <aside aria-label="Received event context">
            <Section title="Received">
              <dl className={classes.facts}>
                <Fact
                  label="Original source"
                  value={`${input.source?.toUpperCase() || "Unavailable"} ${input.sourceId || "ID unavailable"}`}
                />
                <Fact
                  label="Source episode"
                  value={
                    input.mediaType === "movie"
                      ? "Movie"
                      : `Season ${input.season} · Episode ${input.episode}`
                  }
                />
                <Fact
                  label="Activity"
                  value={`${input.event}${input.event?.includes("rate") ? ` · Rating ${input.rating}/10` : ""}`}
                />
                <Fact
                  label="Plex context"
                  value={`${input.account || "Unknown account"} / ${input.library || "Unknown library"}`}
                />
                <div className={classes.fact}>
                  <dt>Received at</dt>
                  <dd>
                    <ReadableTime value={input.receivedAt} />
                  </dd>
                </div>
              </dl>
            </Section>
          </aside>
        </div>
        <Section title="Attempts">
          {data.attempts.length === 0 ? (
            <Text size="sm" c="dimmed">
              No detailed attempts were saved for this historical event. The
              recorded result remains above.
            </Text>
          ) : (
            [...data.attempts].reverse().map((attempt) => (
              <Paper withBorder p="sm" key={attempt.id}>
                <Stack gap="xs">
                  <Group justify="space-between">
                    <Text fw={600} size="sm">
                      {attempt.kind === "live"
                        ? "Original sync"
                        : attempt.kind === "replay"
                          ? "Recovery attempt"
                          : attempt.kind.replaceAll("_", " ")}
                    </Text>
                    <Outcome status={attempt.status} />
                  </Group>
                  <ReadableTime value={attempt.createdAt} />
                  {attempt.message && <Text size="sm">{attempt.message}</Text>}
                  {attempt.code && (
                    <Text size="xs" c="dimmed">
                      {attempt.code}
                    </Text>
                  )}
                  {(attempt.preview || attempt.observed) && (
                    <details className={classes.disclosure}>
                      <summary>View saved plan and MAL observation</summary>
                      <Stack mt="sm" gap="sm">
                        {attempt.preview?.resolution && (
                          <ResolutionTrace
                            resolution={attempt.preview.resolution}
                          />
                        )}
                        {attempt.observed && (
                          <ObservedResult
                            state={attempt.observed}
                            at={attempt.observedAt}
                            source={attempt.observationSource}
                          />
                        )}
                        {attempt.preview && (
                          <Stack gap="xs">
                            <Text size="xs" c="dimmed">
                              Saved plan evaluated at{" "}
                              {new Date(
                                attempt.preview.createdAt,
                              ).toLocaleString()}
                              . These values are not a fresh MAL read.
                            </Text>
                            <ChangeDiff preview={attempt.preview} recorded />
                          </Stack>
                        )}
                      </Stack>
                    </details>
                  )}
                </Stack>
              </Paper>
            ))
          )}
        </Section>
      </Stack>
    </Container>
  );
}
function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className={classes.fact}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}
function CorrectionEditor({
  data,
  csrf,
  onSelect,
  onSaved,
  onCancel,
}: {
  data: EventDetail;
  csrf?: string;
  onSelect: (value: { malId: number; episode: number }) => void;
  onSaved: () => void;
  onCancel: () => void;
}) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<MALResult | null>(null);
  const episodeInputRef = useRef<HTMLInputElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const previewCorrectionRef = useRef<HTMLButtonElement>(null);
  const changeEntryRef = useRef<HTMLButtonElement>(null);
  const ruleHeadingRef = useRef<HTMLHeadingElement>(null);
  const focusIntent = useRef<"search" | "review" | "rule" | null>(null);
  const [episode, setEpisode] = useState<number | string>(
    data.input.episode || 1,
  );
  const [rule, setRule] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [discardRuleOnly, setDiscardRuleOnly] = useState(false);
  useEffect(() => {
    const intent = focusIntent.current;
    if (!intent) return;
    focusIntent.current = null;
    if (intent === "search") {
      searchInputRef.current?.focus();
    } else if (intent === "rule") {
      ruleHeadingRef.current?.focus();
    } else if (episodeInputRef.current && !episodeInputRef.current.disabled) {
      episodeInputRef.current.focus();
    } else if (
      previewCorrectionRef.current &&
      !previewCorrectionRef.current.disabled
    ) {
      previewCorrectionRef.current.focus();
    } else {
      changeEntryRef.current?.focus();
    }
  }, [selected, rule, confirmDiscard]);
  const [anyServer, setAnyServer] = useState(false);
  const [start, setStart] = useState<number | string>(
    data.input.mediaType === "movie" ? 1 : 1,
  );
  const [end, setEnd] = useState<number | string>(
    data.input.mediaType === "movie" ? 1 : data.input.episode || 1,
  );
  const [target, setTarget] = useState<number | string>(1);
  const search = useMutation({
    mutationFn: () => syncAPI.search(query.trim()),
  });
  const ruleBody = {
    ...(!data.input.serverId ? { anyServer } : {}),
    eventId: data.event.id,
    malId: selected?.malId || 0,
    sourceEpisodeStart: Number(start),
    sourceEpisodeEnd: Number(end),
    malEpisodeStart: Number(target),
  };
  const [impactDraft, setImpactDraft] = useState("");
  const impact = useMutation({
    mutationFn: (body: typeof ruleBody) => syncAPI.previewRule(body, csrf!),
    onSuccess: (_data, body) => setImpactDraft(JSON.stringify(body)),
  });
  const save = useMutation({
    mutationFn: () =>
      syncAPI.saveRule(
        { ...ruleBody, impactFingerprint: impact.data!.fingerprint },
        csrf!,
      ),
    onSuccess: () => {
      onSaved();
      focusIntent.current = "review";
      setRule(false);
    },
  });
  const validEpisode =
    Number.isSafeInteger(Number(episode)) &&
    Number(episode) > 0 &&
    (!selected?.totalEpisodes || Number(episode) <= selected.totalEpisodes);
  const validRange =
    Number(start) > 0 &&
    Number(end) >= Number(start) &&
    Number(target) > 0 &&
    [start, end, target].every((v) => Number.isSafeInteger(Number(v))) &&
    (!selected?.totalEpisodes ||
      Number(target) + Number(end) - Number(start) <= selected.totalEpisodes);
  const allowRule =
    data.input.source !== "myanimelist" &&
    !!data.input.sourceId &&
    !!data.input.account &&
    !!data.input.library &&
    (data.input.source !== "anidb" || data.input.season === 1);
  return (
    <Paper withBorder p="md" className={classes.editor}>
      <Stack gap="md">
        <RuleDraftGuard dirty={rule} onDiscard={onCancel} />
        <Group justify="space-between" align="start">
          <div>
            <Text
              component="h3"
              ref={ruleHeadingRef}
              tabIndex={-1}
              size="md"
              fw={700}
              m={0}
            >
              {rule ? "Create a persistent local rule" : "Correct this event"}
            </Text>
            <Badge mt="xs" variant="light" color={rule ? "blue" : "gray"}>
              {rule ? "Future matching events" : "This event only"}
            </Badge>
          </div>
          <Button
            size="xs"
            variant="subtle"
            onClick={() => {
              setDiscardRuleOnly(false);
              if (rule) setConfirmDiscard(true);
              else onCancel();
            }}
          >
            Close editor
          </Button>
        </Group>
        {confirmDiscard && (
          <Alert color="yellow" title="Discard this unsaved rule draft?">
            <Group mt="sm">
              <Button
                color="red"
                onClick={() => {
                  if (discardRuleOnly) {
                    focusIntent.current = "review";
                    setRule(false);
                    setConfirmDiscard(false);
                  } else onCancel();
                }}
              >
                Discard rule draft
              </Button>
              <Button
                onClick={() => {
                  focusIntent.current = "rule";
                  setConfirmDiscard(false);
                }}
              >
                Keep editing
              </Button>
            </Group>
          </Alert>
        )}
        {!rule && (
          <Text size="sm">
            {data.input.mediaType === "movie"
              ? "Movie"
              : `Source S${data.input.season} · E${data.input.episode}`}{" "}
            · {data.input.source.toUpperCase()} {data.input.sourceId}. Future
            mappings stay unchanged.
          </Text>
        )}
        {!selected && (
          <>
            <Text className={classes.stepLabel}>1 · Choose a destination</Text>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setSelected(null);
                search.mutate();
              }}
            >
              <Group align="end">
                <TextInput
                  style={{ flex: 1, minWidth: 180 }}
                  label="MAL title or numeric ID"
                  ref={searchInputRef}
                  value={query}
                  onChange={(e) => setQuery(e.currentTarget.value)}
                />
                <Button
                  type="submit"
                  disabled={!query.trim()}
                  loading={search.isPending}
                >
                  Search MAL entries
                </Button>
              </Group>
            </form>
            {search.error && <ErrorNotice error={search.error} />}{" "}
            {search.data?.data.length === 0 && (
              <Text size="sm">No matching titles. Try a numeric MAL ID.</Text>
            )}
            <div aria-live="polite">
              {search.isPending && (
                <Text size="sm">Searching MAL entries…</Text>
              )}
            </div>
            {search.data?.data.map((result) => (
              <Paper key={result.malId} withBorder p="sm">
                <Group justify="space-between" align="start">
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <Text
                      size="sm"
                      fw={600}
                      style={{ overflowWrap: "anywhere" }}
                    >
                      {result.title}
                    </Text>
                    <Text size="sm" c="dimmed">
                      MAL {result.malId} ·{" "}
                      {result.mediaType || "Type unavailable"} ·{" "}
                      {result.totalEpisodes || "Unknown"} episodes
                    </Text>
                  </div>
                  <Button
                    onClick={() => {
                      focusIntent.current = "review";
                      setSelected(result);
                      setRule(false);
                    }}
                  >
                    Select entry
                  </Button>
                </Group>
              </Paper>
            ))}
          </>
        )}
        {selected && (
          <Stack>
            <div className={classes.target}>
              <Group justify="space-between" gap="xs">
                <Text className={classes.stepLabel}>Selected destination</Text>
                {!rule && (
                  <Button
                    size="compact-xs"
                    variant="subtle"
                    ref={changeEntryRef}
                    onClick={() => {
                      focusIntent.current = "search";
                      setSelected(null);
                    }}
                  >
                    Change entry
                  </Button>
                )}
              </Group>
              <Text size="sm" mt={4}>
                <MALLink id={selected.malId} title={selected.title} />
              </Text>
              <Text size="xs" mt={4}>
                {selected.mediaType || "Type unavailable"} ·{" "}
                {selected.totalEpisodes || "Unknown"} episodes
              </Text>
            </div>
            {!rule && (
              <>
                <Text className={classes.stepLabel}>
                  2 · Review this event’s correction
                </Text>
                <NumberInput
                  label="Target MAL episode"
                  ref={episodeInputRef}
                  min={1}
                  max={
                    data.input.mediaType === "movie"
                      ? 1
                      : selected.totalEpisodes || undefined
                  }
                  value={episode}
                  disabled={data.input.mediaType === "movie"}
                  onChange={setEpisode}
                  allowDecimal={false}
                />
                <Text size="sm">
                  {data.input.mediaType === "movie"
                    ? "Source movie"
                    : `Source episode ${data.input.episode}`}{" "}
                  → MAL {selected.malId}
                  {data.input.event.includes("rate")
                    ? ` · Rating ${data.input.rating}/10`
                    : ` · episode ${episode}`}
                  . Next, review a fresh preview before applying.
                </Text>
                <Group className={classes.actions}>
                  <Button
                    ref={previewCorrectionRef}
                    disabled={!validEpisode || !csrf}
                    onClick={() =>
                      onSelect({
                        malId: selected.malId,
                        episode: Number(episode),
                      })
                    }
                  >
                    Preview correction
                  </Button>
                </Group>
                <Divider />
                <div>
                  <Text size="sm" fw={600}>
                    Apply this mapping to future events?
                  </Text>
                  <Text size="xs" mt={4}>
                    A local rule is separate from this event correction and
                    requires its own scope review.
                  </Text>
                </div>
                <Button
                  variant="subtle"
                  style={{ alignSelf: "start" }}
                  onClick={() => {
                    focusIntent.current = "rule";
                    setRule(true);
                  }}
                >
                  Save as a local rule…
                </Button>
              </>
            )}
            {rule && (
              <Stack>
                <Button
                  variant="subtle"
                  size="compact-xs"
                  style={{ alignSelf: "start" }}
                  onClick={() => {
                    setDiscardRuleOnly(true);
                    setConfirmDiscard(true);
                  }}
                >
                  ← Back to event correction
                </Button>
                <Text className={classes.stepLabel}>
                  1 · Define scope and conversion
                </Text>
                <Text fw={600}>Review a persistent local rule</Text>
                <Text size="sm">
                  {data.input.source.toUpperCase()} {data.input.sourceId} ·
                  season {data.input.season} · {data.input.account} /{" "}
                  {data.input.library}
                  {data.input.serverId
                    ? ` · server ${data.input.serverId}`
                    : ""}
                </Text>
                <RuleServerScope
                  serverId={data.input.serverId}
                  anyServer={anyServer}
                  onChange={setAnyServer}
                  account={data.input.account}
                  library={data.input.library}
                />
                {!allowRule && (
                  <Alert color="yellow">
                    This event lacks the stable identity required for a scoped
                    rule, or is an AniDB season beyond season one. Use an
                    event-only correction.
                  </Alert>
                )}
                <SimpleGrid cols={{ base: 1, md: 3 }}>
                  <NumberInput
                    label="Source episode start"
                    value={start}
                    onChange={setStart}
                    min={1}
                    disabled={data.input.mediaType === "movie"}
                    allowDecimal={false}
                  />
                  <NumberInput
                    label="Source episode end"
                    value={end}
                    onChange={setEnd}
                    min={Number(start) || 1}
                    disabled={data.input.mediaType === "movie"}
                    allowDecimal={false}
                  />
                  <NumberInput
                    label="MAL episode start"
                    value={target}
                    onChange={setTarget}
                    min={1}
                    disabled={data.input.mediaType === "movie"}
                    allowDecimal={false}
                  />
                </SimpleGrid>
                <Paper withBorder p="sm">
                  <Text size="sm" fw={600}>
                    Source episodes {start}–{end} → MAL {selected.malId},
                    episodes {target}–
                    {Number(target) + Number(end) - Number(start)}.
                  </Text>
                  <Text size="sm" mt={4}>
                    This event: source episode {data.input.episode} → MAL
                    episode{" "}
                    {Number(target) + data.input.episode - Number(start)}
                    {data.input.episode < Number(start) ||
                    data.input.episode > Number(end)
                      ? " (outside this rule’s range)"
                      : ""}
                    .
                  </Text>
                  <Text size="sm">
                    Saving changes matching for future events. Existing events
                    are not retried. This rule is personal and is not published
                    to the community.
                  </Text>
                </Paper>
                {!validRange && (
                  <Text c="red" size="sm">
                    Enter a valid positive range within the selected MAL title’s
                    episode count.
                  </Text>
                )}
                <Text size="sm" c="dimmed">
                  {data.overrides.filter((r) => r.active).length} active rules
                  are attached to this event. The server rejects overlapping
                  ranges.
                </Text>
                {impact.error && <ErrorNotice error={impact.error} />}
                <Text className={classes.stepLabel}>
                  2 · Review impact before saving
                </Text>
                <Button
                  disabled={!csrf || !allowRule || !validRange}
                  loading={impact.isPending}
                  onClick={() => impact.mutate({ ...ruleBody })}
                >
                  Review rule impact
                </Button>
                {impact.data && impactDraft === JSON.stringify(ruleBody) && (
                  <Alert
                    color={impact.data.ready ? "blue" : "yellow"}
                    title="Rule impact"
                  >
                    <Text size="sm">
                      Server scope:{" "}
                      {impact.data.scope.anyServer
                        ? `All servers · ${data.input.account} / ${data.input.library}`
                        : impact.data.scope.serverId || "Not selected"}
                    </Text>
                    <Text size="sm">
                      {impact.data.affectedEventIds.length} stored events match
                      this scope. They will not be retried.
                    </Text>
                    {impact.data.affectedEventIds.length > 0 && (
                      <Text size="xs">
                        Event IDs: {impact.data.affectedEventIds.join(", ")}
                      </Text>
                    )}
                    {impact.data.conflicts.map((conflict) => (
                      <Text key={conflict.id} size="sm">
                        Conflict with revision {conflict.revision}:{" "}
                        {conflict.source.toUpperCase()} {conflict.sourceId},
                        episodes {conflict.sourceEpisodeStart}–
                        {conflict.sourceEpisodeEnd} → MAL {conflict.malId}
                      </Text>
                    ))}
                    {impact.data.warnings.map((warning, index) => (
                      <Text key={index} size="sm">
                        {warning}
                      </Text>
                    ))}
                  </Alert>
                )}
                {save.error && <ErrorNotice error={save.error} />}
                <Button
                  loading={save.isPending}
                  disabled={
                    !csrf ||
                    !allowRule ||
                    !validRange ||
                    (!data.input.serverId && !anyServer) ||
                    !impact.data?.ready ||
                    impactDraft !== JSON.stringify(ruleBody)
                  }
                  onClick={() => save.mutate()}
                >
                  Save local rule
                </Button>
              </Stack>
            )}
            {save.isSuccess && (
              <Alert color="green" role="status">
                Local rule saved. Preview this event again to review the new
                mapping.
              </Alert>
            )}
          </Stack>
        )}
      </Stack>
    </Paper>
  );
}
