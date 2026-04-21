package config

import (
	"context"

	"github.com/opencloud-eu/opencloud/pkg/shared"
)

// Config combines all available configuration parts.
type Config struct {
	Commons *shared.Commons `yaml:"-"` // don't use this directly as configuration for a service

	Service Service `yaml:"-"`

	LogLevel string `yaml:"loglevel" env:"OC_LOG_LEVEL;MUSIC_LOG_LEVEL" desc:"The log level. Valid values are: 'panic', 'fatal', 'error', 'warn', 'info', 'debug', 'trace'."`
	Debug    Debug  `yaml:"debug"`

	HTTP HTTP `yaml:"http"`

	// OpenCloud API configuration. The music service is a pure HTTP client:
	// it authenticates against OpenCloud's Graph API by forwarding the
	// user's HTTP Basic credentials (username + app token) from the
	// Subsonic request, and streams audio bytes via WebDAV. No CS3/Reva
	// gateway, no OIDC exchange.
	OpenCloud OpenCloud `yaml:"opencloud"`

	Context context.Context `yaml:"-"`
}

// Service defines the service name.
type Service struct {
	Name string
}

// Debug defines the available debug configuration.
type Debug struct {
	Addr   string `yaml:"addr" env:"MUSIC_DEBUG_ADDR" desc:"Bind address of the debug server."`
	Token  string `yaml:"token" env:"MUSIC_DEBUG_TOKEN" desc:"Token to secure the metrics endpoint."`
	Pprof  bool   `yaml:"pprof" env:"MUSIC_DEBUG_PPROF" desc:"Enables pprof."`
	Zpages bool   `yaml:"zpages" env:"MUSIC_DEBUG_ZPAGES" desc:"Enables zpages."`
}

// HTTP defines the available http configuration.
type HTTP struct {
	Addr      string                `yaml:"addr" env:"MUSIC_HTTP_ADDR" desc:"The bind address of the HTTP service."`
	Namespace string                `yaml:"-"`
	Root      string                `yaml:"root" env:"MUSIC_HTTP_ROOT" desc:"Subdirectory that serves as the root for this HTTP service."`
	CORS      CORS                  `yaml:"cors"`
	TLS       shared.HTTPServiceTLS `yaml:"tls"`
}

// CORS defines the available cors configuration.
type CORS struct {
	AllowedOrigins   []string `yaml:"allow_origins" env:"OC_CORS_ALLOW_ORIGINS;MUSIC_CORS_ALLOW_ORIGINS"`
	AllowedMethods   []string `yaml:"allow_methods" env:"OC_CORS_ALLOW_METHODS;MUSIC_CORS_ALLOW_METHODS"`
	AllowedHeaders   []string `yaml:"allow_headers" env:"OC_CORS_ALLOW_HEADERS;MUSIC_CORS_ALLOW_HEADERS"`
	AllowCredentials bool     `yaml:"allow_credentials" env:"OC_CORS_ALLOW_CREDENTIALS;MUSIC_CORS_ALLOW_CREDENTIALS"`
}

// OpenCloud holds the URLs and TLS settings for talking to the OpenCloud
// Graph API and WebDAV endpoints. URL is used for both by default;
// InternalURL, if set, overrides for service-to-service traffic inside the
// cluster.
type OpenCloud struct {
	URL         string `yaml:"url" env:"OC_URL;MUSIC_OPENCLOUD_URL" desc:"Base URL of the OpenCloud instance (used for Graph API and WebDAV calls)."`
	InternalURL string `yaml:"internal_url" env:"OC_INTERNAL_URL;MUSIC_OPENCLOUD_INTERNAL_URL" desc:"Optional internal URL used for service-to-service traffic (defaults to URL)."`
	Insecure    bool   `yaml:"insecure" env:"OC_INSECURE;MUSIC_INSECURE" desc:"Skip TLS verification when talking to OpenCloud (self-signed certs in dev)."`
}
