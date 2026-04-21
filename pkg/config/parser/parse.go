package parser

import (
	"errors"
	"net/url"

	occfg "github.com/opencloud-eu/opencloud/pkg/config"
	"github.com/opencloud-eu/opencloud/pkg/config/envdecode"

	"github.com/opencloud-eu/opencloud-music/pkg/config"
	"github.com/opencloud-eu/opencloud-music/pkg/config/defaults"
)

// ParseConfig loads configuration from known sources (config files, env vars).
func ParseConfig(cfg *config.Config) error {
	if err := occfg.BindSourcesToStructs(cfg.Service.Name, cfg); err != nil {
		return err
	}

	defaults.EnsureDefaults(cfg)

	if err := envdecode.Decode(cfg); err != nil {
		if !errors.Is(err, envdecode.ErrNoTargetFieldsAreSet) {
			return err
		}
	}

	defaults.Sanitize(cfg)

	return Validate(cfg)
}

// Validate checks mandatory fields and URL shape.
func Validate(cfg *config.Config) error {
	if cfg.OpenCloud.URL == "" {
		return errors.New("opencloud-music: OC_URL is required")
	}
	u, err := url.Parse(cfg.OpenCloud.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("opencloud-music: OC_URL must be http(s)://...")
	}
	return nil
}
