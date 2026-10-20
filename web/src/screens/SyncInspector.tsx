import { syncKeys, statusLabels } from "@components/sync/values";
import { useState } from "react";
import {
  Alert,
  Button,
  Container,
  Group,
  Pagination,
  Paper,
  NumberInput,
  SimpleGrid,
  Select,
  Skeleton,
  Stack,
  Tabs,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { SyncInspectorRoute } from "@app/routes";
import { syncAPI, type LocalRule } from "@api/sync";
import {
  ErrorNotice,
  MALLink,
  Outcome,
  ReadableTime,
  RuleServerScope,
} from "@components/sync/shared";

export function SyncInspector() {
  const search = SyncInspectorRoute.useSearch();
  const navigate = SyncInspectorRoute.useNavigate();
  const queryClient = useQueryClient();
  const [refreshedAt, setRefreshedAt] = useState("");
  const cap = useQuery({
    queryKey: syncKeys.capabilities,
    queryFn: syncAPI.capabilities,
  });
  const events = useQuery({
    queryKey: ["sync", "events", search],
    queryFn: () =>
      syncAPI.events({
        limit: 25,
        offset: ((search.page || 1) - 1) * 25,
        search: search.q || "",
        status: search.status || "",
        source: search.source || "",
        event: search.event || "",
      }),
    enabled: search.tab !== "rules",
  });
  const refresh = useMutation({
    mutationFn: () => syncAPI.refresh(cap.data!.csrfToken),
    onSuccess: (data) => {
      setRefreshedAt(data.refreshedAt);
      void queryClient.invalidateQueries({ queryKey: syncKeys.all });
    },
  });
  const filters = (patch: Partial<typeof search>) =>
    void navigate({ search: (prev) => ({ ...prev, ...patch, page: 1 }) });
  return (
    <Container size={1200} py="lg">
      <Stack gap="lg">
        <Group justify="space-between">
          <div>
            <Title order={1} size="h2">
              Sync Inspector
            </Title>
            <Text c="dimmed" size="sm">
              Understand each MAL sync, preview a correction, and recover
              safely.
            </Text>
          </div>
          <Button
            loading={refresh.isPending}
            disabled={!cap.data}
            onClick={() => refresh.mutate()}
          >
            Refresh mapping data
          </Button>
        </Group>
        {cap.error && <ErrorNotice error={cap.error} />}
        <div aria-live="polite">
          {refresh.error && <ErrorNotice error={refresh.error} />}
          <Text size="xs" c="dimmed">
            {refreshedAt
              ? `Mapping data refreshed ${new Date(refreshedAt).toLocaleString()}. Existing events were not retried.`
              : "Refresh checks the catalogue and mapping cache. Refresh time unavailable."}
          </Text>
        </div>
        <Tabs
          value={search.tab || "events"}
          onChange={(value) =>
            filters({ tab: value === "rules" ? "rules" : "events" })
          }
        >
          <Tabs.List>
            <Tabs.Tab value="events">Events</Tabs.Tab>
            <Tabs.Tab value="rules">Local rules</Tabs.Tab>
          </Tabs.List>
          <Tabs.Panel value="events" pt="md">
            <Stack>
              <Group align="end" grow>
                <TextInput
                  label="Search events"
                  placeholder="Title, source ID or MAL ID"
                  value={search.q || ""}
                  onChange={(e) => filters({ q: e.currentTarget.value })}
                />
                <Select
                  label="MAL outcome"
                  clearable
                  placeholder="All outcomes"
                  data={Object.entries(statusLabels).map(([value, label]) => ({
                    value,
                    label,
                  }))}
                  value={search.status || null}
                  onChange={(v) => filters({ status: v || "" })}
                />
                <Select
                  label="Source provider"
                  clearable
                  placeholder="All providers"
                  data={["anidb", "tvdb", "tmdb", "myanimelist"].map((v) => ({
                    value: v,
                    label: v.toUpperCase(),
                  }))}
                  value={search.source || null}
                  onChange={(v) => filters({ source: v || "" })}
                />
              </Group>
              <Group>
                <Button
                  size="xs"
                  onClick={() => filters({ status: "needs_mapping" })}
                >
                  Needs mapping
                </Button>
                <Button
                  size="xs"
                  variant="subtle"
                  onClick={() =>
                    filters({ q: "", status: "", source: "", event: "" })
                  }
                >
                  Clear filters
                </Button>
                <Text size="sm" c="dimmed">
                  {events.data?.count ?? "…"} events
                </Text>
              </Group>
              {events.isPending ? (
                <Stack>
                  {[1, 2, 3].map((v) => (
                    <Skeleton key={v} height={100} />
                  ))}
                </Stack>
              ) : events.error ? (
                <Stack>
                  <ErrorNotice error={events.error} />
                  <Button onClick={() => void events.refetch()}>
                    Retry loading events
                  </Button>
                </Stack>
              ) : events.data?.data.length ? (
                events.data.data.map((event) => (
                  <Paper key={event.id} withBorder p="md" radius="md">
                    <Group justify="space-between" align="start" wrap="wrap">
                      <Stack gap={5} style={{ minWidth: 0, flex: 1 }}>
                        <Text fw={700} style={{ overflowWrap: "anywhere" }}>
                          {event.title || "Untitled event"}
                        </Text>
                        <Text size="sm">
                          {event.mediaType === "movie"
                            ? "Movie"
                            : `S${event.season} E${event.episode}`}
                          {event.event.includes("rate")
                            ? ` · Rating ${event.rating}/10`
                            : ""}{" "}
                          · {event.account || "Unknown account"} /{" "}
                          {event.library || "Unknown library"}
                        </Text>
                        <Group gap="xs">
                          <Outcome status={event.status} />
                          {event.lastAction &&
                            !["Original sync", "Received"].includes(
                              event.lastAction,
                            ) && (
                              <Text size="xs" c="dimmed">
                                Latest {event.lastAction.replaceAll("_", " ")}{" "}
                                attempt
                              </Text>
                            )}
                          {event.malId > 0 ? (
                            <MALLink id={event.malId} title={event.malTitle} />
                          ) : (
                            <Text size="sm" c="dimmed">
                              {event.source?.toUpperCase() || "Unknown source"}{" "}
                              {event.sourceId || "ID unavailable"}
                            </Text>
                          )}
                        </Group>
                        {event.errorMessage && (
                          <Text size="sm" c="dimmed">
                            {event.errorMessage}
                          </Text>
                        )}
                        <ReadableTime value={event.receivedAt} />
                      </Stack>
                      <Button
                        renderRoot={(props) => (
                          <Link
                            {...props}
                            to="/sync/events/$eventId"
                            params={{ eventId: String(event.id) }}
                            search={search}
                          />
                        )}
                      >
                        Inspect
                      </Button>
                    </Group>
                  </Paper>
                ))
              ) : (
                <Alert title="No events match these filters">
                  Clear the filters to see all received events. New Plex events
                  will appear here.
                </Alert>
              )}
              <Pagination
                total={Math.max(1, Math.ceil((events.data?.count || 0) / 25))}
                value={search.page || 1}
                onChange={(page) =>
                  void navigate({ search: (prev) => ({ ...prev, page }) })
                }
                aria-label="Event pages"
              />
            </Stack>
          </Tabs.Panel>
          <Tabs.Panel value="rules" pt="md">
            <LocalRules csrf={cap.data?.csrfToken} />
          </Tabs.Panel>
        </Tabs>
      </Stack>
    </Container>
  );
}
function LocalRules({ csrf }: { csrf?: string }) {
  const queryClient = useQueryClient();
  const [confirm, setConfirm] = useState<string | null>(null);
  const rules = useQuery({ queryKey: syncKeys.rules, queryFn: syncAPI.rules });
  const [editing, setEditing] = useState<LocalRule | null>(null);
  const update = useMutation({
    mutationFn: (rule: LocalRule) => syncAPI.updateRule(rule, csrf!),
    onSuccess: () => {
      setEditing(null);
      void queryClient.invalidateQueries({ queryKey: syncKeys.all });
    },
  });
  const disable = useMutation({
    mutationFn: (rule: LocalRule) =>
      syncAPI.disableRule(rule.id, rule.revision, csrf!),
    onSuccess: () => {
      setConfirm(null);
      void queryClient.invalidateQueries({ queryKey: syncKeys.all });
    },
  });
  return (
    <Stack>
      <Text size="sm" c="dimmed">
        Personal rules layer over your selected community or custom-file
        mappings. Create a rule from an event correction. Existing events are
        never retried automatically.
      </Text>
      <Button
        size="xs"
        loading={rules.isFetching}
        onClick={() => void rules.refetch()}
      >
        Reload rules
      </Button>
      {rules.isPending && <Skeleton height={100} />}{" "}
      {rules.error && <ErrorNotice error={rules.error} />}{" "}
      {disable.error && <ErrorNotice error={disable.error} />}{" "}
      {update.error && <ErrorNotice error={update.error} />}{" "}
      {rules.data?.data.length === 0 && (
        <Alert title="No local rules yet">
          Inspect an event and choose a MAL entry to save a scoped rule.
        </Alert>
      )}
      {rules.data?.data.map((rule) => (
        <Paper key={rule.id} withBorder p="md">
          <Stack gap="sm">
            <Group justify="space-between">
              <Text fw={600}>
                {rule.source.toUpperCase()} {rule.sourceId} · season{" "}
                {rule.season}
              </Text>
              <Group>
                <Outcome status={rule.active ? "enabled" : "disabled"} />
                <Text size="xs">Revision {rule.revision}</Text>
              </Group>
            </Group>
            <Text size="sm">
              Source episodes {rule.sourceEpisodeStart}–{rule.sourceEpisodeEnd}{" "}
              → <MALLink id={rule.malId} /> · MAL episodes{" "}
              {rule.malEpisodeStart}–
              {rule.malEpisodeStart +
                rule.sourceEpisodeEnd -
                rule.sourceEpisodeStart}
            </Text>
            <Text size="sm" c="dimmed">
              {rule.account} / {rule.library}
              {rule.anyServer
                ? " · All servers"
                : rule.serverId
                  ? ` · Server ${rule.serverId}`
                  : " · Server scope not selected"}
            </Text>
            <ReadableTime value={rule.createdAt} />
            <Group>
              <Button
                renderRoot={(props) => (
                  <Link
                    {...props}
                    to="/sync/events/$eventId"
                    params={{ eventId: String(rule.eventId) }}
                  />
                )}
              >
                Inspect source event
              </Button>
              <Button disabled={!csrf} onClick={() => setEditing({ ...rule })}>
                Edit rule
              </Button>
              {!rule.active && (
                <Button
                  disabled={!csrf}
                  loading={update.isPending}
                  onClick={() => setEditing({ ...rule, active: true })}
                >
                  Enable rule
                </Button>
              )}
              {rule.active && (
                <Button
                  color="red"
                  disabled={!csrf}
                  onClick={() => setConfirm(rule.id)}
                >
                  Disable rule
                </Button>
              )}
            </Group>
            {editing?.id === rule.id && (
              <RuleEditor
                draft={editing}
                onChange={setEditing}
                csrf={csrf}
                pending={update.isPending}
                onSave={(fingerprint) =>
                  update.mutate({ ...editing, impactFingerprint: fingerprint })
                }
                onCancel={() => setEditing(null)}
              />
            )}
            {confirm === rule.id && (
              <Alert
                color="yellow"
                title={`Disable rule for MAL ${rule.malId}?`}
              >
                <Stack gap="sm">
                  <Text size="sm">
                    Future matches return to the selected base mappings.
                    Previous MAL updates remain unchanged. This revision stays
                    in history.
                  </Text>
                  <Group>
                    <Button
                      color="red"
                      loading={disable.isPending}
                      onClick={() => disable.mutate(rule)}
                    >
                      Confirm disable
                    </Button>
                    <Button onClick={() => setConfirm(null)}>Keep rule</Button>
                  </Group>
                </Stack>
              </Alert>
            )}
          </Stack>
        </Paper>
      ))}
    </Stack>
  );
}

function RuleEditor({
  draft,
  onChange,
  csrf,
  pending,
  onSave,
  onCancel,
}: {
  draft: LocalRule;
  onChange: (rule: LocalRule) => void;
  csrf?: string;
  pending: boolean;
  onSave: (fingerprint: string) => void;
  onCancel: () => void;
}) {
  const [reviewed, setReviewed] = useState("");
  const impact = useMutation({
    mutationFn: async (body: LocalRule) => {
      const [impact, results] = await Promise.all([
        syncAPI.previewRule(body, csrf!),
        syncAPI.search(String(body.malId)),
      ]);
      return {
        ...impact,
        validatedTitle: results.data.find(
          (result) => result.malId === body.malId,
        )?.title,
      };
    },
    onSuccess: (_data, body) => setReviewed(JSON.stringify(body)),
  });
  const valid =
    [
      draft.malId,
      draft.sourceEpisodeStart,
      draft.sourceEpisodeEnd,
      draft.malEpisodeStart,
    ].every((v) => Number.isSafeInteger(v) && v > 0) &&
    draft.sourceEpisodeEnd >= draft.sourceEpisodeStart;
  const conflicts =
    impact.data?.conflicts.filter((rule) => rule.id !== draft.id) || [];
  const field = (
    key:
      "malId" | "sourceEpisodeStart" | "sourceEpisodeEnd" | "malEpisodeStart",
    label: string,
  ) => (
    <NumberInput
      label={label}
      value={draft[key]}
      min={1}
      allowDecimal={false}
      onChange={(value) => onChange({ ...draft, [key]: Number(value) })}
    />
  );
  return (
    <Paper withBorder p="md">
      <Stack>
        <Text fw={600}>Edit revision {draft.revision}</Text>
        <RuleServerScope
          serverId={draft.serverId}
          anyServer={draft.anyServer ?? false}
          onChange={(anyServer) => onChange({ ...draft, anyServer })}
          account={draft.account}
          library={draft.library}
        />
        <SimpleGrid cols={{ base: 1, sm: 2 }}>
          {field("malId", "Destination MAL ID")}
          {field("sourceEpisodeStart", "Source episode start")}
          {field("sourceEpisodeEnd", "Source episode end")}
          {field("malEpisodeStart", "MAL episode start")}
        </SimpleGrid>
        <Text size="sm">
          Source episodes {draft.sourceEpisodeStart}–{draft.sourceEpisodeEnd} →
          MAL {draft.malId}, episodes {draft.malEpisodeStart}–
          {draft.malEpisodeStart +
            draft.sourceEpisodeEnd -
            draft.sourceEpisodeStart}
          . Existing events are not retried.
        </Text>
        {impact.error && <ErrorNotice error={impact.error} />}
        <Button
          disabled={!valid || !csrf}
          loading={impact.isPending}
          onClick={() => impact.mutate({ ...draft })}
        >
          Review edited rule
        </Button>
        {impact.data && reviewed === JSON.stringify(draft) && (
          <Alert
            color={!impact.data.ready || conflicts.length ? "yellow" : "blue"}
          >
            {impact.data.validatedTitle && (
              <MALLink id={draft.malId} title={impact.data.validatedTitle} />
            )}
            <Text size="sm">
              Server scope:{" "}
              {impact.data.scope.anyServer
                ? `All servers · ${draft.account} / ${draft.library}`
                : impact.data.scope.serverId || "Not selected"}
            </Text>
            <Text size="sm">
              {impact.data.affectedEventIds.length} matching stored events;{" "}
              {conflicts.length} conflicting rules.
            </Text>
            {impact.data.warnings.map((warning, index) => (
              <Text key={index} size="sm">
                {warning}
              </Text>
            ))}
            {conflicts.map((rule) => (
              <Text key={rule.id} size="sm">
                MAL {rule.malId} · episodes {rule.sourceEpisodeStart}–
                {rule.sourceEpisodeEnd}
              </Text>
            ))}
          </Alert>
        )}
        <Group>
          <Button
            disabled={
              !valid ||
              !csrf ||
              !impact.data?.ready ||
              (!draft.serverId && !draft.anyServer) ||
              reviewed !== JSON.stringify(draft) ||
              conflicts.length > 0
            }
            loading={pending}
            onClick={() => onSave(impact.data!.fingerprint)}
          >
            Save rule revision
          </Button>
          <Button onClick={onCancel}>Discard edits</Button>
        </Group>
      </Stack>
    </Paper>
  );
}
