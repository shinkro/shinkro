import {Link} from "@tanstack/react-router";
import {Anchor, Badge, Card, Group, Stack, Text, Timeline, TimelineItem, Alert, List} from "@mantine/core";
import {formatDistanceToNow} from "date-fns";
import {FaExclamationCircle, FaInfoCircle} from "react-icons/fa";
import type { PlexHistoryItem, AnimeUpdateErrorType, PlexErrorType } from "@app/types/Plex";
import { formatStatusLabel, formatEventName } from "@utils/index";

export type RecentTimelineProps = {
    timelineItems: PlexHistoryItem[];
};

function formatAnimeUpdateErrorType(errorType?: AnimeUpdateErrorType): string {
    if (!errorType) return "Unknown Error";
    
    const errorLabels: Record<AnimeUpdateErrorType, string> = {
        "MAL_AUTH_FAILED": "MAL Authentication Failed",
        "MAPPING_NOT_FOUND": "Mapping Not Found",
        "ANIME_NOT_IN_DB": "Anime Not in Database",
        "MAL_API_FETCH_FAILED": "MAL API Fetch Failed",
        "MAL_API_UPDATE_FAILED": "MAL API Update Failed",
        "UNKNOWN_ERROR": "Unknown Error",
    };
    
    return errorLabels[errorType] || errorType;
}

function getAnimeUpdateErrorMessage(errorType?: AnimeUpdateErrorType): string | null {
    if (!errorType) return null;
    
    const helpMessages: Partial<Record<AnimeUpdateErrorType, string>> = {
        "MAL_AUTH_FAILED": "This could be a MAL API issue. Try re-authenticating your MyAnimeList account in the settings.",
        "MAPPING_NOT_FOUND": "Try updating the community/custom mapping.",
        "ANIME_NOT_IN_DB": "This anime wasn't found in the internal database. Try updating the community/custom mapping.",
        "MAL_API_FETCH_FAILED": "The MAL API might be temporarily down.",
        "MAL_API_UPDATE_FAILED": "The MAL API might be temporarily down.",
    };
    
    return helpMessages[errorType] || null;
}

function formatPlexErrorType(errorType?: PlexErrorType): string {
    if (!errorType) return "Unknown Error";
    
    const errorLabels: Record<PlexErrorType, string> = {
        "AGENT_NOT_SUPPORTED": "Metadata Agent Not Supported",
        "EXTRACTION_FAILED": "Metadata Extraction Failed",
        "UNKNOWN_ERROR": "Unknown Error",
    };
    
    return errorLabels[errorType] || errorType;
}

function getPlexErrorMessage(errorType?: PlexErrorType): string | null {
    if (!errorType) return null;
    
    const helpMessages: Partial<Record<PlexErrorType, string>> = {
        "AGENT_NOT_SUPPORTED": "Switch to a supported metadata agent in your Plex library settings like HAMA, MyAnimeList.bundle, or Plex's default.",
        "EXTRACTION_FAILED": "Please open an issue on GitHub or contact us on Discord with shinkro logs.",
    };
    
    return helpMessages[errorType] || null;
}

