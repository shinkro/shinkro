import { syncKeys, statusLabels, displayValue } from "./values";
import {
  Alert,
  Checkbox,
  Anchor,
  Badge,
  Button,
  Group,
  Paper,
  Stack,
  SimpleGrid,
  Table,
  Text,
} from "@mantine/core";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  syncAPI,
  SyncError,
  type Preview,
  type Resolution,
  type MALState,
} from "@api/sync";

export function Outcome({ status }: { status: string }) {
  return (
    <Badge
      variant="light"
      color={
        status === "synced" || status === "no_change" || status === "enabled"
          ? "green"
          : status === "needs_mapping" || status === "needs_review"
            ? "yellow"
            : status === "failed" || status === "needs_sign_in"
              ? "red"
              : status === "queued" || status === "processing"
                ? "blue"
                : "gray"
      }
    >
      {statusLabels[status] || status.replaceAll("_", " ")}
    </Badge>
  );
}
export function ReadableTime({ value }: { value?: string }) {
  const date = value && !value.startsWith("0001-") ? new Date(value) : null;
  return (
    <Text component="time" size="sm" c="dimmed" dateTime={value}>
      {date && !isNaN(date.getTime())
        ? date.toLocaleString()
        : "Time unavailable"}
    </Text>
  );
}
export function ErrorNotice({ error }: { error: unknown }) {
  return (
    <Alert color="red" title="Unable to complete this action" role="alert">
      {error instanceof Error
        ? error.message
        : "Something went wrong. Please retry."}
      {error instanceof SyncError &&
        (error.status === 401 || error.status === 403) && (
          <Group mt="sm">
            <Link
              to="/login"
              search={{
                redirect: window.location.pathname + window.location.search,
              }}
            >
              Sign in again
            </Link>
          </Group>
        )}
    </Alert>
  );
}
export function DevBanner({
  developmentBuild = false,
}: {
  developmentBuild?: boolean;
}) {
  const cap = useQuery({
    queryKey: syncKeys.capabilities,
    queryFn: syncAPI.capabilities,
  });
  return cap.data?.writesEnabled === false ? (
    <Alert color="yellow" title="MAL writes disabled on this instance">
      {developmentBuild && (
        <Badge color="yellow" mb="xs">
          Development build
        </Badge>
      )}
      <Text size="sm">
        Inspect, preview, refresh data and save local rules here. Applying a
        sync to MyAnimeList is disabled by the server.
      </Text>
    </Alert>
  ) : null;
}
export function MALLink({ id, title }: { id: number; title?: string }) {
  return (
    <Anchor
      href={`https://myanimelist.net/anime/${id}`}
      target="_blank"
      rel="noreferrer"
      style={{ overflowWrap: "anywhere" }}
    >
      {title || `MAL ${id}`}
      {title ? ` · MAL ${id}` : ""}
    </Anchor>
  );
}
const origins: Record<string, string> = {
  direct_mal: "Direct MAL",
  canonical_anidb: "Canonical AniDB",
  community_rule: "Community rule",
  custom_file: "Custom file",
  catalogue: "Catalogue",
  local_override: "Local override",
  event_correction: "This event only",
};
export function ResolutionTrace({ resolution }: { resolution: Resolution }) {
  return (
    <Stack gap="sm">
      <Group>
        <Badge variant="outline">
          {origins[resolution.origin] || resolution.origin}
        </Badge>
        {resolution.malId > 0 && (
          <MALLink id={resolution.malId} title={resolution.title} />
        )}
      </Group>
      {resolution.episode > 0 && (
        <Text size="sm">Target episode {resolution.episode}</Text>
      )}
      <Stack component="ol" gap="xs" m={0} pl="lg">
        {resolution.steps?.map((step, index) => (
          <li key={index}>
            <Text size="sm" fw={600}>
              {step.label}
            </Text>
            <Text size="sm" c="dimmed" style={{ overflowWrap: "anywhere" }}>
              {step.detail}
            </Text>
          </li>
        ))}
      </Stack>
      {!!resolution.sources?.length && (
        <Stack gap="xs">
          <Text size="sm" fw={600}>
            Data used
          </Text>
          {resolution.sources.map((source, index) => (
            <div key={`${source.kind}-${index}`}>
              <Text size="sm">
                {source.label} · {source.entries.toLocaleString()} entries
              </Text>
              {source.lastRefreshAt &&
              !source.lastRefreshAt.startsWith("0001-") ? (
                <Group gap="xs">
                  <Text size="xs" c="dimmed">
                    Last successful refresh
                  </Text>
                  <ReadableTime value={source.lastRefreshAt} />
                </Group>
              ) : (
                <>
                  <Group gap="xs">
                    <Text size="xs" c="dimmed">
                      Local observation
                    </Text>
                    <ReadableTime value={source.observedAt} />
                  </Group>
                  <Text size="xs" c="dimmed">
                    Upstream freshness is unknown.
                  </Text>
                </>
              )}
            </div>
          ))}
          <details>
            <summary>Source diagnostics</summary>
            <Stack gap="xs" mt="xs">
              {resolution.sources.map((source, index) => (
                <div key={`${source.kind}-${index}`}>
                  <Text size="xs" fw={600}>
                    {source.label}
                  </Text>
                  <Text
                    component="code"
                    size="xs"
                    style={{ overflowWrap: "anywhere", userSelect: "all" }}
                  >
                    {source.contentHash}
                  </Text>
                </div>
              ))}
            </Stack>
          </details>
        </Stack>
      )}
    </Stack>
  );
}
const fields: Record<string, string> = {
  episodesWatched: "Episodes watched",
  num_watched_episodes: "Episodes watched",
  status: "Watch status",
  score: "Score",
  isRewatching: "Rewatching",
  is_rewatching: "Rewatching",
  rewatchCount: "Rewatch count",
  num_times_rewatched: "Rewatch count",
  startDate: "Start date",
  finishDate: "Finish date",
  start_date: "Start date",
  finish_date: "Finish date",
};
export function ChangeDiff({
  preview,
  recorded = false,
}: {
  preview: Preview;
  recorded?: boolean;
}) {
  return preview.changes?.length ? (
    <Table
      withTableBorder
      verticalSpacing="sm"
      style={{ tableLayout: "fixed" }}
    >
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Field</Table.Th>
          <Table.Th>{recorded ? "MAL at preview" : "Current on MAL"}</Table.Th>
          <Table.Th>{recorded ? "Planned change" : "Proposed"}</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {preview.changes.map((change) => (
          <Table.Tr key={change.field}>
            <Table.Td style={{ overflowWrap: "anywhere" }}>
              {fields[change.field] || change.field}
            </Table.Td>
            <Table.Td style={{ overflowWrap: "anywhere" }}>
              {displayValue(change.before)}
            </Table.Td>
            <Table.Td style={{ overflowWrap: "anywhere" }}>
              <Text fw={600} size="sm">
                {displayValue(change.after)}
              </Text>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  ) : preview.ready ? (
    <Alert color="green" title="No change needed">
      {recorded
        ? "No change was planned for this attempt."
        : "The MAL state at this preview already satisfies the event. Nothing will be written."}
    </Alert>
  ) : null;
}
export function SignInAction({ code }: { code?: string }) {
  return code?.includes("AUTH") || code?.includes("SIGN_IN") ? (
    <Button component={Link} to="/settings/mal">
      Open MyAnimeList settings
    </Button>
  ) : null;
}
export function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Paper withBorder p="md" radius="md">
      <Stack gap="md">
        <Text component="h2" size="lg" fw={700} m={0}>
          {title}
        </Text>
        {children}
      </Stack>
    </Paper>
  );
}

export function RuleServerScope({
  serverId,
  anyServer,
  onChange,
  account,
  library,
}: {
  serverId: string;
  anyServer: boolean;
  onChange: (selected: boolean) => void;
  account: string;
  library: string;
}) {
  if (serverId)
    return (
      <Text size="sm" c="dimmed">
        Server scope: {serverId}
      </Text>
    );
  return (
    <Alert
      color="yellow"
      title="Choose a server scope for this historical event"
    >
      <Stack gap="sm">
        <Text size="sm">
          The original server identity was not saved. An event-only correction
          remains available. To match future events, explicitly include all
          servers within this Plex account and library.
        </Text>
        <Checkbox
          label="All servers in this library and Plex account"
          description={`${account} / ${library}. The provider, title ID, season and episode range still limit this rule.`}
          checked={anyServer}
          onChange={(event) => onChange(event.currentTarget.checked)}
        />
      </Stack>
    </Alert>
  );
}

export function ObservedResult({
  state,
  at,
  source,
}: {
  state: MALState;
  at?: string;
  source?: string;
}) {
  const labels: Record<string, string> = {
    write_response: "MAL update response",
    confirmation_read: "Confirmed by a MAL read after update",
    reconciliation: "MAL read during reconciliation",
    no_change_read: "MAL read when no change was needed",
  };
  const facts = [
    ["Episodes watched", state.episodesWatched],
    ["Watch status", state.status],
    ["Score", state.score || "Not rated"],
    ["Rewatching", state.isRewatching],
    ["Rewatch count", state.rewatchCount],
    ["Start date", state.startDate],
    ["Finish date", state.finishDate],
  ] as const;
  return (
    <Paper withBorder p="sm">
      <Stack gap="sm">
        <Text fw={600} size="sm">
          Observed MAL result
        </Text>
        <Group justify="space-between">
          <Text size="xs" c="dimmed">
            {source
              ? labels[source] || "Recorded MAL observation"
              : "Recorded MAL observation"}
          </Text>
          <ReadableTime value={at} />
        </Group>
        {state.malId > 0 && <MALLink id={state.malId} title={state.title} />}
        <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="xs">
          {facts.map(([label, value]) => (
            <div key={label}>
              <Text size="xs" c="dimmed">
                {label}
              </Text>
              <Text size="sm">{displayValue(value)}</Text>
            </div>
          ))}
        </SimpleGrid>
        <Text size="xs" c="dimmed">
          This is the state MAL returned at the recorded time. It is separate
          from the saved plan and may have changed since.
        </Text>
      </Stack>
    </Paper>
  );
}
