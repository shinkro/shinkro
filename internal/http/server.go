package http

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"github.com/shinkro/shinkro/internal/config"
	"github.com/shinkro/shinkro/internal/database"
	"github.com/shinkro/shinkro/pkg/sse"
	"github.com/shinkro/shinkro/web"
)

type Server struct {
	log                 zerolog.Logger
	db                  *database.DB
	config              *config.AppConfig
	cookieStore         *sessions.CookieStore
	version             string
	commit              string
	date                string
	plexService         plexService
	plexsettingsService plexsettingsService
	malauthService      malauthService
	anilistAuthService  anilistAuthService
	apiService          apikeyService
	authService         authService
	mappingService      mappingService
	fsService           filesystemService
	notificationService notificationService
	animeUpdateService  animeupdateService
	sse                 *sse.Server
}

func NewServer(log zerolog.Logger, config *config.AppConfig, db *database.DB, version string, commit string, date string, plexSvc plexService, plexsettingsSvc plexsettingsService, malauthSvc malauthService, anilistAuthSvc anilistAuthService, apiSvc apikeyService, authSvc authService, mappingSvc mappingService, fsSvc filesystemService, notificationSvc notificationService, animeUpdateSvc animeupdateService, sseServer *sse.Server) Server {
	return Server{
		log:                 log.With().Str("module", "http").Logger(),
		config:              config,
		db:                  db,
		version:             version,
		commit:              commit,
		date:                date,
		cookieStore:         sessions.NewCookieStore([]byte(config.Config.SessionSecret)),
		plexService:         plexSvc,
		plexsettingsService: plexsettingsSvc,
		malauthService:      malauthSvc,
		anilistAuthService:  anilistAuthSvc,
		apiService:          apiSvc,
		authService:         authSvc,
		mappingService:      mappingSvc,
		fsService:           fsSvc,
		notificationService: notificationSvc,
		animeUpdateService:  animeUpdateSvc,
		sse:                 sseServer,
	}
}

func (s Server) Open() error {
	addr := fmt.Sprintf("%v:%v", s.config.Config.Host, s.config.Config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: time.Second * 15,
	}

	s.log.Info().Msgf("Starting server. Listening on %s", listener.Addr().String())

	return server.Serve(listener)
}

func (s Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(hlog.NewHandler(s.log))

	baseUrl, webBase := normalizeBaseUrl(s.config.Config.BaseUrl)
	c := cors.New(cors.Options{
		AllowCredentials:   true,
		AllowedMethods:     []string{"HEAD", "OPTIONS", "GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowOriginFunc:    func(origin string) bool { return true },
		OptionsPassthrough: true,
		// Enable Debugging for testing, consider disabling in production
		Debug: false,
	})

	r.Use(c.Handler)

	encoder := encoder{}

	apiRouter := chi.NewRouter()
	apiRouter.Use(hlog.NewHandler(s.log))

	// Public health check endpoint (no authentication required)
	apiRouter.Route("/healthz", func(r chi.Router) {
		r.Get("/liveness", LivenessHandler)
	})

	apiRouter.Route("/auth", newAuthHandler(encoder, s.log, s, s.config.Config, s.cookieStore, s.authService).Routes)

	apiRouter.Group(func(r chi.Router) {
		r.Use(s.IsAuthenticated)
		r.Route("/config", newConfigHandler(encoder, s, s.config).Routes)
		r.Route("/plex", newPlexHandler(encoder, s.plexService).Routes)
		r.Route("/plex/settings", newPlexsettingsHandler(encoder, s.plexsettingsService).Routes)
		r.Route("/malauth", newmalauthHandler(encoder, s.malauthService, s.cookieStore).Routes)
		r.Route("/anilistauth", newAnilistAuthHandler(encoder, s.anilistAuthService, s.cookieStore, s.config.Config.Port).Routes)
		r.Route("/keys", newAPIKeyHandler(encoder, s.apiService).Routes)
		r.Route("/mapping", newMappingHandler(encoder, s.mappingService).Routes)
		r.Route("/fs", newFilesystemHandler(encoder, s.fsService).Routes)
		r.Route("/notification", newNotificationHandler(encoder, s.notificationService).Routes)
		r.Route("/animeupdate", newAnimeupdateHandler(encoder, s.animeUpdateService).Routes)
		r.Get("/updates/latest", GetLatestReleaseHandler)

		// SSE events endpoint
		r.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
			// inject CORS headers to bypass checks
			s.sse.Headers = map[string]string{
				"Content-Type":      "text/event-stream",
				"Cache-Control":     "no-cache",
				"Connection":        "keep-alive",
				"X-Accel-Buffering": "no",
			}
			s.sse.ServeHTTP(w, r)
		})
	})

	// Mount API routes under baseUrl + "api"
	r.Mount(baseUrl+"api", apiRouter)

	// Create a separate web router for the SPA and static files
	webRouter := chi.NewRouter()
	web.RegisterHandler(webRouter, s.version, baseUrl)

	// Mount the web router under the baseUrl
	r.Mount(webBase, webRouter)

	return r
}
