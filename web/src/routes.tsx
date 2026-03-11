import {
    createRootRouteWithContext,
    createRoute,
    createRouter,
    Navigate,
    Outlet,
    redirect,
} from "@tanstack/react-router";
import {z} from "zod";
import {QueryClient} from "@tanstack/react-query";
import {APIClient} from "@api/APIClient";
import {Login, Onboarding} from "@screens/auth";
import {Layout} from "@components/layout";
import {NotFound} from "@components/alerts/NotFound";
import {Dashboard} from "@screens/Dashboard";
import {Logs} from "@screens/Logs";
import {Settings} from "@screens/Settings";
import {PlexPayloads} from "@screens/PlexPayloads";
import {AnimeUpdates} from "@screens/AnimeUpdates";
import {Application} from "@screens/settings/Application";
import {User} from "@screens/settings/User";
import {Api} from "@screens/settings/Api";
import {Notifications} from "@screens/settings/Notifications";
import {Logs as SettingsLogs} from "@screens/settings/Logs";
import {Plex} from "@screens/settings/Plex";
import {Mal} from "@screens/settings/Mal";
import {Anilist} from "@screens/settings/Anilist";
import {MapSettings} from "@screens/settings/Mapping";
import {AuthContext, SettingsContext} from "@utils/Context";
import {TanStackRouterDevtools} from "@tanstack/react-router-devtools";
import {ReactQueryDevtools} from "@tanstack/react-query-devtools";
import {queryClient} from "@api/QueryClient";
import {MalAuthCallback} from "@screens/MalAuthCallback.tsx";
import {AnilistAuthCallback} from "@screens/AnilistAuthCallback.tsx";
import {Loader, Center} from "@mantine/core";
import {
    plexCountsQueryOptions,
    animeUpdateCountQueryOptions,
    recentAnimeUpdatesQueryOptions,
    plexHistoryQueryOptions,
    ConfigQueryOptions,
    PlexSettingsQueryOptions,
    MalQueryOptions,
    AnilistQueryOptions,
    MappingQueryOptions,
    NotificationsQueryOptions,
    ApikeysQueryOptions,
    LogQueryOptions,
} from "@api/queries";

const DashboardRoute = createRoute({
    getParentRoute: () => AuthIndexRoute,
    path: "/",
    loader: async ({ context }) => {
        // Prefetch dashboard data for smoother loading
        const settings = SettingsContext.get();
        const limit = settings.timelineLimit || 5;
        
        await Promise.all([
            context.queryClient.ensureQueryData(plexCountsQueryOptions()),
            context.queryClient.ensureQueryData(animeUpdateCountQueryOptions()),
            context.queryClient.ensureQueryData(recentAnimeUpdatesQueryOptions(16)),
            context.queryClient.ensureQueryData(plexHistoryQueryOptions({ limit })),
        ]);
        
        return {};
    },
    component: Dashboard,
});

const LogsRoute = createRoute({
    getParentRoute: () => AuthIndexRoute,
    path: "logs",
    pendingMs: 3000,
    component: Logs,
});

export const PlexPayloadsRoute = createRoute({
    getParentRoute: () => AuthIndexRoute,
    path: "plex-payloads",
    pendingMs: 3000,
    validateSearch: z.object({
        highlight: z.coerce.string().optional(),
    }),
    component: PlexPayloads,
});

const AnimeUpdatesRoute = createRoute({
    getParentRoute: () => AuthIndexRoute,
    path: "anime-updates",
    pendingMs: 3000,
    component: AnimeUpdates,
});

export const OnboardRoute = createRoute({
    getParentRoute: () => RootRoute,
    path: "onboard",
    beforeLoad: async () => {
        // Check if onboarding is available for this instance
        // and redirect if needed
        try {
            await APIClient.auth.canOnboard();
        } catch {
            console.error("onboarding not available, redirect to login");

            throw redirect({
                to: LoginRoute.to,
            });
        }
    },
    component: Onboarding,
});

export const LoginRoute = createRoute({
    getParentRoute: () => RootRoute,
    path: "login",
    validateSearch: z.object({
        redirect: z.string().optional(),
    }),
    beforeLoad: ({navigate}) => {
        // handle canOnboard
        APIClient.auth
            .canOnboard()
            .then(() => {
                console.info("onboarding available, redirecting");

                navigate({to: OnboardRoute.to});
            })
            .catch(() => {
                console.info("onboarding not available, please login");
            });
    },
}).update({component: Login});

export const AuthRoute = createRoute({
    getParentRoute: () => RootRoute,
    id: "auth",
    // Validate session before loading protected routes to avoid firing many API calls
    beforeLoad: async ({context, location}) => {
        if (!AuthContext.get().isLoggedIn) {
            throw redirect({
                to: LoginRoute.to,
                search: {
                    redirect: location.href,
                },
            });
        }
        // Validate cookie/session; if invalid, reset and redirect before children mount
        try {
            await APIClient.auth.validate();
        } catch {
            AuthContext.reset();
            throw redirect({
                to: LoginRoute.to,
                search: {
                    redirect: location.href,
                },
            });
        }
        return context;
    },
});

