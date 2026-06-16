import {
    Modal,
    TextInput,
    Switch,
    Button,
    Stack,
    Group,
    MultiSelect,
    Select,
} from "@mantine/core";
import {TfiReload} from "react-icons/tfi";
import {useForm} from "@mantine/form";
import {useEffect, useState, useRef} from "react";
import {
    PlexConfig,
    PlexServer,
    PlexLibrary,
    PlexServerResponse,
} from "@app/types/Plex";
import {APIClient} from "@api/APIClient";
import {displayNotification} from "@components/notifications";
import {useMutation, useQueryClient} from "@tanstack/react-query";
import {PlexSettingsKeys} from "@api/query_keys.ts";

interface Props {
    opened: boolean;
    onClose: () => void;
    defaultValues?: Partial<PlexConfig>;
}

export const PlexSettings = ({
                                 opened,
                                 onClose,
                                 defaultValues,
                             }: Props) => {
    const form = useForm<PlexConfig>({
        initialValues: {
            host: "",
            port: 0,
            tls: false,
            tls_skip: false,
            plex_user: "",
            anime_libs: [],
            plex_client_enabled: true,
            client_id: "",
            ...defaultValues,
        },
        validate: {
            host: (v: string) => (v ? null : "Host is required"),
            port: (v: number) => (v ? null : "Port is required"),
            plex_user: (v: string) => (v ? null : "Username is required"),
            anime_libs: (v: string[]) =>
                v.length === 0 ? "At least one anime library is required" : null,
            client_id: (v: string) => (v ? null : "Client ID is required"),
        },
    });

    const queryClient = useQueryClient();
    const [polling, setPolling] = useState(false);
    const [oauthInfo, setOauthInfo] = useState<{
        pin_id: number;
        client_id: string;
        code: string;
    } | null>(null);
    const isModalOpenRef = useRef(opened);
    const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
    const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Servers state and selected server (using Select)
    const [servers, setServers] = useState<PlexServerResponse | null>(null);
    const [authenticated, setAuthenticated] = useState(false);
    const [selectedServer, setSelectedServer] = useState<PlexServer | null>(null);
    const [loadingServers, setLoadingServers] = useState(false);

    // Libraries state for anime libraries
    const [libraries, setLibraries] = useState<PlexLibrary[]>([]);
    const [loadingLibraries, setLoadingLibraries] = useState(false);
    const [testingConnection, setTestingConnection] = useState(false);

    // Keep ref in sync with opened prop and stop polling when modal closes
    useEffect(() => {
        isModalOpenRef.current = opened;
        
        // Stop polling when modal closes
        if (!opened) {
            setPolling(false);
            setOauthInfo(null);
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
            if (timeoutRef.current) {
                clearTimeout(timeoutRef.current);
                timeoutRef.current = null;
            }
        }
    }, [opened]);

    useEffect(() => {
        if (defaultValues && Object.keys(defaultValues).length !== 0) {
            form.setValues(defaultValues);
        } else {
            form.reset();
            setServers(null);
            setAuthenticated(false);
            setSelectedServer(null);
        }
    }, [defaultValues]);

    const handlePlexLogin = async () => {
        try {
            await APIClient.plex.testToken()
            setAuthenticated(true);
            displayNotification(
                {
                    title: "Plex Already Authenticated",
                    message: "Choose a Server",
                    type: "info",
                }
            )
            return
        } catch (err) {
            setAuthenticated(false);
            const error = err as Error;
            console.error(error);
        }
        try {
            setPolling(true);
            const result = await APIClient.plex.startOAuth();
            setOauthInfo({
                pin_id: result.pin_id,
                client_id: result.client_id,
                code: result.code,
            });
            window.open(result.auth_url, "_blank");
        } catch (err) {
            const error = err as Error;
            setPolling(false);
            displayNotification({
                title: "OAuth Error",
                message: error.message || "Failed to start Plex login",
                type: "error",
            });
        }
    };

    useEffect(() => {
        if (!polling || !oauthInfo) return;
        
        // Don't start polling if modal is closed
        if (!isModalOpenRef.current) {
            setPolling(false);
            setOauthInfo(null);
            return;
        }
        
        let timeoutReached = false;
        timeoutRef.current = setTimeout(() => {
            timeoutReached = true;
            setPolling(false);
            setOauthInfo(null);
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
            // Only show timeout notification if modal is still open
            if (isModalOpenRef.current) {
                displayNotification({
                    title: "Timeout",
                    message: "Authentication with Plex timed out after 60 seconds",
                    type: "error",
                });
            }
        }, 60000);

        intervalRef.current = setInterval(async () => {
            // Stop polling if modal is closed or timeout reached
            if (timeoutReached || !isModalOpenRef.current) {
                if (intervalRef.current) {
                    clearInterval(intervalRef.current);
                    intervalRef.current = null;
                }
                if (timeoutRef.current) {
                    clearTimeout(timeoutRef.current);
                    timeoutRef.current = null;
                }
                setPolling(false);
                setOauthInfo(null);
                return;
            }
            
            try {
                const result = await APIClient.plex.pollOAuth(
                    oauthInfo.pin_id,
                    oauthInfo.client_id,
                    oauthInfo.code
                );
                if (result.message === "waiting for auth") {
                    return;
                }
                
                // Clear intervals
                if (intervalRef.current) {
                    clearInterval(intervalRef.current);
                    intervalRef.current = null;
                }
                if (timeoutRef.current) {
                    clearTimeout(timeoutRef.current);
                    timeoutRef.current = null;
                }
                
                // Only process success if modal is still open
                if (!isModalOpenRef.current) {
                    setPolling(false);
                    setOauthInfo(null);
                    return;
                }
                
                setPolling(false);
                setOauthInfo(null);
                setAuthenticated(true)
                form.setValues((prev) => ({
                    ...prev,
                    token: result.token,
                    plex_user: result.plex_user,
                    client_id: result.client_id,
                }));
                displayNotification({
                    title: "Plex Login Successful",
                    message: `Logged in as ${result.plex_user}`,
                    type: "success",
                });
            } catch (err) {
                // Clear intervals on error
                if (intervalRef.current) {
                    clearInterval(intervalRef.current);
                    intervalRef.current = null;
                }
                if (timeoutRef.current) {
                    clearTimeout(timeoutRef.current);
                    timeoutRef.current = null;
                }
                
                setAuthenticated(false);
                setPolling(false);
                setOauthInfo(null);
                
                // Only show error notification if modal is still open
                if (isModalOpenRef.current) {
                    const error = err as Error;
                    displayNotification({
                        title: "Plex Login Failed",
                        message: error.message || "Polling error",
                        type: "error",
                    });
                }
            }
        }, 1000);

        return () => {
            if (intervalRef.current) {
                clearInterval(intervalRef.current);
                intervalRef.current = null;
            }
            if (timeoutRef.current) {
                clearTimeout(timeoutRef.current);
                timeoutRef.current = null;
            }
        };
    }, [polling, oauthInfo, form]);

    useEffect(() => {
        if (authenticated) {
            void loadServers();
        }
    }, [authenticated]);

    useEffect(() => {
        if (servers && servers.Servers.length > 0) {
            setSelectedServer(servers.Servers[0]);
        }
    }, [servers]);

    useEffect(() => {
        if (selectedServer) {
            // Auto-populate from selected server, but allow manual editing
            form.setFieldValue("host", selectedServer.connections[0].address);
            form.setFieldValue("port", selectedServer.connections[0].port);
            form.setFieldValue("tls", selectedServer.connections[0].protocol === "https");
        }
    }, [selectedServer]);

    const loadServers = async () => {
        if (!form.getValues().client_id) {
            displayNotification({
                title: "Missing Credentials",
                message: "Please authenticate with Plex first.",
                type: "error",
            });
            return;
        }
        try {
            setLoadingServers(true);
            const response = await APIClient.plex.servers(form.getValues());
            const serverList = (response?.Servers ?? []).filter(
                (server: PlexServer) => server.owned && server.provides === "server"
            );
            if (!serverList.length) {
                displayNotification({
                    title: "No Servers Found",
                    message: "No Plex servers found for your account.",
                    type: "error",
                });
                setServers(null);
                return;
            }
            setServers({Servers: serverList});
        } catch (err) {
            const error = err as Error;
            displayNotification({
                title: "Failed to Load Servers",
                message: error.message || "Could not load Plex servers.",
                type: "error",
            });
            setServers(null);
        } finally {
            setLoadingServers(false);
        }
    };

    const handleServerSelection = (value: string | null) => {
        if (!value) {
            setSelectedServer(null);
            return;
        }
        const server = servers?.Servers.find((s) => s.clientIdentifier === value);
        setSelectedServer(server || null);
    };


    const loadLibrariesForServer = async () => {
        const values = form.getValues();
        if (!values.host || !values.port) {
            displayNotification({
                title: "Missing Information",
                message: "Please enter host and port before loading libraries.",
                type: "error",
            });
            return;
        }

        if (!values.client_id) {
            displayNotification({
                title: "Not Authenticated",
                message: "Please authenticate with Plex first.",
                type: "error",
            });
            return;
        }

        try {
            setLoadingLibraries(true);
            const response = await APIClient.plex.libraries(values);
            setLibraries(response.MediaContainer.Directory || []);
            displayNotification({
                title: "Libraries Loaded",
                message: "Successfully loaded libraries from Plex server.",
                type: "success",
            });
        } catch (err) {
            const error = err as Error;
            displayNotification({
                title: "Failed to Load Libraries",
                message: error.message || "Could not load libraries. Please check your host, port, and TLS settings.",
                type: "error",
            });
        } finally {
            setLoadingLibraries(false);
        }
    };


    // Prepare options for the MultiSelect (deduplicated)
    const libraryOptions = Array.from(
        new Set(libraries.map((lib) => lib.title))
    ).map((title) => ({value: title, label: title}));

    const testPlex = async () => {
        try {
            setTestingConnection(true)
            await APIClient.plex.test(form.getValues());
            displayNotification({
                title: "Test Successful",
                message: "Plex connection test was successful",
                type: "success",
            });
        } catch (err) {
            const error = err as Error;
            displayNotification({
                title: "Test Failed",
                message: error.message || "Plex connection test failed.",
                type: "error",
            });
        } finally {
            setTestingConnection(false);
        }
    }

    const mutation = useMutation({
        mutationFn: APIClient.plex.updateSettings,
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: PlexSettingsKeys.config()});
            onClose();
            displayNotification({
                title: "Success",
                message: "Plex settings updated successfully",
                type: "success",
            });
        },
        onError: (error) => {
            displayNotification({
                title: "Update failed",
                message: error.message || "Could not update Plex settings",
                type: "error",
            });
        },
    });

    const handleFormSubmit = (values: PlexConfig) => {
        mutation.mutate(values);
    };

    const portInput = form.getInputProps("port", {type: "input"});
    const hostInput = form.getInputProps("host", {type: "input"});
    
    // Check if we can load libraries (authenticated and host/port filled)
    const canLoadLibraries = authenticated && form.getValues().host && form.getValues().port && form.getValues().client_id;
    return (
        <Modal opened={opened} onClose={onClose} title="Plex Settings">
            <form onSubmit={form.onSubmit(handleFormSubmit)}>
                <Stack align="stretch">
                    <Button loading={polling} onClick={handlePlexLogin} variant="light">
                        Authenticate with Plex
                    </Button>
                    <Group align="flex-end" justify="flex-start">
                        <Select style={{flex: 1}}
                                label="Plex Server"
                                placeholder="Load servers after authenticating"
                                data={servers?.Servers.map((server) => ({
                                    value: server.clientIdentifier,
                                    label: server.name,
                                }))}
                                value={selectedServer ? selectedServer.clientIdentifier : ""}
                                onChange={handleServerSelection}
                                disabled={!servers || polling || loadingServers}
                                searchable
                                clearable
                        />
                        <Button onClick={loadServers} disabled={loadingServers || polling} loading={loadingServers}>
                            <TfiReload/>
                        </Button>
                    </Group>
                    <TextInput
                        label="Host"
                        placeholder="Enter Plex server host (e.g., 192.168.1.100 or plex.example.com)"
                        {...hostInput}
                        disabled={loadingServers}
                    />
                    <TextInput
                        label="Port"
                        placeholder="Enter Plex server port (e.g., 32400)"
                        type="number"
                        value={form.getValues().port || ""}
                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                            const value = e.target.value;
                            form.setFieldValue("port", value ? parseInt(value, 10) : 0);
                        }}
                        onBlur={portInput.onBlur}
                        disabled={loadingServers}
                        error={portInput.error}
                    />
                    <Group align="flex-end" justify="flex-start">
                        <Switch
                            label="TLS (HTTPS)"
                            {...form.getInputProps("tls", {type: "checkbox"})}
                        />
                        <Switch
                            label="Skip TLS Verification"
                            {...form.getInputProps("tls_skip", {type: "checkbox"})}
                        />
                    </Group>
                    <Group align="flex-end">
                        <MultiSelect style={{flex: 1}}
                                     label="Anime Libraries"
                                     placeholder="Pick Anime Libraries"
                                     data={libraryOptions}
                                     value={form.getValues().anime_libs}
                                     disabled={loadingLibraries || libraryOptions.length === 0}
                                     searchable
                                     clearable
                                     hidePickedOptions
                                     {...form.getInputProps("anime_libs", {type: "input"})}
                        />
                        <Button onClick={loadLibrariesForServer} loading={loadingLibraries}
                                disabled={loadingLibraries || polling || loadingServers || !canLoadLibraries}>
                            <TfiReload/>
                        </Button>
                    </Group>
                    <TextInput
                        label="Username"
                        placeholder="Enter Plex username"
                        {...form.getInputProps("plex_user")}
                    />
                    <TextInput
                        label="Client ID"
                        placeholder="To generate client ID, authenticate with Plex"
                        disabled
                        {...form.getInputProps("client_id")}
                    />
                    <Switch
                        label="Enable Plex Client"
                        {...form.getInputProps("plex_client_enabled", {type: "checkbox"})}
                    />

                    <Group mt="sm" justify="flex-end">
                        <Button variant="default" onClick={onClose} disabled={polling}>
                            CANCEL
                        </Button>
                        <Button onClick={testPlex} loading={testingConnection}>
                            TEST
                        </Button>
                        <Button type="submit" disabled={polling}>
                            SAVE
                        </Button>
                    </Group>
                </Stack>
            </form>
        </Modal>
    );
};