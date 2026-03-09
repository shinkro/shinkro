package config

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/shinkro/shinkro/internal/logger"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shinkro/shinkro/internal/api"
	"github.com/shinkro/shinkro/internal/domain"
)

type AppConfig struct {
	Config *domain.Config
	m      *sync.Mutex
}

func NewConfig(dir string, version string) *AppConfig {

	c := &AppConfig{
		m: new(sync.Mutex),
	}

	c.defaultConfig()
	c.Config.ConfigPath = dir
	c.Config.Version = version
	lp, err := filepath.Abs(filepath.Join(c.Config.ConfigPath, "logs/shinkro.log"))
	if err != nil {
		log.Fatal(err)
	}
	c.Config.LogPath = lp

	c.parseConfig(dir)
	c.parseEnv()

	return c
}

func (c *AppConfig) defaultConfig() {
	c.Config = &domain.Config{
		Version:         "dev",
		Host:            "localhost",
		Port:            7011,
		BaseUrl:         "/",
		LogLevel:        "INFO",
		LogMaxSize:      50,
		LogMaxBackups:   3,
		SessionSecret:   api.GenerateSecureToken(16),
		EncryptionKey:   api.GenerateSecureToken(32),
		CheckForUpdates: true,
	}
}

var configTemplate = `###Example config.toml for shinkro
###LogLevel can be set to any one of the following: "INFO", "ERROR", "DEBUG", "TRACE"
###LogxMaxSize is in MB.

Host = "{{ .host }}"

Port = 7011

#BaseUrl = "/shinkro"

SessionSecret = "{{ .sessionSecret }}"

EncryptionKey = "{{ .encryptionKey }}"

LogLevel = "INFO"

#LogPath = "{{ .logPath }}"

#LogMaxSize = 50

#LogMaxBackups = 3

CheckForUpdates = true
`

func (c *AppConfig) WriteConfig(configPath string, configFile string) error {
	cfgPath := filepath.Join(configPath, configFile)

	// check if configPath exists, if not create it
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(configPath, os.ModePerm)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	// check if config exists, if not create it
	if _, err := os.Stat(cfgPath); errors.Is(err, os.ErrNotExist) {
		// set default host
		host := "127.0.0.1"

		if _, err := os.Stat("/.dockerenv"); err == nil {
			// docker creates a .dockerenv file at the root
			// of the directory tree inside the container.
			// if this file exists then the viewer is running
			// from inside a docker container so return true
			host = "0.0.0.0"
		} else if _, err := os.Stat("/dev/.lxc-boot-id"); err == nil {
			// lxc creates this file containing the uuid
			// of the container in every boot.
			// if this file exists then the viewer is running
			// from inside a lxc container so return true
			host = "0.0.0.0"
		} else if os.Getpid() == 1 {
			// if we're running as pid 1, we're honoured.
			// but there's a good chance this is an isolated namespace
			// or a container.
			host = "0.0.0.0"
		} else if user := os.Getenv("USERNAME"); user == "ContainerAdministrator" || user == "ContainerUser" {
			/* this is the correct code below, but golang helpfully Panics when it can't find netapi32.dll
			   the issue was first reported 7 years ago, but is fixed in go 1.24 where the below code works.
			*/
			/*
				 u, err := user.Current(); err == nil && u != nil &&
				(u.Name == "ContainerAdministrator" || u.Name == "ContainerUser") {
				// Windows conatiners run containers as ContainerAdministrator by default */
			host = "0.0.0.0"
		} else if pd, _ := os.Open("/proc/1/cgroup"); pd != nil {
			defer pd.Close()
			b := make([]byte, 4096)
			pd.Read(b)
			if strings.Contains(string(b), "/docker") || strings.Contains(string(b), "/lxc") {
				host = "0.0.0.0"
			}
		}

		f, err := os.Create(cfgPath)
		if err != nil { // perm 0666
			// handle failed create
			log.Printf("error creating file: %q", err)
			return err
		}
		defer f.Close()

		// setup text template to inject variables into
		tmpl, err := template.New("config").Parse(configTemplate)
		if err != nil {
			return errors.Wrap(err, "could not create config template")
		}

		tmplVars := map[string]string{
			"host":          host,
			"sessionSecret": c.Config.SessionSecret,
			"encryptionKey": c.Config.EncryptionKey,
			"logPath":       c.Config.LogPath,
		}

		var buffer bytes.Buffer
		if err = tmpl.Execute(&buffer, &tmplVars); err != nil {
			return errors.Wrap(err, "could not write torrent url template output")
		}

		if _, err = f.WriteString(buffer.String()); err != nil {
			log.Printf("error writing contents to file: %v %q", configPath, err)
			return err
		}

		return f.Sync()
	}

	return nil
}