export function RecentTimeline({timelineItems}: RecentTimelineProps) {
    if (!timelineItems || timelineItems.length === 0) {
        return <Text>No recent activity found.</Text>;
    }

    return (
        <Timeline active={-1} bulletSize={24} lineWidth={2}>
            {timelineItems.map((item) => {
                const plex = item.plex;
                const animeUpdate = item.animeUpdate;
                
                // Read status from consolidated fields
                const plexSuccess = plex?.success;
                const animeUpdateStatusValue = animeUpdate?.status;
                
                // MAL success requires a recorded successful update. Plex receipt alone is separate.
                const isSuccess = plexSuccess === true && 
                                 animeUpdateStatusValue === "SUCCESS";
                const isFailed = plexSuccess === false || animeUpdateStatusValue === "FAILED";
                
                const score = animeUpdate?.listStatus?.score;
                const statusText = animeUpdate?.listStatus?.status;
                
                // Get anime title from various sources
                const animeTitle = animeUpdate?.listDetails?.title ||
                                 plex?.Metadata?.grandparentTitle || 
                                 plex?.Metadata?.title || 
                                 'Unknown Title';
                
                return (
                    <TimelineItem key={plex.id}>
                        <Card shadow="xs" padding="sm" radius="md" withBorder>
                            <Group>
                                <Text fw={700}>{animeTitle}</Text>
                                <Link to="/sync/events/$eventId" params={{eventId:String(plex.id)}}>Inspect sync</Link>
                                <Text size="xs" c="dimmed">
                                    {plex.timestamp ? formatDistanceToNow(new Date(plex.timestamp), {addSuffix: true}) : "-"}
                                </Text>
                            </Group>
                            <Group gap="sm" mb={4} mt="xs">
                                {plex?.Metadata?.librarySectionTitle && (
                                    <Badge color="gray" variant="outline">
                                        {plex.Metadata.librarySectionTitle}
                                    </Badge>
                                )}
                                <Badge color="plex" variant="outline">
                                    {formatEventName(plex.event)}
                                </Badge>
                                {isSuccess ? (
                                    <Badge color="green" variant="outline">Successful</Badge>
                                ) : isFailed ? (
                                    <Badge color="red" variant="outline">Failed</Badge>
                                ) : (
                                    <Badge color="yellow" variant="outline">MAL result not recorded</Badge>
                                )}
                            </Group>
                            
                            {/* Success case - show anime update details */}
                            {isSuccess && animeUpdate && (
                                <Stack gap={2} mt={4}>
                                    <Text size="sm" fw={700}>MyAnimeList Update Details</Text>
                                    <Text size="sm">
                                        Progress: {animeUpdate?.listStatus?.num_episodes_watched}/{animeUpdate?.listDetails?.totalEpisodeNum || '?'}
                                    </Text>
                                    <Text size="sm">
                                        Score: {typeof score === 'number' && score > 0 ? score : 'Not Scored'}
                                    </Text>
                                    {statusText && (
                                        <Text size="sm">Status: {formatStatusLabel(statusText, true)}</Text>
                                    )}
                                    {(animeUpdate.malid && animeUpdate.malid > 0) && (
                                        <Anchor 
                                            href={`https://myanimelist.net/anime/${animeUpdate.malid}`}
                                            target="_blank" 
                                            underline="hover" 
                                            c="mal" 
                                            fw={700}
                                        >
                                            View on MAL
                                        </Anchor>
                                    )}
                                </Stack>
                            )}
                            
                            {/* Failed case - show detailed error information */}
                            {isFailed && (
                                <Stack gap="xs" mt={4}>
                                    {/* Plex errors (metadata extraction, agent issues) */}
                                    {plexSuccess === false && plex?.errorType && (
                                        <Stack gap="xs">
                                            <Stack gap={4}>
                                                <Alert 
                                                    icon={<FaExclamationCircle size={16} />} 
                                                    title={formatPlexErrorType(plex.errorType)}
                                                    color="red" 
                                                    variant="light"
                                                >
                                                    {plex.errorMsg && (
                                                        <Text size="sm">{plex.errorMsg}</Text>
                                                    )}
                                                </Alert>
                                            </Stack>
                                            
                                            {getPlexErrorMessage(plex.errorType) && (
                                                <Alert 
                                                    icon={<FaInfoCircle size={16} />}
                                                    color="blue" 
                                                    variant="light"
                                                >
                                                    <Text size="sm">{getPlexErrorMessage(plex.errorType)}</Text>
                                                </Alert>
                                            )}
                                            
                                            <Text size="xs" fw={700}>
                                                Error occurred during Plex metadata processing.
                                            </Text>
                                        </Stack>
                                    )}
                                    
                                    {/* AnimeUpdate errors (MAL API, mapping, etc.) */}
                                    {animeUpdate?.status === "FAILED" && animeUpdate?.errorType && (
                                        <Stack gap="xs">
                                            <Stack gap={4}>
                                                <Alert 
                                                    icon={<FaExclamationCircle size={16} />} 
                                                    title={formatAnimeUpdateErrorType(animeUpdate.errorType)}
                                                    color="red" 
                                                    variant="light"
                                                >
                                                    {animeUpdate.errorMessage && (
                                                        <Text size="sm">{animeUpdate.errorMessage}</Text>
                                                    )}
                                                </Alert>
                                            </Stack>
                                            
                                            {/* Plex Payload Details - only show if we have both sourceDB and sourceID */}
                                            {(animeUpdate.sourceDB && typeof animeUpdate.sourceID === 'number' && animeUpdate.sourceID > 0) && (
                                                <Stack gap={4}>
                                                    <Text size="sm" fw={700}>Plex Payload Details</Text>
                                                    <List size="sm" spacing="xs">
                                                        {Boolean(animeUpdate.malid && animeUpdate.malid > 0) && (
                                                            <List.Item>
                                                                <Group gap="xs">
                                                                    <Text size="sm" fw={700}>MAL ID:</Text>
                                                                    <Anchor 
                                                                        href={`https://myanimelist.net/anime/${animeUpdate.malid}`}
                                                                        target="_blank"
                                                                        size="sm"
                                                                    >
                                                                        {animeUpdate.malid}
                                                                    </Anchor>
                                                                </Group>
                                                            </List.Item>
                                                        )}
                                                        <List.Item>
                                                            <Text size="sm">
                                                                <Text component="span" fw={700}>Source: </Text>
                                                                {animeUpdate.sourceDB} ID: {animeUpdate.sourceID}
                                                            </Text>
                                                        </List.Item>
                                                        {animeUpdate.seasonNum !== undefined && plex?.Metadata?.type !== "movie" && (
                                                            <List.Item>
                                                                <Text size="sm">
                                                                    <Text component="span" fw={700}>Season: </Text>
                                                                    {animeUpdate.seasonNum}
                                                                </Text>
                                                            </List.Item>
                                                        )}
                                                        {animeUpdate.episodeNum !== undefined && plex?.Metadata?.type !== "movie" && (
                                                            <List.Item>
                                                                <Text size="sm">
                                                                    <Text component="span" fw={700}>Episode: </Text>
                                                                    {animeUpdate.episodeNum}
                                                                </Text>
                                                            </List.Item>
                                                        )}
                                                    </List>
                                                </Stack>
                                            )}
                                            
                                            {getAnimeUpdateErrorMessage(animeUpdate.errorType) && (
                                                <Alert 
                                                    icon={<FaInfoCircle size={16} />}
                                                    color="blue" 
                                                    variant="light"
                                                >
                                                    <Text size="sm">{getAnimeUpdateErrorMessage(animeUpdate.errorType)}</Text>
                                                </Alert>
                                            )}
                                            
                                            <Text size="xs" fw={700}>
                                                Error occurred during MyAnimeList update.
                                            </Text>
                                        </Stack>
                                    )}
                                    
                                    {/* Fallback for old error messages without errorType */}
                                    {plexSuccess === false && !plex?.errorType && plex?.errorMsg && (
                                        <Alert 
                                            icon={<FaExclamationCircle size={16} />} 
                                            title="Processing Error"
                                            color="red" 
                                            variant="light"
                                        >
                                            <Text size="sm">{plex.errorMsg}</Text>
                                        </Alert>
                                    )}
                                </Stack>
                            )}
                            
                            {/* No status at all */}
                            {!animeUpdate && !plex?.errorMsg && (
                                <Text size="sm" c="dimmed" mt={4}>
                                    No MyAnimeList update for this event.
                                </Text>
                            )}
                        </Card>
                    </TimelineItem>
                );
            })}
        </Timeline>
    );
}
