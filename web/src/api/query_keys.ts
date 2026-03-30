export const SettingsKeys = {
    all: ["settings"] as const,
    updates: () => [...SettingsKeys.all, "updates"] as const,
    config: () => [...SettingsKeys.all, "config"] as const,
    lists: () => [...SettingsKeys.all, "list"] as const,
};

export const PlexSettingsKeys = {
    all: ["plex_settings"] as const,
    config: () => [...PlexSettingsKeys.all, "config"] as const,
}

export const MalAuthKeys = {
    all: ["mal_auth"] as const,
    config: () => [...MalAuthKeys.all, "config"] as const,
}

export const AnilistAuthKeys = {
    all: ["anilist_auth"] as const,
    config: () => [...AnilistAuthKeys.all, "config"] as const,
}

export const ApiKeys = {
    all: ["api_keys"] as const,
    lists: () => [...ApiKeys.all, "list"] as const,
    details: () => [...ApiKeys.all, "detail"] as const,
    detail: (id: string) => [...ApiKeys.details(), id] as const,
};

export const MappingKeys = {
    all: ["mappings"] as const,
    lists: () => [...MappingKeys.all, "list"] as const,
};

export const LogKeys = {
    all: ["logs"] as const,
    lists: () => [...LogKeys.all, "list"] as const,
    content: () => [...LogKeys.all, "content"] as const,
}

export const NotificationKeys = {
    all: ["notifications"] as const,
    lists: () => [...NotificationKeys.all, "list"] as const,
    details: () => [...NotificationKeys.all, "detail"] as const,
    detail: (id: number) => [...NotificationKeys.details(), id] as const,
};

export const PlexKeys = {
    all: ["plex"] as const,
    lists: () => [...PlexKeys.all, "history", "table"] as const,
    timeline: () => [...PlexKeys.all, "history", "timeline"] as const,
    counts: () => [...PlexKeys.all, "counts"] as const,
    history: (type: "timeline" | "table", params: Record<string, unknown>) => [
        ...PlexKeys.all,
        "history",
        type,
        params,
    ] as const,
    payloads: (params: Record<string, unknown>) => [
        ...PlexKeys.all,
        "payloads",
        params,
    ] as const,
};

export const AnimeUpdateKeys = {
    all: ["animeUpdate"] as const,
    lists: (params: Record<string, unknown>) => [
        ...AnimeUpdateKeys.all,
        "list",
        params,
    ] as const,
};
