export interface PlexConfig {
    host: string;
    port: number;
    tls: boolean;
    tls_skip: boolean;
    client_id: string;
    anime_libs: string[];
    plex_user: string;
    plex_client_enabled: boolean;
}

export interface PlexOAuthStartResponse {
    pin_id: number;
    code: string;
    client_id: string;
    auth_url: string;
}

export interface PlexOAuthPollResponse {
    token: string;
    plex_user: string;
    client_id: string;
    message: string;
}

export interface PlexConnection {
    protocol: string;
    address: string;
    port: number;
    uri: string;
    local: boolean;
    relay: boolean;
    IPv6: boolean;
}

export interface PlexServerResponse {
    Servers: PlexServer[];
}

export interface PlexServer {
    name: string;
    product: string;
    productVersion: string;
    platform: string | null;
    platformVersion: string | null;
    device: string | null;
    clientIdentifier: string;
    createdAt: string; // or Date if you want to parse it
    lastSeenAt: string;
    provides: string;
    ownerId: number | null;
    sourceTitle: string | null;
    publicAddress: string;
    accessToken: string;
    owned: boolean;
    home: boolean;
    synced: boolean;
    relay: boolean;
    presence: boolean;
    httpsRequired: boolean;
    publicAddressMatches: boolean;
    dnsRebindingProtection: boolean;
    natLoopbackSupported: boolean;
    connections: PlexConnection[];
}

export interface PlexLibraryResponse {
    MediaContainer: {
        size: number;
        allowSync: boolean;
        title1: string;
        Directory: PlexLibrary[];
    };
}

export interface PlexLibrary {
    allowSync: boolean;
    art: string;
    composite: string;
    filters: boolean;
    refreshing: boolean;
    thumb: string;
    key: string;
    type: string;
    title: string;
    agent: string;
    scanner: string;
    language: string;
    uuid: string;
    updatedAt: number;
    createdAt: number;
    scannedAt: number;
    content: boolean;
    directory: boolean;
    contentChangedAt: number;
    hidden: number;
    Location: PlexLibraryLocation[];
}

export interface PlexLibraryLocation {
    id: number;
    path: string;
}


export interface PlexMetadataMinimal {
    librarySectionTitle?: string;
    grandparentTitle?: string;
    title?: string;
    index?: number;
    parentIndex?: number;
    type?: string;
    guid?: string;
    guids?: Array<{ id: string }>;
}

export interface PlexPayloadMinimal {
    id: number;
    event: string;
    timestamp: string;
    source?: string;
    Metadata?: PlexMetadataMinimal;
    // Status fields (consolidated from plex_status table)
    success?: boolean | null;
    errorType?: PlexErrorType;
    errorMsg?: string;
    // Allow additional fields from backend without strict typing
    [key: string]: unknown;
}

export type PlexErrorType = 
    | "AGENT_NOT_SUPPORTED"
    | "EXTRACTION_FAILED"
    | "UNKNOWN_ERROR";

import type { TimelineAnimeUpdate } from "./Anime";

export type AnimeUpdateStatusType = "PENDING" | "SUCCESS" | "FAILED";

export type AnimeUpdateErrorType = 
    | "MAL_AUTH_FAILED"
    | "MAPPING_NOT_FOUND"
    | "ANIME_NOT_IN_DB"
    | "MAL_API_FETCH_FAILED"
    | "MAL_API_UPDATE_FAILED"
    | "UNKNOWN_ERROR";

export interface PlexHistoryItem {
    plex: PlexPayloadMinimal;
    animeUpdate?: TimelineAnimeUpdate;
}

export interface PlexPayloadListItem {
    plex: PlexPayloadMinimal;
}

export interface FindPlexPayloadsResponse {
    data: PlexPayloadListItem[];
    count: number;
}
