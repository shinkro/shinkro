import {ActionIcon, AppShell, Code, Flex, Group, Image, Menu, NavLink, rem, Title, Burger, Badge} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import Logo from "@app/logo.svg";
import {displayNotification} from "@components/notifications";
import {MdDarkMode, MdLightMode} from "react-icons/md";
import {FaDiscord, FaGithub, FaUser} from "react-icons/fa";
import {GrHelpBook} from "react-icons/gr";
import {BiLogOut} from "react-icons/bi";
import {SiMyanimelist, SiPlex, SiAnilist} from "react-icons/si";
import {FaSlidersH} from "react-icons/fa";
import {useMutation, useQuery} from "@tanstack/react-query";
import {Link, Outlet, useNavigate, useRouterState} from "@tanstack/react-router";
import {APIClient} from "@api/APIClient";
import {ConfigQueryOptions, latestReleaseQueryOptions} from "@api/queries";
import {AuthContext, useThemeToggle} from "@utils/Context";
import {ExternalLink} from "@components/ExternalLink";
import {NAV_ROUTES} from "./navigation";
import classes from "./Layout.module.css";
import { Text } from "@mantine/core";
import { baseUrl, normalizePathname } from "@utils";

export const Layout = () => {
    const [opened, {toggle}] = useDisclosure();
    const navigate = useNavigate();
    const fullPathname = useRouterState().location.pathname;
    const pathname = normalizePathname(fullPathname, baseUrl());

    const {isError: isConfigError, error: configError, data: config} = useQuery(ConfigQueryOptions(true));
    const { data: latestRelease } = useQuery(latestReleaseQueryOptions());
    if (isConfigError) {
        console.log(configError);
    }

    const {colorScheme, toggleTheme} = useThemeToggle();

    const [settingsOpened, { close: closeSettings, toggle: toggleSettings }] = useDisclosure(false);

    const handleNavLinkClick = (itemName?: string) => {
        if (window.innerWidth < 768 && opened) {
            toggle();
        }
        // Close settings if Dashboard or Logs is clicked
        if (itemName && (itemName === 'Dashboard' || itemName === 'Logs')) {
            closeSettings();
        }
    };

    const handleSettingsClick = () => {
        if (window.innerWidth < 768 && opened) {
            toggle();
        }
        // Navigate to General settings when Settings is clicked
        navigate({ to: '/settings' });
    };

    const logoutMutation = useMutation({
        mutationFn: APIClient.auth.logout,
        onSuccess: () => {
            displayNotification({
                title: "Logged out",
                message: "You have been logged out. Goodbye!",
                type: "success",
            });
            AuthContext.reset();
            void navigate({to: "/login"})
        },
        onError: (err) => {
            console.error("logout error", err);
        },
    });

    // Helper to compare versions (assumes semver, ignores pre-release/build)
    function isUpdateAvailable(current: string, latest: string) {
        const cur = current.replace(/^v/, '').split('.').map(Number);
        const lat = latest.replace(/^v/, '').split('.').map(Number);
        for (let i = 0; i < Math.max(cur.length, lat.length); i++) {
            const a = cur[i] || 0, b = lat[i] || 0;
            if (a < b) return true;
            if (a > b) return false;
        }
        return false;
    }

    const isDevOrNightly = config?.version && /dev|nightly/i.test(config.version);


    const settingsData = [
        { label: 'General', icon: FaSlidersH, link: '/settings' },
        { label: 'Plex', icon: SiPlex, link: '/settings/plex' },
        { label: 'MyAnimeList', icon: SiMyanimelist, link: '/settings/mal' },
        { label: 'AniList', icon: SiAnilist, link: '/settings/anilist' },
    ];


    return (
        <AppShell
            header={{height: 60}}
            navbar={{width: 300, breakpoint: 'sm', collapsed: {mobile: !opened}}}
            className={classes.appshell}
        >
            <AppShell.Header className={classes.header}>
                <Group h="100%" px={"md"} align="center" gap={0}>
                    <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" mr={"xs"}/>
                    <Link to="/" style={{textDecoration: "none", color: "inherit", display: "flex", alignItems: "center"}}>
                        <Image src={Logo} style={{width: 60, height: 60, flexShrink: 0}}/>
                    </Link>
                    <Flex align="flex-end" gap={"xs"} ml={"xs"}>
                        <Link to="/" style={{textDecoration: "none", color: "inherit"}}>
                            <Title order={3}>shinkro</Title>
                        </Link>
                        <Code fw={700} className={classes.code}>
                            {config?.version}
                        </Code>
                    </Flex>
                    <Flex align="center" mt="xs">
                        {config?.check_for_updates && latestRelease?.tag_name && config?.version && !isDevOrNightly && isUpdateAvailable(config.version, latestRelease.tag_name) && (
                            <Badge color="blue" size="xs" ml={4} component="a" href={`https://github.com/shinkro/shinkro/releases/tag/${latestRelease.tag_name}`} target="_blank" style={{ verticalAlign: 'middle', cursor: 'pointer' }}>
                                Update Available!
                            </Badge>
                        )}
                    </Flex>
                    <Menu
                        shadow="md"
                        width={200}
                        position="bottom-start"
                        offset={8}
                        withArrow
                        arrowPosition="center"
                        transitionProps={{transition: "skew-up", duration: 150}}
                    >
                        <Menu.Target>
                            <ActionIcon
                                variant="outline"
                                radius="md"
                                size="lg"
                                ml={"auto"}
                            >
                                <FaUser style={{width: "80%", height: "80%"}}/>
                            </ActionIcon>
                        </Menu.Target>

                        <Menu.Dropdown>
                            <Menu.Item
                                onClick={() => logoutMutation.mutate()}
                                leftSection={
                                    <BiLogOut style={{width: rem(20), height: rem(20)}}/>
                                }
                            >
                                Logout
                            </Menu.Item>
                        </Menu.Dropdown>
                    </Menu>

                    <ExternalLink href="https://discord.gg/ZkYdfNgbAT">
                        <ActionIcon variant="filled" color="#7289da" radius="md" size="lg" visibleFrom={"sm"} ml={"xs"}>
                            <FaDiscord style={{width: "80%", height: "80%"}}/>
                        </ActionIcon>
                    </ExternalLink>

                    <ExternalLink href="https://github.com/shinkro/shinkro">
                        <ActionIcon variant="default" radius="md" size="lg" visibleFrom={"sm"} ml={"xs"}>
                            <FaGithub style={{width: "80%", height: "80%"}}/>
                        </ActionIcon>
                    </ExternalLink>

                    <ExternalLink href="https://docs.shinkro.com">
                        <ActionIcon variant="default" radius="md" size="lg" visibleFrom={"sm"} ml={"xs"}>
                            <GrHelpBook style={{width: "80%", height: "80%"}}/>
                        </ActionIcon>
                    </ExternalLink>

                    <ActionIcon
                        variant="outline"
                        radius="xl"
                        size="lg"
                        onClick={toggleTheme}
                        aria-label="Theme Switch"
                        ml={"xs"}
                    >
                        {colorScheme === "dark" ? (
                            <MdLightMode style={{width: "80%", height: "80%"}}/>
                        ) : (
                            <MdDarkMode style={{width: "80%", height: "80%"}}/>
                        )}
                    </ActionIcon>
                </Group>
            </AppShell.Header>

            <AppShell.Navbar className={classes.navbar}>
                {NAV_ROUTES.map((item, itemIdx) => {
                    if (item.name === 'Settings') {
                        return (
                            <NavLink
                                key={item.name + itemIdx}
                                label={
                                    <Group gap={6} align="center">
                                        <item.icon size={20} />
                                        <Text fw={700}>{item.name}</Text>
                                    </Group>
                                }
                                variant="light"
                                color="blue"
                                childrenOffset={28}
                                opened={settingsOpened}
                                onChange={toggleSettings}
                                active={pathname.startsWith('/settings')}
                                onClick={handleSettingsClick}
                            >
                                {settingsData.map((setting) => (
                                    <Link
                                        key={setting.label}
                                        to={setting.link}
                                        style={{textDecoration: "none", color: "inherit"}}
                                        onClick={() => handleNavLinkClick()}
                                    >
                                        {() => (
                                            <NavLink
                                                component="button"
                                                label={
                                                    <Group gap={6} align="center">
                                                        <setting.icon size={14} />
                                                        <Text fw={500} size="sm">{setting.label}</Text>
                                                    </Group>
                                                }
                                                active={setting.label === 'General' ? pathname.startsWith('/settings') && !pathname.startsWith('/settings/plex') && !pathname.startsWith('/settings/mal') && !pathname.startsWith('/settings/anilist') : pathname === setting.link}
                                                variant="light"
                                                color="blue"
                                            />
                                        )}
                                    </Link>
                                ))}
                            </NavLink>
                        );
                    }

                    return (
                        <Link
                            key={item.name + itemIdx}
                            to={item.path}
                            params={{}}
                            style={{textDecoration: "none", color: "inherit"}}
                            onClick={() => handleNavLinkClick(item.name)}
                        >
                            {({isActive}) => {
                                return (
                                    <NavLink
                                        component="button"
                                        label={
                                            <Group gap={6} align="center">
                                                <item.icon size={20} />
                                                <Text fw={700}>{item.name}</Text>
                                            </Group>
                                        }
                                        active={isActive && !settingsOpened}
                                        variant="light"
                                        color="blue"
                                    />
                                );
                            }}
                        </Link>
                    );
                })}
            </AppShell.Navbar>
            <AppShell.Main>
                <div className={classes.variableCenter}>
                    <Outlet/>
                </div>
            </AppShell.Main>
        </AppShell>
    );
};
