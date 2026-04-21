package command

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os/signal"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/opencloud-eu/opencloud/pkg/config/configlog"
	"github.com/opencloud-eu/opencloud/pkg/cors"
	"github.com/opencloud-eu/opencloud/pkg/log"
	"github.com/opencloud-eu/opencloud/pkg/middleware"
	"github.com/opencloud-eu/opencloud/pkg/runner"
	"github.com/opencloud-eu/opencloud/pkg/version"
	"github.com/spf13/cobra"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
	"github.com/opencloud-eu/opencloud-music/internal/graph"
	"github.com/opencloud-eu/opencloud-music/internal/stream"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic"
	"github.com/opencloud-eu/opencloud-music/pkg/config"
	"github.com/opencloud-eu/opencloud-music/pkg/config/parser"
)

// Server is the entrypoint for the server command.
func Server(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: fmt.Sprintf("start the %s service without runtime (unsupervised mode)", cfg.Service.Name),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return configlog.ReturnFatal(parser.ParseConfig(cfg))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := log.Configure(cfg.Service.Name, cfg.Commons, cfg.LogLevel)

			var cancel context.CancelFunc
			if cfg.Context == nil {
				cfg.Context, cancel = signal.NotifyContext(context.Background(), runner.StopSignals...)
				defer cancel()
			}
			ctx := cfg.Context

			// Resolve the Graph/WebDAV base URL exactly once here,
			// after env-var binding. Fall back to the public URL
			// when OC_INTERNAL_URL is unset (the common single-host
			// case).
			graphURL := cfg.OpenCloud.InternalURL
			if graphURL == "" {
				graphURL = cfg.OpenCloud.URL
			}

			graphClient, err := graph.New(graphURL, cfg.OpenCloud.Insecure)
			if err != nil {
				return fmt.Errorf("graph client: %w", err)
			}
			graphClient.SetLogger(logger)
			streamProxy := stream.New(cfg.OpenCloud.Insecure)

			mux := chi.NewMux()
			mux.Use(
				chimiddleware.RequestID,
				middleware.Version(cfg.Service.Name, version.GetString()),
				middleware.Logger(logger),
				middleware.Cors(
					cors.Logger(logger),
					cors.AllowedOrigins(cfg.HTTP.CORS.AllowedOrigins),
					cors.AllowedMethods(cfg.HTTP.CORS.AllowedMethods),
					cors.AllowedHeaders(cfg.HTTP.CORS.AllowedHeaders),
					cors.AllowCredentials(cfg.HTTP.CORS.AllowCredentials),
				),
				auth.Middleware,
			)

			// WebDAV URLs need the PUBLIC OpenCloud URL (what the
			// user's browser/app sees) because our stream proxy
			// hits it on behalf of the client. We hand that to the
			// Subsonic server so it can synthesise WebDAV URLs when
			// search hits don't carry one.
			srv := subsonic.NewServer(logger, graphClient, streamProxy)
			srv.SetPublicBaseURL(cfg.OpenCloud.URL)
			subsonic.Mount(mux, srv)

			server := &stdhttp.Server{
				Addr:    cfg.HTTP.Addr,
				Handler: mux,
			}

			gr := runner.NewGroup()
			gr.Add(runner.NewGolangHttpServerRunner(cfg.Service.Name+".http", server))

			logger.Info().
				Str("addr", cfg.HTTP.Addr).
				Str("opencloud_url", cfg.OpenCloud.URL).
				Str("graph_url", graphURL).
				Bool("insecure", cfg.OpenCloud.Insecure).
				Msg("opencloud-music listening")

			grResults := gr.Run(ctx)

			for _, grResult := range grResults {
				if grResult.RunnerError != nil {
					return grResult.RunnerError
				}
			}
			return nil
		},
	}
}