func (c *AppConfig) parseConfig(dir string) {
	dirClean := path.Clean(dir)

	if dirClean != "" {
		if err := c.WriteConfig(dirClean, "config.toml"); err != nil {
			log.Printf("write error: %q", err)
		}
	}

	dir = filepath.Join(dirClean, "config.toml")

	k := koanf.New(".")
	if err := k.Load(file.Provider(dir), toml.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	err := k.Unmarshal("", c.Config)
	if err != nil {
		log.Fatalf("error unmarshalling config: %v", err)
	}
}

func (c *AppConfig) parseEnv() {
	prefix := "SHINKRO_"

	if v := os.Getenv(prefix + "HOST"); v != "" {
		c.Config.Host = v
	}

	if v := os.Getenv(prefix + "PORT"); v != "" {
		i, _ := strconv.ParseInt(v, 10, 32)
		if i > 0 {
			c.Config.Port = int(i)
		}
	}

	if v := os.Getenv(prefix + "BASE_URL"); v != "" {
		c.Config.BaseUrl = v
	}

	if v := os.Getenv(prefix + "LOG_LEVEL"); v != "" {
		c.Config.LogLevel = v
	}

	if v := os.Getenv(prefix + "LOG_PATH"); v != "" {
		c.Config.LogPath = v
	}

	if v := os.Getenv(prefix + "LOG_MAX_SIZE"); v != "" {
		i, _ := strconv.ParseInt(v, 10, 32)
		if i > 0 {
			c.Config.LogMaxSize = int(i)
		}
	}

	if v := os.Getenv(prefix + "LOG_MAX_BACKUPS"); v != "" {
		i, _ := strconv.ParseInt(v, 10, 32)
		if i > 0 {
			c.Config.LogMaxBackups = int(i)
		}
	}

	if v := os.Getenv(prefix + "SESSION_SECRET"); v != "" {
		c.Config.SessionSecret = v
	}

	if v := os.Getenv(prefix + "ENCRYPTION_KEY"); v != "" {
		c.Config.EncryptionKey = v
	}

	if v := os.Getenv(prefix + "CHECK_FOR_UPDATES"); v != "" {
		c.Config.CheckForUpdates = strings.EqualFold(strings.ToLower(v), "true")
	}
}

func (c *AppConfig) DynamicReload(log zerolog.Logger) {
	// Initialize koanf instance
	k := koanf.New(".")

	// Load the initial config file with the appropriate parser
	provider := file.Provider(filepath.Join(c.Config.ConfigPath, "config.toml"))
	if err := k.Load(provider, toml.Parser()); err != nil {
		log.Fatal().Msg("error loading config file")
	}

	// Watch for changes in the configuration file
	provider.Watch(func(event interface{}, err error) {
		if err != nil {
			log.Fatal().Err(err).Msg("watch error")
			return
		}

		// Lock the configuration to ensure thread safety
		c.m.Lock()
		defer c.m.Unlock()
		log.Debug().Msg("config file changed, reloading...")

		// Reload the configuration
		k = koanf.New(".")
		if err := k.Load(provider, toml.Parser()); err != nil {
			log.Fatal().Msg("error loading config file")
		}

		// Update the Config fields
		if c.Config != nil {
			c.Config.LogLevel = k.String("LogLevel")
			err := logger.GetInstance().SetLogLevel(c.Config.LogLevel)
			if err != nil {
				log.Error().Err(err).Msg("error updating log level")
			}

			c.Config.LogPath = k.String("LogPath")
			c.Config.CheckForUpdates = k.Bool("CheckForUpdates")
		}

		log.Debug().Msg("config file reloaded!")
	})

	<-make(chan bool)
}

func (c *AppConfig) UpdateConfig() error {
	filePath := path.Join(c.Config.ConfigPath, "config.toml")

	f, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("could not read config file: %s", filePath))
	}

	lines := strings.Split(string(f), "\n")
	lines = c.processLines(lines)

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
		return errors.Wrap(err, fmt.Sprintf("could not write config file: %s", filePath))
	}

	return nil
}

func (c *AppConfig) processLines(lines []string) []string {
	// keep track of not found values to append at bottom
	var (
		foundLineUpdate   = false
		foundLineLogLevel = false
		foundLineLogPath  = false
	)

	for i, line := range lines {
		// set checkForUpdates
		if !foundLineUpdate && strings.Contains(line, "CheckForUpdates =") {
			lines[i] = fmt.Sprintf("CheckForUpdates = %t", c.Config.CheckForUpdates)
			foundLineUpdate = true
		}
		if !foundLineLogLevel && strings.Contains(line, "LogLevel =") {
			lines[i] = fmt.Sprintf(`LogLevel = "%s"`, c.Config.LogLevel)
			foundLineLogLevel = true
		}
		if !foundLineLogPath && strings.Contains(line, "LogPath =") {
			if c.Config.LogPath == "" {
				// Check if the line already has a value
				matches := strings.Split(line, "=")
				if len(matches) > 1 && strings.TrimSpace(matches[1]) != `""` {
					lines[i] = line // Preserve the existing line
				} else {
					lines[i] = `#LogPath = ""`
				}
			} else {
				lines[i] = fmt.Sprintf("LogPath = \"%s\"", c.Config.LogPath)
			}
			foundLineLogPath = true
		}
	}

	// append missing vars to bottom
	if !foundLineUpdate {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("CheckForUpdates = %t", c.Config.CheckForUpdates))
	}

	if !foundLineLogLevel {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(`LogLevel = "%s"`, c.Config.LogLevel))
	}

	if !foundLineLogPath {
		lines = append(lines, "")
		if c.Config.LogPath == "" {
			lines = append(lines, `#LogPath = ""`)
		} else {
			lines = append(lines, fmt.Sprintf(`LogPath = "%s"`, c.Config.LogPath))
		}
	}

	return lines
}
