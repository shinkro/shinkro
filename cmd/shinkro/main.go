package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/asaskevich/EventBus"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/pflag"

	"github.com/shinkro/shinkro/internal/anime"
	"github.com/shinkro/shinkro/internal/animeupdate"
	"github.com/shinkro/shinkro/internal/api"
	"github.com/shinkro/shinkro/internal/auth"
	"github.com/shinkro/shinkro/internal/config"
	"github.com/shinkro/shinkro/internal/database"
	"github.com/shinkro/shinkro/internal/domain"
	"github.com/shinkro/shinkro/internal/events"
	"github.com/shinkro/shinkro/internal/filesystem"
	"github.com/shinkro/shinkro/internal/http"
	"github.com/shinkro/shinkro/internal/logger"
	"github.com/shinkro/shinkro/internal/malauth"
	"github.com/shinkro/shinkro/internal/mapping"
	"github.com/shinkro/shinkro/internal/notification"
	"github.com/shinkro/shinkro/internal/plex"
	"github.com/shinkro/shinkro/internal/plexsettings"
	"github.com/shinkro/shinkro/internal/server"
	"github.com/shinkro/shinkro/internal/user"
	"github.com/shinkro/shinkro/pkg/sse"
)

const usage = `shinkro
Sync your Anime watch status in Plex to myanimelist.net!
Usage:
    shinkro --config <path to shinkro configuration directory>          Run shinkro
    shinkro setup --config <dir> --username <user> --password <pass>   Setup new config, DB, and admin user
    shinkro --config=<dir> change-password <username> --password <pass> Change password for user
    shinkro version                                                     Print version info
    shinkro help                                                        Show this help message
`

