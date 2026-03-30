import { Container, Paper, Tabs , Stack, Title} from "@mantine/core";
import { Outlet, useRouterState, useNavigate } from "@tanstack/react-router";
import { FaCog, FaUserCog, FaKey, FaBell, FaMap } from "react-icons/fa";
import { BsStack } from "react-icons/bs";
import { baseUrl, normalizePathname } from "@utils";

export const Settings = () => {
    const fullPathname = useRouterState().location.pathname;
    const navigate = useNavigate();
    
    const normalizedPathname = normalizePathname(fullPathname, baseUrl());
    
    // Determine active tab based on normalized path
    const getActiveTab = () => {
        if (normalizedPathname === '/settings') return 'application';
        if (normalizedPathname === '/settings/user') return 'user';
        if (normalizedPathname === '/settings/api') return 'api';
        if (normalizedPathname === '/settings/mapping') return 'mapping';
        if (normalizedPathname === '/settings/notifications') return 'notifications';
        if (normalizedPathname === '/settings/logs') return 'logs';
        return 'application';
    };

    const handleTabChange = (value: string | null) => {
        if (!value) return;
        
        const routeMap: Record<string, string> = {
            'application': '/settings',
            'user': '/settings/user',
            'api': '/settings/api',
            'mapping': '/settings/mapping',
            'notifications': '/settings/notifications',
            'logs': '/settings/logs',
        };
        
        const route = routeMap[value];
        if (route) {
            navigate({ to: route });
        }
    };

    return (
        <Container size={1200} px="md" component="main">
            <Stack gap="md" p="md">
                <Title order={2}>Settings</Title>
            <Paper mt="md" withBorder p={"md"} h={"100%"} mih={"80vh"}>
                {normalizedPathname === '/settings' || (normalizedPathname.startsWith('/settings/') && !normalizedPathname.includes('/plex') && !normalizedPathname.includes('/mal') && !normalizedPathname.includes('/anilist')) ? (
                    <Tabs value={getActiveTab()} onChange={handleTabChange} variant="default">
                        <Tabs.List justify="space-between">
                            <Tabs.Tab value="application" leftSection={<FaCog size={14} />}>
                                Application
                            </Tabs.Tab>
                            <Tabs.Tab value="user" leftSection={<FaUserCog size={14} />}>
                                User
                            </Tabs.Tab>
                            <Tabs.Tab value="api" leftSection={<FaKey size={14} />}>
                                API Keys
                            </Tabs.Tab>
                            <Tabs.Tab value="mapping" leftSection={<FaMap size={14} />}>
                                Mapping
                            </Tabs.Tab>
                            <Tabs.Tab value="notifications" leftSection={<FaBell size={14} />}>
                                Notifications
                            </Tabs.Tab>
                            <Tabs.Tab value="logs" leftSection={<BsStack size={14} />}>
                                Logs
                            </Tabs.Tab>
                        </Tabs.List>
                    </Tabs>
                ) : null}
                <Outlet />
            </Paper>
        </Stack>
        </Container>
    );
};
