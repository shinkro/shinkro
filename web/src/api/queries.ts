import {queryOptions, keepPreviousData} from "@tanstack/react-query";
import {APIClient} from "@api/APIClient";
import {
    ApiKeys,
    NotificationKeys,
    SettingsKeys,
    PlexSettingsKeys,
    MalAuthKeys,
    AnilistAuthKeys,
    MappingKeys,
    LogKeys,
    PlexKeys,
    AnimeUpdateKeys,
} from "@api/query_keys";
import { baseUrl } from "@utils";
import type { ColumnFilter } from "@tanstack/react-table";

export const ConfigQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: SettingsKeys.config(),
        queryFn: () => APIClient.config.get(),
        retry: false,
        refetchOnWindowFocus: false,
        enabled: enabled,
    });

export const PlexSettingsQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: PlexSettingsKeys.config(),
        queryFn: () => APIClient.plex.getSettings(),
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: false,
    });

export const MalQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: MalAuthKeys.config(),
        queryFn: () => APIClient.malauth.get(),
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: false,
    });

export const AnilistQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: AnilistAuthKeys.config(),
        queryFn: () => APIClient.anilistauth.get(),
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: false,
    });

export const MappingQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: MappingKeys.lists(),
        queryFn: () => APIClient.mapping.get(),
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: false,
    });

export const LogQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: LogKeys.lists(),
        queryFn: () => APIClient.fs.listLogs(),
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: true,
    });

export const LogContentQueryOptions = (enabled: boolean = true) =>
    queryOptions({
        queryKey: LogKeys.content(),
        queryFn: async () => {
            const response = await fetch(`${window.location.origin}${baseUrl()}api/fs/logs/shinkro.log`);
            if (!response.ok) throw new Error("Failed to fetch log");
            return response.text();
        },
        enabled: enabled,
        retry: false,
        refetchOnWindowFocus: false,
    });

export const NotificationsQueryOptions = () =>
    queryOptions({
        queryKey: NotificationKeys.lists(),
        queryFn: () => APIClient.notifications.getAll(),
    });

export const ApikeysQueryOptions = () =>
    queryOptions({
        queryKey: ApiKeys.lists(),
        queryFn: () => APIClient.apikeys.getAll(),
        refetchOnWindowFocus: false,
    });

export const plexHistoryQueryOptions = (opts: {
    limit?: number;
} = { limit: 5 }) =>
    queryOptions({
        queryKey: PlexKeys.history("timeline", opts),
        queryFn: () => APIClient.plex.history({ limit: opts.limit }),
        placeholderData: keepPreviousData,
        staleTime: 5000,
        refetchOnWindowFocus: true,
    });

export const plexCountsQueryOptions = () =>
    queryOptions({
        queryKey: PlexKeys.counts(),
        queryFn: () => APIClient.plex.getCounts(),
        staleTime: 5000,
        refetchOnWindowFocus: true,
    });

export const animeUpdateCountQueryOptions = () =>
    queryOptions({
        queryKey: ["animeUpdateCount"],
        queryFn: () => APIClient.animeupdate.getCount(),
        staleTime: 5000,
        refetchOnWindowFocus: true,
    });

export const recentAnimeUpdatesQueryOptions = (limit: number = 16) =>
    queryOptions({
        queryKey: ["recentAnimeUpdates", limit],
        queryFn: () => APIClient.animeupdate.getRecent(limit),
        staleTime: 5000,
        refetchOnWindowFocus: true,
    });

export const animeUpdateListQueryOptions = (params: {
    limit?: number;
    offset?: number;
    q?: string;
    status?: string;
    errorType?: string;
    source?: string;
}) =>
    queryOptions({
        queryKey: AnimeUpdateKeys.lists(params),
        queryFn: () => APIClient.animeupdate.getList(params),
        placeholderData: keepPreviousData,
        staleTime: 5000,
    });

export const latestReleaseQueryOptions = () =>
    queryOptions({
        queryKey: ["latestRelease"],
        queryFn: () => APIClient.updates.getLatestRelease(),
        staleTime: 1000 * 60 * 60, // 1 hour
    });

export const plexPayloadsQueryOptions = (
    pageIndex: number,
    pageSize: number,
    filters: ColumnFilter[]
) => {
    const params: {
        offset?: number;
        limit?: number;
        q?: string;
        event?: string;
        source?: string;
        status?: string;
    } = {
        offset: pageIndex * pageSize,
        limit: pageSize,
    };

    filters.forEach((filter) => {
        if (!filter.value) return;

        if (filter.id === "event") {
            if (typeof filter.value === "string") {
                params.event = filter.value;
            }
        } else if (filter.id === "source") {
            if (typeof filter.value === "string") {
                params.source = filter.value;
            }
        } else if (filter.id === "status") {
            if (typeof filter.value === "string") {
                params.status = filter.value;
            }
        } else if (filter.id === "payload") {
            if (typeof filter.value === "string") {
                params.q = filter.value;
            }
        }
    });

    return queryOptions({
        queryKey: PlexKeys.history("table", { pageIndex, pageSize, filters }),
        queryFn: () => APIClient.plex.findPayloads(params),
        placeholderData: keepPreviousData,
        staleTime: 5000,
    });
};
