import { useState, useMemo, useRef, useLayoutEffect, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import {
    useReactTable,
    getCoreRowModel,
    flexRender,
    type ColumnDef,
    type ColumnFilter,
    type PaginationState,
} from "@tanstack/react-table";
import {
    Container,
    Paper,
    Stack,
    Title,
    Table,
    Badge,
    Group,
    Text,
    Select,
    TextInput,
    Button,
    ActionIcon,
} from "@mantine/core";
import { FaLink } from "react-icons/fa";
import { AuthContext } from "@utils/Context";
import { Navigate } from "@tanstack/react-router";
import { animeUpdateListQueryOptions } from "@api/queries";
import type { AnimeUpdateListItem } from "@app/types/Anime";
import { AgeCell, StatusBadge, SourceBadge, TablePagination, ActionsCell, ViewDetailsModal, InfoTooltip } from "@components/table";
import { useDisclosure } from "@mantine/hooks";
import { getMALLink } from "@utils/sourceLinks";

const TruncatedTitle = ({ value }: { value: string }) => {
    const textRef = useRef<HTMLDivElement | null>(null);
    const [isTruncated, setIsTruncated] = useState(false);

    const checkTruncation = (element: HTMLDivElement | null) => {
        if (element) {
            requestAnimationFrame(() => {
                const isOverflowing = element.scrollWidth > element.clientWidth;
                setIsTruncated(isOverflowing);
            });
        }
    };

    const setRef = (element: HTMLDivElement | null) => {
        textRef.current = element;
        checkTruncation(element);
    };

    useLayoutEffect(() => {
        checkTruncation(textRef.current);
    }, [value]);

    useEffect(() => {
        const handleResize = () => {
            checkTruncation(textRef.current);
        };
        window.addEventListener("resize", handleResize);
        return () => window.removeEventListener("resize", handleResize);
    }, []);

    const textElement = (
        <Text
            ref={setRef}
            fw={600}
            size="sm"
            style={{
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                maxWidth: "100%",
            }}
        >
            {value}
        </Text>
    );

    if (isTruncated) {
        return <InfoTooltip label={value}>{textElement}</InfoTooltip>;
    }

    return textElement;
};

function AnimeUpdatesTableContent({
    pagination,
    setPagination,
    columnFilters,
    setColumnFilters,
    setSelectedUpdate,
    open,
}: {
    pagination: PaginationState;
    setPagination: (state: PaginationState) => void;
    columnFilters: ColumnFilter[];
    setColumnFilters: (filters: ColumnFilter[]) => void;
    setSelectedUpdate: (update: AnimeUpdateListItem | null) => void;
    open: () => void;
}) {
    const queryParams = useMemo(() => {
        const params: {
            limit?: number;
            offset?: number;
            q?: string;
            status?: string;
            errorType?: string;
            source?: string;
        } = {
            limit: pagination.pageSize,
            offset: pagination.pageIndex * pagination.pageSize,
        };

        const searchFilter = columnFilters.find((f) => f.id === "search");
        if (searchFilter?.value) {
            params.q = searchFilter.value as string;
        }

        const statusFilter = columnFilters.find((f) => f.id === "status");
        if (statusFilter?.value) {
            params.status = statusFilter.value as string;
        }

        const errorTypeFilter = columnFilters.find((f) => f.id === "errorType");
        if (errorTypeFilter?.value) {
            params.errorType = errorTypeFilter.value as string;
        }

        const sourceFilter = columnFilters.find((f) => f.id === "source");
        if (sourceFilter?.value) {
            params.source = sourceFilter.value as string;
        }

        return params;
    }, [pagination, columnFilters]);

    const { data, error } = useQuery(
        animeUpdateListQueryOptions(queryParams)
    );

    const columns = useMemo<ColumnDef<AnimeUpdateListItem>[]>(
        () => [
            {
                header: "Age",
                accessorKey: "animeUpdate.timestamp",
                cell: ({ row }) => (
                    <AgeCell timestamp={row.original.animeUpdate.timestamp} />
                ),
            },
            {
                header: "Anime Update",
                accessorKey: "animeUpdate",
                id: "animeUpdate",
                cell: ({ row }) => {
                    const update = row.original.animeUpdate;
                    const isFailed = update.status === "FAILED";
                    const title = update.listDetails?.title || "Unknown";
                    const malId = update.malid;
                    const watchedNum = update.listDetails?.watchedNum;
                    const totalEpisodeNum = update.listDetails?.totalEpisodeNum;
                    const score = update.listStatus?.score;
                    const episodeNum = update.episodeNum;

                    const malLink = !isFailed && malId > 0 ? getMALLink(malId) : null;

                    return (
                        <Stack gap={4} style={{ maxWidth: "500px" }}>
                            <TruncatedTitle value={title} />
                            <Group gap="xs">
                                {!isFailed && malId > 0 && malLink && (
                                    <Button
                                        component="a"
                                        href={malLink}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        variant="subtle"
                                        size="compact-xs"
                                        color="blue"
                                        style={{
                                            height: 16,
                                            padding: "0 4px",
                                            fontSize: "9px",
                                            lineHeight: 1.1,
                                        }}
                                    >
                                        MAL: {malId}
                                    </Button>
                                )}
                                {!isFailed && watchedNum !== undefined && totalEpisodeNum !== undefined && (
                                    <Badge size="xs" variant="transparent" color="gray">
                                        {watchedNum}/{totalEpisodeNum === 0 ? "?" : totalEpisodeNum}
                                    </Badge>
                                )}
                                {!isFailed && score !== undefined && score > 0 && (
                                    <Badge size="xs" variant="transparent" color="yellow">
                                        ⭐ {score}
                                    </Badge>
                                )}
                                {!isFailed && episodeNum > 0 && (
                                    <Badge size="xs" variant="transparent">
                                        E{episodeNum}
                                    </Badge>
                                )}
                            </Group>
                        </Stack>
                    );
                },
                meta: {
                    filterVariant: "search",
                },
            },
            {
                header: "Actions",
                accessorKey: "actions",
                cell: ({ row }) => {
                    const update = row.original.animeUpdate;
                    const plexPayloadLink = update.plexID ? `/plex-payloads?highlight=${update.plexID}` : null;
                    
                    return (
                        <Group gap="xs" justify="center">
                            <ActionsCell
                                inspectId={update.plexID}
                                onView={() => {
                                    setSelectedUpdate(row.original);
                                    open();
                                }}
                            />
                            {plexPayloadLink && (
                                <InfoTooltip label="View Plex Payload">
                                    <ActionIcon
                                        component="a"
                                        href={plexPayloadLink}
                                        variant="outline"
                                    >
                                        <FaLink size={16} />
                                    </ActionIcon>
                                </InfoTooltip>
                            )}
                        </Group>
                    );
                },
            },
            {
                header: "Status",
                accessorKey: "animeUpdate.status",
                id: "status",
                cell: ({ row }) => {
                    const update = row.original.animeUpdate;
                    return (
                        <StatusBadge
                            status={update.status}
                            errorType={update.errorType}
                            errorMessage={update.errorMessage}
                            variant="icon"
                        />
                    );
                },
            },
            {
                header: "Source",
                accessorKey: "animeUpdate.sourceDB",
                id: "source",
                cell: ({ row }) => {
                    const update = row.original.animeUpdate;
                    return (
                        <SourceBadge
                            sourceDB={update.sourceDB}
                            sourceID={update.sourceID}
                            variant="button"
                        />
                    );
                },
            },
        ],
        [setSelectedUpdate, open]
    );

    const tableInstance = useReactTable({
        columns,
        data: data?.data || [],
        getCoreRowModel: getCoreRowModel(),
        manualFiltering: true,
        manualPagination: true,
        rowCount: data?.count || 0,
        state: {
            columnFilters,
            pagination,
        },
        onPaginationChange: (updater) => {
            setPagination(typeof updater === "function" ? updater(pagination) : updater);
        },
        onColumnFiltersChange: (updater) => {
            setColumnFilters(typeof updater === "function" ? updater(columnFilters) : updater);
        },
    });

    if (error) {
        return <Text c="red">Error loading anime updates</Text>;
    }

    return (
        <>
            <Table.ScrollContainer minWidth={800}>
                <Table highlightOnHover>
                    <Table.Thead>
                        {tableInstance.getHeaderGroups().map((headerGroup) => (
                            <Table.Tr key={headerGroup.id}>
                                {headerGroup.headers.map((header) => {
                                    const shouldCenter = header.column.id !== "animeUpdate";
                                    return (
                                        <Table.Th
                                            key={header.id}
                                            style={{ textAlign: shouldCenter ? "center" : "left" }}
                                        >
                                            {header.isPlaceholder
                                                ? null
                                                : flexRender(header.column.columnDef.header, header.getContext())}
                                        </Table.Th>
                                    );
                                })}
                            </Table.Tr>
                        ))}
                    </Table.Thead>
                    <Table.Tbody>
                        {tableInstance.getRowModel().rows.length === 0 ? (
                            <Table.Tr>
                                <Table.Td colSpan={columns.length} style={{ textAlign: "center" }}>
                                    <Text>No anime updates found</Text>
                                </Table.Td>
                            </Table.Tr>
                        ) : (
                            tableInstance.getRowModel().rows.map((row) => (
                                <Table.Tr key={row.id}>
                                    {row.getVisibleCells().map((cell) => {
                                        const shouldCenter = cell.column.id !== "animeUpdate";
                                        return (
                                            <Table.Td
                                                key={cell.id}
                                                style={{ textAlign: shouldCenter ? "center" : "left" }}
                                            >
                                                {flexRender(cell.column.columnDef.cell, cell.getContext())}
                                            </Table.Td>
                                        );
                                    })}
                                </Table.Tr>
                            ))
                        )}
                    </Table.Tbody>
                </Table>
            </Table.ScrollContainer>

            {data && data.count > 0 && (
                <TablePagination
                    table={tableInstance}
                    totalCount={data.count}
                />
            )}
        </>
    );
}

export const AnimeUpdates = () => {
    const isLoggedIn = AuthContext.useSelector((s) => s.isLoggedIn);
    if (!isLoggedIn) {
        return <Navigate to="/login" />;
    }

    const [pagination, setPagination] = useState<PaginationState>({
        pageIndex: 0,
        pageSize: 10,
    });

    const [columnFilters, setColumnFilters] = useState<ColumnFilter[]>([]);
    const [selectedUpdate, setSelectedUpdate] = useState<AnimeUpdateListItem | null>(null);
    const [opened, { open, close }] = useDisclosure(false);

    return (
        <Container size={1200} px="md" component="main">
            <Stack gap="md" mt="md">
                <Title order={2}>Anime Updates</Title>
                {/* Filters */}
                <Paper mt="md" withBorder p="md">
                    <Stack gap="md">
                        <Group>
                            <TextInput
                                placeholder="Search by title, MAL ID, or SourceDB:ID"
                                value={(columnFilters.find((f) => f.id === "search")?.value as string) || ""}
                                onChange={(e) => {
                                    const filter = columnFilters.filter((f) => f.id !== "search");
                                    if (e.target.value) {
                                        filter.push({ id: "search", value: e.target.value });
                                    }
                                    setColumnFilters(filter);
                                }}
                                style={{ flex: 1 }}
                            />
                            <Select
                                placeholder="Status"
                                data={[
                                    { value: "SUCCESS", label: "Success" },
                                    { value: "FAILED", label: "Failed" },
                                    { value: "PENDING", label: "Pending" },
                                ]}
                                value={(columnFilters.find((f) => f.id === "status")?.value as string) || ""}
                                onChange={(value) => {
                                    const filter = columnFilters.filter((f) => f.id !== "status");
                                    if (value) {
                                        filter.push({ id: "status", value });
                                    }
                                    setColumnFilters(filter);
                                }}
                                clearable
                            />
                            <Select
                                placeholder="Error Type"
                                data={[
                                    { value: "MAL_AUTH_FAILED", label: "MAL Auth Failed" },
                                    { value: "MAPPING_NOT_FOUND", label: "Mapping Not Found" },
                                    { value: "ANIME_NOT_IN_DB", label: "Anime Not In DB" },
                                    { value: "MAL_API_FETCH_FAILED", label: "MAL API Fetch Failed" },
                                    { value: "MAL_API_UPDATE_FAILED", label: "MAL API Update Failed" },
                                    { value: "UNKNOWN_ERROR", label: "Unknown Error" },
                                ]}
                                value={(columnFilters.find((f) => f.id === "errorType")?.value as string) || ""}
                                onChange={(value) => {
                                    const filter = columnFilters.filter((f) => f.id !== "errorType");
                                    if (value) {
                                        filter.push({ id: "errorType", value });
                                    }
                                    setColumnFilters(filter);
                                }}
                                clearable
                            />
                            <Select
                                placeholder="Source"
                                data={[
                                    { value: "tvdb", label: "TVDB" },
                                    { value: "tmdb", label: "TMDB" },
                                    { value: "anidb", label: "AniDB" },
                                    { value: "myanimelist", label: "MAL" },
                                ]}
                                value={(columnFilters.find((f) => f.id === "source")?.value as string) || ""}
                                onChange={(value) => {
                                    const filter = columnFilters.filter((f) => f.id !== "source");
                                    if (value) {
                                        filter.push({ id: "source", value });
                                    }
                                    setColumnFilters(filter);
                                }}
                                clearable
                            />
                        </Group>

                        {/* Table */}
                        <AnimeUpdatesTableContent
                            pagination={pagination}
                            setPagination={setPagination}
                            columnFilters={columnFilters}
                            setColumnFilters={setColumnFilters}
                            setSelectedUpdate={setSelectedUpdate}
                            open={open}
                        />
                    </Stack>
                </Paper>
            </Stack>

            {/* Anime Update Details Modal */}
            <ViewDetailsModal
                opened={opened}
                onClose={close}
                title="Anime Update Details"
                data={selectedUpdate}
            />
        </Container>
    );
};