function AuthenticatedLayout() {
    const isLoggedIn = AuthContext.useSelector((s) => s.isLoggedIn);
    if (!isLoggedIn) {
        const redirectParam =
            location.pathname.length > 1
                ? {redirect: location.pathname}
                : undefined;
        return <Navigate to="/login" search={redirectParam}/>;
    }

    return (
        <div>
            <Layout/>
        </div>
    );
}

export const MalAuthRoute = createRoute({
    getParentRoute: () => RootRoute,
    component: MalAuthCallback,
    path: "malauth/callback",
    pendingMs: 3000,
});

export const AnilistAuthCallbackRoute = createRoute({
    getParentRoute: () => RootRoute,
    component: AnilistAuthCallback,
    path: "anilistauth/callback",
    pendingMs: 3000,
});

export const SettingsRoute = createRoute({
    getParentRoute: () => AuthIndexRoute,
    path: "settings",
    pendingMs: 3000,
    component: Settings,
});

export const SettingsApplicationRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "/",
    loader: (opts) => opts.context.queryClient.ensureQueryData(ConfigQueryOptions()),
    pendingMs: 3000,
    component: Application,
});

export const SettingsUserRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "user",
    pendingMs: 3000,
    component: User,
});

export const SettingsApiRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "api",
    loader: (opts) => opts.context.queryClient.ensureQueryData(ApikeysQueryOptions()),
    pendingMs: 3000,
    component: Api,
});

export const SettingsNotificationsRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "notifications",
    loader: (opts) => opts.context.queryClient.ensureQueryData(NotificationsQueryOptions()),
    pendingMs: 3000,
    component: Notifications,
});

export const SettingsLogsRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "logs",
    loader: async (opts) => {
        await Promise.all([
            opts.context.queryClient.ensureQueryData(ConfigQueryOptions()),
            opts.context.queryClient.ensureQueryData(LogQueryOptions()),
        ]);
    },
    pendingMs: 3000,
    component: SettingsLogs,
});

export const SettingsPlexRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "plex",
    loader: (opts) => opts.context.queryClient.ensureQueryData(PlexSettingsQueryOptions()),
    pendingMs: 3000,
    component: Plex,
});

export const SettingsMalRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "mal",
    loader: (opts) => opts.context.queryClient.ensureQueryData(MalQueryOptions()),
    pendingMs: 3000,
    component: Mal,
});

export const SettingsAnilistRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "anilist",
    loader: (opts) => opts.context.queryClient.ensureQueryData(AnilistQueryOptions()),
    pendingMs: 3000,
    component: Anilist,
});

export const SettingsMappingRoute = createRoute({
    getParentRoute: () => SettingsRoute,
    path: "mapping",
    loader: (opts) => opts.context.queryClient.ensureQueryData(MappingQueryOptions()),
    pendingMs: 3000,
    component: MapSettings,
});

export const AuthIndexRoute = createRoute({
    getParentRoute: () => AuthRoute,
    component: AuthenticatedLayout,
    id: "authenticated-routes",
});

// Root component that renders the outlet for child routes
export const RootComponent = () => {
    const settings = SettingsContext.useValue();
    return (
        <div className="full-height">
            <Outlet/>
            {settings.debug ? (
                <>
                    <TanStackRouterDevtools/>
                    <ReactQueryDevtools initialIsOpen={false}/>
                </>
            ) : null}
        </div>
    );
};

export const RootRoute = createRootRouteWithContext<{
    queryClient: QueryClient;
}>()({
    component: RootComponent,
    notFoundComponent: NotFound,
});

const settingsRouteTree = SettingsRoute.addChildren([
    SettingsApplicationRoute,
    SettingsUserRoute,
    SettingsApiRoute,
    SettingsNotificationsRoute,
    SettingsLogsRoute,
    SettingsPlexRoute,
    SettingsMalRoute,
    SettingsAnilistRoute,
    SettingsMappingRoute,
]);
const authenticatedTree = AuthRoute.addChildren([
    AuthIndexRoute.addChildren([DashboardRoute, LogsRoute, PlexPayloadsRoute, AnimeUpdatesRoute, settingsRouteTree]),
]);
const routeTree = RootRoute.addChildren([
    LoginRoute,
    OnboardRoute,
    MalAuthRoute,
    AnilistAuthCallbackRoute,
    authenticatedTree,
]);

export const Router = createRouter({
    routeTree,
    defaultPendingComponent: () => (
        <Center style={{ height: "400px" }}>
            <Loader size="lg" />
        </Center>
    ),
    context: {
        queryClient,
    },
});
