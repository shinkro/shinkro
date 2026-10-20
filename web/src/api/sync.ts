import { baseUrl } from "@utils";

export type SyncStatus =
  | "synced"
  | "no_change"
  | "needs_mapping"
  | "needs_sign_in"
  | "needs_review"
  | "failed"
  | "queued"
  | "processing"
  | "received"
  | "cancelled";
export interface SyncEvent {
  id: number;
  title: string;
  episodeTitle: string;
  account: string;
  library: string;
  event: string;
  mediaType: string;
  source: string;
  sourceId: number;
  season: number;
  episode: number;
  rating: number;
  receivedAt: string;
  status: SyncStatus;
  errorCode: string;
  errorMessage: string;
  malId: number;
  malTitle: string;
  lastAction: string;
  legacy: boolean;
}
export interface SyncInput extends Omit<
  SyncEvent,
  | "id"
  | "status"
  | "errorCode"
  | "errorMessage"
  | "malId"
  | "malTitle"
  | "lastAction"
  | "legacy"
> {
  version: number;
  eventId: number;
  serverId: string;
}
export interface SourceSnapshot {
  kind: string;
  label: string;
  contentHash: string;
  entries: number;
  observedAt: string;
  lastRefreshAt?: string;
}
export interface Resolution {
  malId: number;
  episode: number;
  origin: string;
  title: string;
  steps: { label: string; detail: string }[];
  fingerprint: string;
  sources?: SourceSnapshot[];
}
export interface MALState {
  malId: number;
  title: string;
  pictureUrl: string;
  totalEpisodes: number;
  status: string;
  episodesWatched: number;
  score: number;
  isRewatching: boolean;
  rewatchCount: number;
  startDate: string;
  finishDate: string;
  updatedAt: string;
}
export interface Preview {
  id: string;
  eventId: number;
  createdAt: string;
  expiresAt: string;
  ready: boolean;
  writesEnabled: boolean;
  code: string;
  message: string;
  resolution: Resolution;
  before?: MALState;
  after?: MALState;
  changes: { field: string; before: unknown; after: unknown }[];
  warnings: string[];
  accountId: string;
}
export interface Attempt {
  observed?: MALState;
  observedAt?: string;
  observationSource?:
    | "write_response"
    | "confirmation_read"
    | "reconciliation"
    | "no_change_read";
  id: string;
  jobId: string;
  kind: string;
  status: string;
  createdAt: string;
  finishedAt: string;
  preview?: Preview;
  code: string;
  message: string;
}
export interface LocalRule {
  anyServer?: boolean;
  impactFingerprint?: string;
  id: string;
  revision: number;
  eventId: number;
  source: string;
  sourceId: number;
  season: number;
  library: string;
  account: string;
  serverId: string;
  mediaType: string;
  sourceEpisodeStart: number;
  sourceEpisodeEnd: number;
  malEpisodeStart: number;
  malId: number;
  active: boolean;
  createdAt: string;
}
export interface EventDetail {
  original?: SyncEvent;
  event: SyncEvent;
  input: SyncInput;
  attempts: Attempt[];
  legacy: boolean;
  replayEligible: boolean;
  replayReason: string;
  overrides: LocalRule[];
}
export interface Job {
  id: string;
  eventId: number;
  kind: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  code: string;
  message: string;
  previewId: string;
}
export interface RuleImpact {
  scope: LocalRule;
  affectedEventIds: number[];
  conflicts: LocalRule[];
  ready: boolean;
  warnings: string[];
  fingerprint: string;
}
export interface Capabilities {
  writesEnabled: boolean;
  csrfToken: string;
  previewTTLSeconds: number;
}
export interface MALResult {
  malId: number;
  title: string;
  pictureUrl: string;
  totalEpisodes: number;
  mediaType: string;
}
export class SyncError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}
async function request<T>(
  path: string,
  method = "GET",
  body?: unknown,
  csrf?: string,
): Promise<T> {
  const response = await fetch(`${baseUrl()}api/sync/${path}`, {
    method,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...(csrf ? { "X-Shinkro-CSRF": csrf } : {}),
    },
    ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
  });
  if (response.status === 204) return undefined as T;
  const data = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new SyncError(
      response.status,
      data.code || "REQUEST_FAILED",
      data.message ||
        (response.status === 401 || response.status === 403
          ? "An operator session is required. Sign in again, then retry."
          : `Request failed (${response.status}).`),
    );
  return data as T;
}
export const syncAPI = {
  capabilities: () => request<Capabilities>("capabilities"),
  events: (filters: Record<string, string | number>) =>
    request<{ data: SyncEvent[]; count: number }>(
      `events?${new URLSearchParams(Object.entries(filters).map(([k, v]) => [k, String(v)]))}`,
    ),
  detail: (id: number) => request<EventDetail>(`events/${id}`),
  preview: (
    id: number,
    csrf: string,
    correction?: { malId: number; episode: number },
  ) =>
    request<Preview>(
      `events/${id}/preview`,
      "POST",
      correction ? { correction } : {},
      csrf,
    ),
  replay: (id: number, previewId: string, csrf: string) =>
    request<Job>(`events/${id}/replays`, "POST", { previewId }, csrf),
  job: (id: string) => request<Job>(`jobs/${encodeURIComponent(id)}`),
  search: (q: string) =>
    request<{ data: MALResult[] }>(`mal/search?q=${encodeURIComponent(q)}`),
  refresh: (csrf: string) =>
    request<{ refreshedAt: string }>("sources/refresh", "POST", {}, csrf),
  rules: () => request<{ data: LocalRule[] }>("overrides"),
  saveRule: (
    body: {
      anyServer?: boolean;
      impactFingerprint: string;
      eventId: number;
      malId: number;
      sourceEpisodeStart: number;
      sourceEpisodeEnd: number;
      malEpisodeStart: number;
    },
    csrf: string,
  ) => request<LocalRule>("overrides", "POST", body, csrf),
  previewRule: (
    body: {
      anyServer?: boolean;
      eventId: number;
      malId: number;
      sourceEpisodeStart: number;
      sourceEpisodeEnd: number;
      malEpisodeStart: number;
    },
    csrf: string,
  ) => request<RuleImpact>("overrides/preview", "POST", body, csrf),
  updateRule: (rule: LocalRule, csrf: string) =>
    request<LocalRule>(
      `overrides/${encodeURIComponent(rule.id)}`,
      "PUT",
      rule,
      csrf,
    ),
  disableRule: (id: string, revision: number, csrf: string) =>
    request<void>(
      `overrides/${encodeURIComponent(id)}?revision=${revision}`,
      "DELETE",
      undefined,
      csrf,
    ),
};
