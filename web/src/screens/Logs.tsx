import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ActionIcon,
  Checkbox,
  Container,
  Group,
  Menu,
  Paper,
  ScrollArea,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { format } from "date-fns";
import { FaCog, FaTrash, FaDownload } from "react-icons/fa";

import { APIClient } from "@api/APIClient";
import { SettingsContext } from "@utils/Context";
import { displayNotification } from "@components/notifications";
import { LogQueryOptions } from "@api/queries";
import { baseUrl } from "@utils";
import type { LogFileResponse } from "@app/types/FileSystem";

type LogEvent = {
  time: string;
  level: string;
  message: string;
};

type LogLevel = "TRC" | "DBG" | "INF" | "ERR" | "WRN" | "FTL" | "PNC";

const LogColors: Record<LogLevel, string> = {
  TRC: "purple",
  DBG: "yellow",
  INF: "green",
  ERR: "red",
  WRN: "yellow",
  FTL: "red",
  PNC: "red",
};

export const Logs = () => {
  const [settings] = SettingsContext.use();
  const viewportRef = useRef<HTMLDivElement>(null);

  const [logs, setLogs] = useState<LogEvent[]>([]);
  const [searchFilter, setSearchFilter] = useState("");
  const [, setRegexPattern] = useState<RegExp | null>(null);
  const [filteredLogs, setFilteredLogs] = useState<LogEvent[]>([]);
  const [isInvalidRegex, setIsInvalidRegex] = useState(false);

  useEffect(() => {
    if (settings.scrollOnNewLog && viewportRef.current) {
      // Use requestAnimationFrame to ensure DOM has updated
      requestAnimationFrame(() => {
        if (viewportRef.current) {
          viewportRef.current.scrollTop = viewportRef.current.scrollHeight;
        }
      });
    }
  }, [filteredLogs, settings.scrollOnNewLog]);

  // Add a useEffect to clear logs div when settings.scrollOnNewLog changes to prevent duplicate entries.
  useEffect(() => {
    setLogs([]);
  }, [settings.scrollOnNewLog]);

  useEffect(() => {
    const es = APIClient.events.logs();

    es.onmessage = (event) => {
      const newData = JSON.parse(event.data) as LogEvent;
      setLogs((prevState) => [...prevState, newData]);
    };

    return () => es.close();
  }, [setLogs, settings]);

  useEffect(() => {
    if (!searchFilter.length) {
      setFilteredLogs(logs);
      setIsInvalidRegex(false);
      return;
    }

    try {
      const pattern = new RegExp(searchFilter, "i");
      setRegexPattern(pattern);
      const newLogs = logs.filter((log) => pattern.test(log.message));
      setFilteredLogs(newLogs);
      setIsInvalidRegex(false);
    } catch {
      // Handle regex errors by showing nothing when the regex pattern is invalid
      setFilteredLogs([]);
      setIsInvalidRegex(true);
    }
  }, [logs, searchFilter]);

  const handleClearLogs = () => {
    setLogs([]);
    displayNotification({
      title: "Logs cleared",
      message: "Logs cleared from view.",
      type: "success",
    });
  };

  return (
    <Container size={1200} px="md" component="main">
      <Stack gap="md" p="md">
        <Title order={2}>Logs</Title>

        <Paper withBorder p="md">
          <Stack gap="md">
            <Group>
              <TextInput
                placeholder="Enter a regex pattern to filter logs by..."
                style={{ flex: 1 }}
                value={searchFilter}
                onChange={(e) => {
                  const inputValue = e.target.value.toLowerCase().trim();
                  setSearchFilter(inputValue);
                }}
                error={isInvalidRegex ? "Invalid regex pattern" : undefined}
              />
              <ActionIcon
                variant="light"
                onClick={handleClearLogs}
                title="Clear logs from view"
              >
                <FaTrash size={14} />
              </ActionIcon>
              <LogsDropdown />
            </Group>

            <ScrollArea h="60vh" type="scroll" viewportRef={viewportRef}>
              <div>
                {filteredLogs.map((entry, idx) => {
                  const shouldIndent = settings.indentLogLines;
                  const messageLines = shouldIndent ? entry.message.split("\n") : [];
                  const firstLine = shouldIndent ? (messageLines[0] || "") : entry.message;
                  const remainingLines = shouldIndent ? messageLines.slice(1) : [];

                  return (
                    <div
                      key={idx}
                      style={{
                        fontFamily: "monospace",
                        fontSize: "14px",
                        lineHeight: shouldIndent ? 1.2 : 1.6,
                        padding: shouldIndent ? "0" : "2px 0",
                        margin: 0,
                        display: shouldIndent ? "grid" : "block",
                        gridTemplateColumns: shouldIndent ? "auto auto 1fr" : undefined,
                        gap: shouldIndent ? "8px" : undefined,
                        alignItems: shouldIndent ? "flex-start" : undefined,
                      }}
                    >
                      {shouldIndent ? (
                        <>
                          <span
                            title={entry.time}
                            style={{
                              color: "var(--mantine-color-dimmed)",
                              fontSize: "14px",
                              whiteSpace: "nowrap",
                            }}
                          >
                            {format(new Date(entry.time), "HH:mm:ss")}
                          </span>
                          {entry.level in LogColors ? (
                            <span
                              style={{
                                color: `var(--mantine-color-${LogColors[entry.level as LogLevel]}-6)`,
                                fontWeight: 600,
                                fontSize: "14px",
                                whiteSpace: "nowrap",
                              }}
                            >
                              {entry.level}
                            </span>
                          ) : null}
                          <div
                            style={{
                              display: "flex",
                              flexDirection: "column",
                              flex: 1,
                            }}
                          >
                            <span
                              style={{
                                color: "var(--mantine-color-text)",
                                fontSize: "14px",
                                whiteSpace: settings.hideWrappedText ? "nowrap" : "pre-wrap",
                                overflow: settings.hideWrappedText ? "hidden" : "visible",
                                textOverflow: settings.hideWrappedText ? "ellipsis" : "clip",
                              }}
                            >
                              {firstLine}
                            </span>
                            {/* Remaining lines indented */}
                            {remainingLines.length > 0 && !settings.hideWrappedText && (
                              <div
                                style={{
                                  color: "var(--mantine-color-text)",
                                  fontSize: "14px",
                                  whiteSpace: "pre-wrap",
                                }}
                              >
                                {remainingLines.map((line, lineIdx) => (
                                  <div key={lineIdx}>{line || " "}</div>
                                ))}
                              </div>
                            )}
                          </div>
                        </>
                      ) : (
                        <span
                          style={{
                            color: "var(--mantine-color-text)",
                            fontSize: "14px",
                            whiteSpace: settings.hideWrappedText ? "nowrap" : "pre-wrap",
                            overflow: settings.hideWrappedText ? "hidden" : "visible",
                            textOverflow: settings.hideWrappedText ? "ellipsis" : "clip",
                          }}
                        >
                          <span
                            title={entry.time}
                            style={{
                              color: "var(--mantine-color-dimmed)",
                              marginRight: "8px",
                            }}
                          >
                            {format(new Date(entry.time), "HH:mm:ss")}
                          </span>
                          {entry.level in LogColors ? (
                            <span
                              style={{
                                color: `var(--mantine-color-${LogColors[entry.level as LogLevel]}-6)`,
                                fontWeight: 600,
                                marginRight: "8px",
                              }}
                            >
                              {entry.level}
                            </span>
                          ) : null}
                          {entry.message}
                        </span>
                      )}
                    </div>
                  );
                })}
              </div>
            </ScrollArea>
          </Stack>
        </Paper>

        <Paper withBorder p="md">
          <LogFiles />
        </Paper>
      </Stack>
    </Container>
  );
};