func init() {
	pflag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
	}
}

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	var configPath string
	var setupUsername string
	var setupPassword string

	d, err := homedir.Dir()
	if err != nil {
		fmt.Fprint(flag.CommandLine.Output(), "FATAL: Unable to get home directory")
		os.Exit(1)
	}

	d = filepath.Join(d, ".config", "shinkro")
	pflag.StringVar(&configPath, "config", d, "path to configuration")
	pflag.StringVar(&setupUsername, "username", "", "username for non-interactive setup")
	pflag.StringVar(&setupPassword, "password", "", "password for non-interactive setup")
	pflag.Parse()
	configPath, err = homedir.Expand(configPath)
	if err != nil {
		fmt.Fprint(flag.CommandLine.Output(), "FATAL: Unable to expand configuration path")
		os.Exit(1)
	}

	switch cmd := pflag.Arg(0); cmd {
	case "":
		cfg := config.NewConfig(configPath, version)
		log := logger.NewLogger(cfg.Config).Logger
		go cfg.DynamicReload(log)
		db := database.NewDB(configPath, &log)

		log.Info().Msg("Starting shinkro")
		log.Info().Msgf("Version: %s", version)
		log.Info().Msgf("Commit: %s", commit)
		log.Info().Msgf("Build date: %s", date)
		log.Info().Msgf("Base URL: %s", cfg.Config.BaseUrl)
		log.Info().Msgf("Log-level: %s", cfg.Config.LogLevel)

		err = db.Migrate()
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}

		// Setup server-sent-events with buffering and replay
		serverEvents := sse.New()
		serverEvents.CreateStreamWithOpts("logs", sse.StreamOpts{
			MaxEntries: 1000,
			AutoReplay: true,
		})

		// Register SSE hook on logger
		logger.GetInstance().RegisterSSEWriter(serverEvents)

		// Setup internal event bus
		bus := EventBus.New()

		// Initialize repositories
		var (
			animeRepo        = database.NewAnimeRepo(log, db)
			animeUpdateRepo  = database.NewAnimeUpdateRepo(log, db)
			plexRepo         = database.NewPlexRepo(log, db)
			plexSettingsRepo = database.NewPlexSettingsRepo(log, db)
			malauthRepo      = database.NewMalAuthRepo(log, db)
			userRepo         = database.NewUserRepo(log, db)
			apiRepo          = database.NewAPIRepo(log, db)
			mappingRepo      = database.NewMappingRepo(log, db)
			notificationRepo = database.NewNotificationRepo(log, db)
		)

		// Initialize services
		var (
			animeService        = anime.NewService(log, animeRepo)
			malauthService      = malauth.NewService(cfg.Config, log, malauthRepo)
			mapService          = mapping.NewService(log, mappingRepo)
			plexSettingsService = plexsettings.NewService(cfg.Config, log, plexSettingsRepo)
			notificationService = notification.NewService(log, notificationRepo)
			animeUpdateService  = animeupdate.NewService(log, animeUpdateRepo, animeService, mapService, malauthService, bus)
			plexService         = plex.NewService(log, plexSettingsService, plexRepo, animeService, mapService, malauthService, animeUpdateService, bus)
			userService         = user.NewService(userRepo, log)
			authService         = auth.NewService(log, userService)
			apiService          = api.NewService(log, apiRepo)
			fsService           = filesystem.NewService(cfg.Config, log)
		)

		// Register event subscribers
		events.NewSubscribers(log, bus, notificationService, plexService, animeUpdateService)

		srv := server.NewServer(log, cfg.Config, animeService, mapService, bus)
		if err := srv.Start(); err != nil {
			log.Fatal().Stack().Err(err).Msg("could not start server")
			return
		}

		errorChannel := make(chan error)

		go func() {
			httpServer := http.NewServer(
				log,
				cfg,
				db,
				version,
				commit,
				date,
				plexService,
				plexSettingsService,
				malauthService,
				apiService,
				authService,
				mapService,
				fsService,
				notificationService,
				animeUpdateService,
				serverEvents,
			)
			errorChannel <- httpServer.Open()
		}()

		sigchnl := make(chan os.Signal, 1)
		signal.Notify(sigchnl, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		for sig := range sigchnl {
			log.Info().Msgf("received signal: %v, shutting down server.", sig)
			if err := db.Close(); err != nil {
				log.Error().Err(err).Msg("failed to close the database connection properly")
				os.Exit(1)
			}
			os.Exit(0)
		}

	case "setup":
		if setupUsername == "" || setupPassword == "" {
			fmt.Fprintln(os.Stderr, "Usage: shinkro setup --config <dir> --username <user> --password <pass>")
			os.Exit(1)
		}

		username := strings.TrimSpace(setupUsername)
		password := strings.TrimSpace(setupPassword)

		if username == "" || password == "" {
			fmt.Fprintln(os.Stderr, "Username and password cannot be empty")
			os.Exit(1)
		}

		cfg := config.NewConfig(configPath, version)
		log := logger.NewLogger(cfg.Config).Logger
		if err := cfg.WriteConfig(configPath, "config.toml"); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
			os.Exit(1)
		}

		db := database.NewDB(configPath, &log)
		if err := db.Migrate(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to migrate DB: %v\n", err)
			os.Exit(1)
		}

		userRepo := database.NewUserRepo(log, db)
		userService := user.NewService(userRepo, log)
		authService := auth.NewService(log, userService)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := authService.CreateUser(ctx, domain.CreateUserRequest{Username: username, Password: password}); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create user: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Setup complete! Config, DB, and admin user created.")
		os.Exit(0)

	case "change-password":
		if pflag.NArg() < 2 || setupPassword == "" {
			fmt.Fprintln(os.Stderr, "Usage: shinkro --config=<dir> change-password <username> --password <new_password>")
			os.Exit(1)
		}

		username := pflag.Arg(1)
		newPassword := strings.TrimSpace(setupPassword)

		if newPassword == "" {
			fmt.Fprintln(os.Stderr, "Password cannot be empty")
			os.Exit(1)
		}

		cfg := config.NewConfig(configPath, version)
		log := logger.NewLogger(cfg.Config).Logger
		db := database.NewDB(configPath, &log)
		userRepo := database.NewUserRepo(log, db)
		userService := user.NewService(userRepo, log)
		authService := auth.NewService(log, userService)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := authService.ResetPassword(ctx, username, newPassword); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to change password: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Password updated successfully.")
		os.Exit(0)

	case "version":
		fmt.Fprintln(flag.CommandLine.Output(), "Version:", version)
		fmt.Fprintln(flag.CommandLine.Output(), "Commit:", commit)
		fmt.Fprintln(flag.CommandLine.Output(), "Build Date:", date)

	default:
		pflag.Usage()
		if cmd != "help" {
			os.Exit(0)
		}
	}
}
