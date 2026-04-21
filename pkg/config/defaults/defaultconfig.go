package defaults

import (
	"strings"

	"github.com/opencloud-eu/opencloud-music/pkg/config"
	"github.com/opencloud-eu/opencloud/pkg/shared"
)

// FullDefaultConfig returns the full default config.
func FullDefaultConfig() *config.Config {
	cfg := DefaultConfig()
	EnsureDefaults(cfg)
	Sanitize(cfg)
	return cfg
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *config.Config {
	return &config.Config{
		Commons: &shared.Commons{
			Log: &shared.Log{},
		},
		Debug: config.Debug{
			Addr: "127.0.0.1:9268",
		},
		Service: config.Service{
			Name: "music",
		},
		HTTP: config.HTTP{
			Addr:      "0.0.0.0:9110",
			Root:      "/",
			Namespace: "eu.opencloud.music",
			CORS: config.CORS{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "HEAD"},
				AllowedHeaders:   []string{"Authorization", "Origin", "Content-Type", "Accept", "Range", "X-API-Key", "X-Request-Id"},
				AllowCredentials: false,
			},
		},
		OpenCloud: config.OpenCloud{
			URL: "https://host.docker.internal:9200",
		},
	}
}

// EnsureDefaults fills in any dependent defaults. Runs BEFORE env-var
// binding — only use it for fields that don't depend on other
// user-configurable values.
func EnsureDefaults(_ *config.Config) {}

// Sanitize normalises the config (trims trailing slashes). Does NOT
// derive InternalURL from URL — that fallback lives in
// command/server.go, where it can run exactly once, after env-var
// binding, without fighting a second Sanitize pass.
func Sanitize(cfg *config.Config) {
	if cfg.HTTP.Root != "/" {
		cfg.HTTP.Root = strings.TrimSuffix(cfg.HTTP.Root, "/")
	}
	cfg.OpenCloud.URL = strings.TrimRight(cfg.OpenCloud.URL, "/")
	cfg.OpenCloud.InternalURL = strings.TrimRight(cfg.OpenCloud.InternalURL, "/")
}
