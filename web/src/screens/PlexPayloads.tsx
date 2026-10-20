import { useState, useMemo, useEffect, useRef, useLayoutEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
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
} from "@mantine/core";
import { AuthContext } from "@utils/Context";
import { Navigate } from "@tanstack/react-router";
import { PlexPayloadsRoute } from "@app/routes";
import { plexPayloadsQueryOptions } from "@api/queries";
import { PlexKeys } from "@api/query_keys";
import type { PlexPayloadListItem } from "@app/types/Plex";
import { displayNotification } from "@components/notifications";
import { formatEventName } from "@utils";
import { APIClient } from "@api/APIClient";
import { AgeCell, StatusBadge, TablePagination, ActionsCell, ViewDetailsModal, InfoTooltip } from "@components/table";
import { useDisclosure } from "@mantine/hooks";

const TruncatedPlexTitle = ({ value }: { value: string }) => {
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

function PlexPayloadsTableContent({
    pagination,
    setPagination,
    columnFilters,
    setColumnFilters,
    highlightedRowId,
    columns,
}: {
    pagination: PaginationState;
    setPagination: (state: PaginationState) => void;
    columnFilters: ColumnFilter[];
    setColumnFilters: (filters: ColumnFilter[]) => void;
    highlightedRowId: number | null;
    columns: ColumnDef<PlexPayloadListItem>[];
}) {
    const { data, error } = useQuery(
        plexPayloadsQueryOptions(pagination.pageIndex, pagination.pageSize, columnFilters)
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
        return <Text c="red">Error loading payloads</Text>;
    }

    return (
        <>
            <Table.ScrollContainer minWidth={800}>
                <Table highlightOnHover>
                    <Table.Thead>
                        {tableInstance.getHeaderGroups().map((headerGroup) => (
                            <Table.Tr key={headerGroup.id}>
                                {headerGroup.headers.map((header) => {
                                    const shouldCenter = header.column.id !== "payload";
                                    return (
                                        <Table.Th key={header.id} style={{ textAlign: shouldCenter ? "center" : "left" }}>
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
                                    <Text c="dimmed">No payloads found</Text>
                                </Table.Td>
                            </Table.Tr>
                        ) : (
                            tableInstance.getRowModel().rows.map((row) => {
                                const plexId = row.original.plex.id;
                                const isHighlighted = highlightedRowId !== null && plexId === highlightedRowId;
                                const highlightColor = isHighlighted 
                                    ? "rgba(100, 116, 139, 0.15)"
                                    : "transparent";
                                return (
                                    <Table.Tr 
                                        key={row.id}
                                        style={{
                                            backgroundColor: highlightColor,
                                            transition: "background-color 2s ease-out",
                                        }}
                                    >
                                        {row.getVisibleCells().map((cell) => {
                                            const shouldCenter = cell.column.id !== "payload";
                                            const isSourceColumn = cell.column.id === "source";
                                            return (
                                                <Table.Td 
                                                    key={cell.id} 
                                                    style={{ 
                                                        textAlign: shouldCenter ? "center" : "left",
                                                        whiteSpace: isSourceColumn ? "nowrap" : "normal"
                                                    }}
                                                >
                                                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                                                </Table.Td>
                                            );
                                        })}
                                    </Table.Tr>
                                );
                            })
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

export const PlexPayloads = () => {
    const isLoggedIn = AuthContext.useSelector((s) => s.isLoggedIn);
    if (!isLoggedIn) {
        return <Navigate to="/login" />;
    }

    const [pagination, setPagination] = useState<PaginationState>({
        pageIndex: 0,
        pageSize: 10,
    });

    const [columnFilters, setColumnFilters] = useState<ColumnFilter[]>([]);
    const [selectedPayload, setSelectedPayload] = useState<PlexPayloadListItem | null>(null);
    const [opened, { open, close }] = useDisclosure(false);
    const search = useSearch({ from: PlexPayloadsRoute.id, strict: true });
    const highlightId = search.highlight ? parseInt(search.highlight, 10) : null;
    const [highlightedRowId, setHighlightedRowId] = useState<number | null>(highlightId);
    const queryClient = useQueryClient();

    useEffect(() => {
        if (highlightId !== null) {
            setHighlightedRowId(highlightId);
            const timer = setTimeout(() => {
                setHighlightedRowId(null);
            }, 2000);
            return () => clearTimeout(timer);
        }
    }, [highlightId]);

    const deleteMutation = useMutation({
        mutationFn: (id: number) => APIClient.plex.deletePayload(id),
        onSuccess: () => {
            displayNotification({
                title: "Success",
                message: "Plex payload deleted successfully",
                type: "success",
            });
            // Invalidate all related queries
            queryClient.invalidateQueries({ queryKey: PlexKeys.lists() });
            queryClient.invalidateQueries({ queryKey: PlexKeys.timeline() });
            queryClient.invalidateQueries({ queryKey: PlexKeys.counts() });
        },
        onError: (error: Error) => {
            displayNotification({
                title: "Error",
                message: error.message || "Failed to delete plex payload",
                type: "error",
            });
        },
    });

    const columns = useMemo<ColumnDef<PlexPayloadListItem>[]>(
        () => [
            {
                header: "Age",
                accessorKey: "plex.timestamp",
                cell: ({ row }) => (
                    <AgeCell timestamp={row.original.plex.timestamp} />
                ),
            },
            {
                header: "Plex Payload",
                accessorKey: "payload",
                id: "payload",
                cell: ({ row }) => {
                    const plex = row.original.plex;
                    const title = plex.Metadata?.grandparentTitle || plex.Metadata?.title || "Unknown";
                    const library = plex.Metadata?.librarySectionTitle || "";
                    const event = plex.event || "";

                    let season = "";
                    let episode = "";
                    const isMovie = plex.Metadata?.type === "movie";
                    if (plex.Metadata && !isMovie) {
                        if (plex.Metadata.parentIndex !== undefined && plex.Metadata.parentIndex > 0) {
                            season = `S${plex.Metadata.parentIndex}`;
                        }
                        if (plex.Metadata.index !== undefined && plex.Metadata.index > 0) {
                            episode = `E${plex.Metadata.index}`;
                        }
                    }

                    return (
                        <Stack gap={4} style={{ maxWidth: "500px" }}>
                            <TruncatedPlexTitle value={title} />
                            <Group gap="xs">
                                {library && (
                                    <Badge size="xs" variant="transparent" color="gray">
                                        {library}
                                    </Badge>
                                )}
                                {event && (
                                    <Badge size="xs" variant="transparent" color="plex">
                                        {formatEventName(event)}
                                    </Badge>
                                )}
                                {season && episode && (
                                    <Badge size="xs" variant="transparent">
                                        {season} {episode}
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
                    const plexId = row.original.plex.id;
                    return (
                        <ActionsCell
                            inspectId={plexId}
                            onView={() => {
                                setSelectedPayload(row.original);
                                open();
                            }}
                            onDelete={() => deleteMutation.mutate(plexId)}
                            deleteTitle="Delete Plex Payload"
                            deleteMessage="This will also delete the related anime update record if it exists."
                            isDeleting={deleteMutation.isPending}
                        />
                    );
                },
            },
            {
                header: "Processing Status",
                accessorKey: "status",
                id: "status",
                cell: ({ row }) => {
                    const plex = row.original.plex;
                    return (
                        <StatusBadge
                            status={plex?.success}
                            errorType={plex?.errorType}
                            errorMessage={plex?.errorMsg}
                            variant="icon"
                        />
                    );
                },
            },
            {
                header: "Source Type",
                accessorKey: "plex.source",
                id: "source",
                cell: ({ row }) => {
                    const source = row.original.plex.source || "";
                    return (
                        <Badge variant="transparent" color={source === "Plex Webhook" ? "plex" : "blue"}>
                            {source || "Unknown"}
                        </Badge>
                    );
                },
            },
        ],
        []
    );

    return (
        <Container size={1200} px="md" component="main">
            <Stack gap="md" mt="md">
            <Title order={2}>Plex Payloads</Title>
                {/* Filters */}
                <Paper mt="md" withBorder p="md">
                <Stack gap="md">
                <Group>
                    <TextInput
                        placeholder="Search by title"
                        value={(columnFilters.find((f) => f.id === "payload")?.value as string) || ""}
                        onChange={(e) => {
                            const filter = columnFilters.filter((f) => f.id !== "payload");
                            if (e.target.value) {
                                filter.push({ id: "payload", value: e.target.value });
                            }
                            setColumnFilters(filter);
                        }}
                        style={{ flex: 1 }}
                    />
                    <Select
                        placeholder="Event"
                        data={[
                            { value: "media.scrobble", label: "Scrobble" },
                            { value: "media.rate", label: "Rate" },
                        ]}
                        value={(columnFilters.find((f) => f.id === "event")?.value as string) || ""}
                        onChange={(value) => {
                            const filter = columnFilters.filter((f) => f.id !== "event");
                            if (value) {
                                filter.push({ id: "event", value });
                            }
                            setColumnFilters(filter);
                        }}
                        clearable
                    />
                    <Select
                        placeholder="Source"
                        data={[
                            { value: "Plex Webhook", label: "Plex Webhook" },
                            { value: "Tautulli", label: "Tautulli" },
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
                    <Select
                        placeholder="Status"
                        data={[
                            { value: "success", label: "Success" },
                            { value: "failed", label: "Failed" },
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
                </Group>

                {/* Table */}
                <PlexPayloadsTableContent
                    pagination={pagination}
                    setPagination={setPagination}
                    columnFilters={columnFilters}
                    setColumnFilters={setColumnFilters}
                    highlightedRowId={highlightedRowId}
                    columns={columns}
                />
            </Stack>
            </Paper>
            </Stack>

            {/* Payload View Modal */}
            <ViewDetailsModal
                opened={opened}
                onClose={close}
                title="Plex Payload"
                data={selectedPayload}
            />
        </Container>
    );
};