export const LogFiles = () => {
  const { isError, error, data } = useQuery(LogQueryOptions());

  if (isError) {
    console.log("could not load log files", error);
  }

  return (
    <div>
      <Stack gap="md">
        <div>
          <Title order={3}>Log files</Title>
          <Text size="sm" c="dimmed">
            Download log files
          </Text>
        </div>

        {data && data.length > 0 ? (
          <Group gap="md" align="flex-start">
            {data.map((file, idx) => (
              <LogFilesItem key={idx} file={file} />
            ))}
          </Group>
        ) : (
          <Text size="sm" c="dimmed" ta="center">
            No log files found
          </Text>
        )}
      </Stack>
    </div>
  );
};

interface LogFilesItemProps {
  file: LogFileResponse;
}

const LogFilesItem = ({ file }: LogFilesItemProps) => {
  const [isDownloading, setIsDownloading] = useState(false);

  const handleDownload = async () => {
    setIsDownloading(true);

    displayNotification({
      title: "Downloading",
      message: "Please wait...",
      type: "info",
    });

    try {
      const response = await fetch(
        `${baseUrl()}api/fs/logs/${file.name}`
      );
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = file.name;
      link.click();
      URL.revokeObjectURL(url);

      displayNotification({
        title: "Download complete",
        message: "Log file downloaded successfully.",
        type: "success",
      });
    } catch {
      displayNotification({
        title: "Download failed",
        message: "Failed to download log file.",
        type: "error",
      });
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <Group align="flex-start" mt="md">
      <Stack gap={0}>
        <Group>
          <Text fw={600}>
            {file.name}
          </Text>
          <ActionIcon
            loading={isDownloading}
            onClick={handleDownload}
          >
            <FaDownload size={12} />
          </ActionIcon>
        </Group>
        <Text size="xs" c="dimmed">
          Size: {file.size_human}
        </Text>
        <Text size="xs" c="dimmed">
          Last Modified: {new Date(file.modified_at).toLocaleString()}
        </Text>
      </Stack>
    </Group>
  );
};

const LogsDropdown = () => {
  const [settings, setSettings] = SettingsContext.use();

  const onSetValue = (
    key: "scrollOnNewLog" | "indentLogLines" | "hideWrappedText",
    newValue: boolean
  ) =>
    setSettings((prevState) => ({
      ...prevState,
      [key]: newValue,
    }));

  return (
    <Menu>
      <Menu.Target>
        <ActionIcon variant="light">
          <FaCog size={14} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>Log Settings</Menu.Label>
        <Menu.Item closeMenuOnClick={false}>
          <Checkbox
            label="Scroll to bottom on new message"
            checked={settings.scrollOnNewLog}
            onChange={(e) => {
              e.stopPropagation();
              onSetValue("scrollOnNewLog", e.currentTarget.checked);
            }}
          />
        </Menu.Item>
        <Menu.Item closeMenuOnClick={false}>
          <Checkbox
            label="Indent log lines"
            description="Indent each log line according to their respective starting position."
            checked={settings.indentLogLines}
            onChange={(e) => {
              e.stopPropagation();
              onSetValue("indentLogLines", e.currentTarget.checked);
            }}
          />
        </Menu.Item>
        <Menu.Item closeMenuOnClick={false}>
          <Checkbox
            label="Hide wrapped text"
            description="Hides text that is meant to be wrapped."
            checked={settings.hideWrappedText}
            onChange={(e) => {
              e.stopPropagation();
              onSetValue("hideWrappedText", e.currentTarget.checked);
            }}
          />
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
};
