import { Button, Group, Stack, Table, Text, Tooltip } from "@mantine/core";
import { useResizeObserver } from "@mantine/hooks";
import { Link } from "@tanstack/react-router";
import { formatDistanceToNowStrict } from "date-fns";
import type { SyncEvent } from "@api/sync";
import { SyncInspectorRoute } from "@app/routes";
import { MALLink, Outcome } from "./shared";
import classes from "./SyncInspector.module.css";

export function EventAge({ value }: { value: string }) {
  const date = new Date(value);
  const valid = value && !value.startsWith("0001-") && !isNaN(date.getTime());
  return valid ? (
    <Tooltip label={date.toLocaleString()} withArrow>
      <Text
        component="time"
        dateTime={value}
        title={date.toLocaleString()}
        size="xs"
        className={classes.meta}
        tabIndex={0}
      >
        {formatDistanceToNowStrict(date, { addSuffix: true })}
      </Text>
    </Tooltip>
  ) : (
    <Text size="xs">Time unavailable</Text>
  );
}
function EventIdentity({ event }: { event: SyncEvent }) {
  return (
    <Stack gap={3}>
      <Text fw={600} size="sm" className={classes.eventTitle}>
        {event.title || "Untitled event"}
      </Text>
      <Text size="xs" className={classes.meta}>
        {event.mediaType === "movie"
          ? "Movie"
          : `S${event.season} · E${event.episode}`}
        {event.event.includes("rate") ? ` · Rating ${event.rating}/10` : ""} ·{" "}
        {event.library || "Unknown library"}
      </Text>
    </Stack>
  );
}
function EventOutcome({ event }: { event: SyncEvent }) {
  return (
    <Stack gap={4} align="start">
      <Outcome status={event.status} />
      {event.lastAction &&
        !["Original sync", "Received"].includes(event.lastAction) && (
          <Text size="xs" className={classes.meta}>
            Latest {event.lastAction.replaceAll("_", " ")} attempt
          </Text>
        )}
      {event.errorMessage && (
        <Text
          size="xs"
          className={classes.eventError}
          title={event.errorMessage}
        >
          {event.errorMessage}
        </Text>
      )}
    </Stack>
  );
}
function EventDestination({ event }: { event: SyncEvent }) {
  return (
    <Stack gap={3}>
      {event.malId > 0 ? (
        <Stack gap={2}>
          {event.malTitle && (
            <Text
              size="xs"
              className={classes.destinationTitle}
              title={event.malTitle}
            >
              {event.malTitle}
            </Text>
          )}
          <Text size="xs">
            <MALLink id={event.malId} />
          </Text>
        </Stack>
      ) : (
        <Text size="sm">No MAL destination</Text>
      )}
      <Text size="xs" className={classes.meta}>
        {event.source?.toUpperCase() || "Unknown source"}{" "}
        {event.sourceId || "ID unavailable"} ·{" "}
        {event.account || "Unknown account"}
      </Text>
    </Stack>
  );
}
export function EventList({ events }: { events: SyncEvent[] }) {
  const search = SyncInspectorRoute.useSearch();
  const [listRef, rect] = useResizeObserver();
  const desktop = rect.width >= 700;
  const inspect = (event: SyncEvent, mobile = false) => (
    <Button
      size={mobile ? "sm" : "compact-sm"}
      fullWidth={mobile}
      renderRoot={(props) => (
        <Link
          {...props}
          to="/sync/events/$eventId"
          params={{ eventId: String(event.id) }}
          search={search}
        />
      )}
      title={`Inspect ${event.title || "event"} — event ${event.id}`}
    >
      Inspect
    </Button>
  );
  return (
    <div ref={listRef}>
      {!desktop ? (
        <div aria-label="Sync events">
          {events.map((event) => (
            <div key={event.id} className={classes.mobileEvent}>
              <Stack gap="sm">
                <EventIdentity event={event} />
                <Group justify="space-between" align="start">
                  <EventOutcome event={event} />
                  <EventAge value={event.receivedAt} />
                </Group>
                <EventDestination event={event} />
                {inspect(event, true)}
              </Stack>
            </div>
          ))}
        </div>
      ) : (
        <Table
          highlightOnHover
          verticalSpacing="xs"
          horizontalSpacing="sm"
          className={classes.table}
          aria-label="Sync events"
        >
          <Table.Thead>
            <Table.Tr>
              <Table.Th w="29%">Event</Table.Th>
              <Table.Th w="23%">MAL outcome</Table.Th>
              <Table.Th w="25%">Destination / source</Table.Th>
              <Table.Th w="14%">Received</Table.Th>
              <Table.Th w="9%">
                <Text size="sm" ta="right" fw={700}>
                  Action
                </Text>
              </Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {events.map((event) => (
              <Table.Tr key={event.id}>
                <Table.Td>
                  <Link
                    to="/sync/events/$eventId"
                    params={{ eventId: String(event.id) }}
                    search={search}
                    title={event.title}
                    style={{ color: "inherit", textDecoration: "none" }}
                  >
                    <EventIdentity event={event} />
                  </Link>
                </Table.Td>
                <Table.Td>
                  <EventOutcome event={event} />
                </Table.Td>
                <Table.Td>
                  <EventDestination event={event} />
                </Table.Td>
                <Table.Td>
                  <EventAge value={event.receivedAt} />
                </Table.Td>
                <Table.Td ta="right">{inspect(event)}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </div>
  );
}
