import { syncKeys, statusLabels } from "@components/sync/values";
import { useState } from "react";
import {
  Alert,
  Anchor,
  Badge,
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
import { EventList } from "@components/sync/EventList";
import { RuleDraftGuard } from "@components/sync/RuleDraftGuard";
import classes from "@components/sync/SyncInspector.module.css";
import { FaSearch, FaSyncAlt } from "react-icons/fa";

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
  const filtered = !!(
    search.q ||
    search.status ||
    search.source ||
    search.event
  );
  const clearFilters = () =>
    filters({ q: "", status: "", source: "", event: "" });
  const count = events.data?.count;
  const start = ((search.page || 1) - 1) * 25 + 1;
  return (
    <Container size={1200} px="md" component="main" className={classes.page}>
      <Stack gap="md" mt="md" pb="lg">
        <div>
          <Group justify="space-between" align="start" gap="sm">
            <div>
              <Title order={2}>Sync Inspector</Title>
              <Text size="sm" mt={4}>
                Understand sync results, preview corrections, and recover
                safely.
              </Text>
            </div>
            <Button
              size="xs"
              className={classes.touchUtility}
              leftSection={<FaSyncAlt size={11} />}
              loading={refresh.isPending}
              disabled={!cap.data}
              onClick={() => refresh.mutate()}
            >
              Refresh mapping data
            </Button>
          </Group>
          <div aria-live="polite">
            {refresh.error && <ErrorNotice error={refresh.error} />}
            {refreshedAt && (
              <Text size="sm">
                Mapping data refreshed {new Date(refreshedAt).toLocaleString()}.
                Existing events were not retried.
              </Text>
            )}
          </div>
        </div>
        {cap.error && (
          <ErrorNotice
            error={cap.error}
            onRetry={() => void cap.refetch()}
            retryLabel="Retry loading sync permissions"
          />
        )}
        <Paper withBorder p="md" className={classes.workspace}>
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
              <Stack gap="md">
                <div className={classes.filters}>
                  <TextInput
                    label="Search events"
                    placeholder="Title, source ID or MAL ID"
                    leftSection={<FaSearch size={13} />}
                    value={search.q || ""}
                    onChange={(e) => filters({ q: e.currentTarget.value })}
                  />
                  <Select
                    label="MAL outcome"
                    clearable
                    placeholder="All outcomes"
                    data={Object.entries(statusLabels).map(
                      ([value, label]) => ({ value, label }),
                    )}
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
                </div>
                <Group justify="space-between" gap="xs">
                  <Group gap="xs" className={classes.touchUtilities}>
                    <Button
                      size="compact-xs"
                      variant={
                        search.status === "needs_mapping" ? "filled" : "outline"
                      }
                      aria-pressed={search.status === "needs_mapping"}
                      onClick={() =>
                        filters({
                          status:
                            search.status === "needs_mapping"
                              ? ""
                              : "needs_mapping",
                        })
                      }
                    >
                      Needs mapping
                    </Button>
                    {filtered && (
                      <Button
                        size="compact-xs"
                        variant="subtle"
                        onClick={clearFilters}
                      >
                        Clear filters
                      </Button>
                    )}
                  </Group>
                  <Text size="xs" className={classes.meta} role="status">
                    {events.isPending
                      ? "Loading events…"
                      : events.error
                        ? "Events unavailable"
                        : count
                          ? `${start}–${Math.min(start + 24, count)} of ${count.toLocaleString()} events`
                          : "0 events"}
                  </Text>
                </Group>
                {events.isPending ? (
                  <Stack gap="xs">
                    {[1, 2, 3, 4].map((v) => (
                      <Skeleton key={v} height={70} />
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
                  <EventList events={events.data.data} />
                ) : (
                  <Alert
                    color="gray"
                    title={
                      filtered
                        ? "No events match these filters"
                        : "No sync events yet"
                    }
                  >
                    <Text size="sm">
                      {filtered
                        ? "Try another title, outcome or provider to find an event."
                        : "New Plex events will appear here. Each event keeps its recorded sync outcome."}
                    </Text>
                    {filtered && (
                      <Button
                        mt="sm"
                        size="xs"
                        className={classes.touchUtility}
                        onClick={clearFilters}
                      >
                        Clear filters
                      </Button>
                    )}
                  </Alert>
                )}
                {!!count && (
                  <Group justify="space-between" gap="sm">
                    <Text size="xs" className={classes.meta}>
                      Page {search.page || 1} of{" "}
                      {Math.max(1, Math.ceil(count / 25))}
                    </Text>
                    <Pagination
                      size="sm"
                      siblings={0}
                      total={Math.max(1, Math.ceil(count / 25))}
                      value={search.page || 1}
                      onChange={(page) =>
                        void navigate({ search: (prev) => ({ ...prev, page }) })
                      }
                      aria-label="Event pages"
                    />
                  </Group>
                )}
              </Stack>
            </Tabs.Panel>
            <Tabs.Panel value="rules" pt="md">
              <LocalRules csrf={cap.data?.csrfToken} />
            </Tabs.Panel>
          </Tabs>
        </Paper>
      </Stack>
    </Container>
  );
}
function LocalRules({ csrf }: { csrf?: string }) {
  const queryClient = useQueryClient();
  const navigate = SyncInspectorRoute.useNavigate();
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
      <Group justify="space-between" align="start">
        <div>
          <Group gap="xs">
            <Text fw={600}>Personal mapping rules</Text>
            {rules.data && (
              <Badge color="gray" variant="light">
                {rules.data.data.length}
              </Badge>
            )}
          </Group>
          <Text size="sm" mt={4}>
            Override matching for future events in a specific Plex library.
            Existing events are never retried automatically.
          </Text>
        </div>
        <Button
          size="compact-xs"
          variant="subtle"
          loading={rules.isFetching}
          onClick={() => void rules.refetch()}
        >
          Reload rules
        </Button>
      </Group>
      {rules.isPending && <Skeleton height={100} />}{" "}
      {rules.error && <ErrorNotice error={rules.error} />}{" "}
      {disable.error && <ErrorNotice error={disable.error} />}{" "}
      {update.error && <ErrorNotice error={update.error} />}{" "}
      {rules.data?.data.length === 0 && (
        <Alert title="No local rules yet">
          <Text size="sm">
            Inspect an event and choose a MAL entry to save a scoped rule.
          </Text>
          <Button
            mt="sm"
            size="xs"
            onClick={() =>
              void navigate({ search: (prev) => ({ ...prev, tab: "events" }) })
            }
          >
            Browse events
          </Button>
        </Alert>
      )}
      {rules.data?.data.map((rule) => (
        <Paper key={rule.id} withBorder p="md">
          <Stack gap="sm">
            <Group justify="space-between">
              <Text fw={600} size="sm">
                {rule.source.toUpperCase()} {rule.sourceId} · S{rule.season} →
                MAL {rule.malId}
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
            <Group gap="sm" className={classes.actions}>
              <Button
                size="xs"
                disabled={!csrf || !!editing}
                onClick={() => setEditing({ ...rule })}
              >
                Edit rule
              </Button>
              {!rule.active && (
                <Button
                  disabled={!csrf || !!editing}
                  size="xs"
                  variant="subtle"
                  loading={update.isPending}
                  onClick={() => setEditing({ ...rule, active: true })}
                >
                  Enable rule
                </Button>
              )}
              {rule.active && (
                <Button
                  color="red"
                  variant="subtle"
                  size="xs"
                  disabled={!csrf || !!editing}
                  onClick={() => setConfirm(rule.id)}
                >
                  Disable rule
                </Button>
              )}
              <Anchor
                size="sm"
                renderRoot={(props) => (
                  <Link
                    {...props}
                    to="/sync/events/$eventId"
                    params={{ eventId: String(rule.eventId) }}
                  />
                )}
              >
                Inspect source event
              </Anchor>
            </Group>
            {editing?.id === rule.id && (
              <RuleEditor
                draft={editing}
                reactivating={!rule.active && editing.active}
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
  reactivating,
}: {
  draft: LocalRule;
  reactivating: boolean;
  onChange: (rule: LocalRule) => void;
  csrf?: string;
  pending: boolean;
  onSave: (fingerprint: string) => void;
  onCancel: () => void;
}) {
  const [initialDraft] = useState(() => JSON.stringify(draft));
  const [confirmDiscard, setConfirmDiscard] = useState(false);
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
    <Paper withBorder p="md" className={classes.editor}>
      <Stack>
        <RuleDraftGuard
          dirty={reactivating || JSON.stringify(draft) !== initialDraft}
          onDiscard={onCancel}
        />
        <Text className={classes.stepLabel}>1 · Edit scope and conversion</Text>
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
        <Group className={classes.actions}>
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
          <Button
            disabled={pending}
            onClick={() =>
              JSON.stringify(draft) !== initialDraft
                ? setConfirmDiscard(true)
                : onCancel()
            }
          >
            Discard edits
          </Button>
        </Group>
        {confirmDiscard && (
          <Alert color="yellow" title="Discard these unsaved rule edits?">
            <Text size="sm">
              The saved rule will keep its current revision.
            </Text>
            <Group mt="sm">
              <Button color="red" disabled={pending} onClick={onCancel}>
                Discard rule edits
              </Button>
              <Button onClick={() => setConfirmDiscard(false)}>
                Keep editing
              </Button>
            </Group>
          </Alert>
        )}
      </Stack>
    </Paper>
  );
}
