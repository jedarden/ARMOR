package server

import (
	"fmt"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
)

// NewWithBackend builds an ARMOR *Server whose full S3 request pipeline
// (SigV4 auth, aws-chunked streaming decode, ACL enforcement, and the real S3
// handlers in internal/server/handlers) runs against an injected backend
// instead of a real B2/Cloudflare deployment.
//
// It exists so that integration and compatibility tests — notably
// tests/aws-cli-compatibility, which shells out to the real `aws` and `rclone`
// CLIs — can exercise the production request path end-to-end without cloud
// credentials or network access. The returned Server's Handler() is the same
// authenticated mux the live server serves.
//
// Canary, provenance, manifest, dashboard, presign, and B2-keys wiring are
// intentionally omitted: they are not exercised by S3 data-plane operations
// (cp/ls/rm/sync) and would require cloud connectivity to initialise.
// Manifest and provenance are nil, which the handlers already support (the
// existing handlers tests construct Handlers without them). canaryDisabled is
// set so /readyz would not dereference a nil canary monitor if it were hit.
func NewWithBackend(cfg *config.Config, be backend.Backend) (*Server, error) {
	routes := make([]keymanager.Route, 0, len(cfg.KeyRoutes))
	for _, r := range cfg.KeyRoutes {
		routes = append(routes, keymanager.Route{Prefix: r.Prefix, KeyName: r.KeyName})
	}
	km, err := keymanager.New(cfg.MEK, cfg.NamedKeys, routes)
	if err != nil {
		return nil, fmt.Errorf("test server: create key manager: %w", err)
	}

	return &Server{
		config:         cfg,
		backend:        be,
		cache:          backend.NewMetadataCache(cfg.CacheMaxEntries, cfg.CacheTTL),
		footerCache:    backend.NewFooterCache(cfg.CacheMaxEntries, cfg.CacheTTL),
		keyManager:     km,
		canaryDisabled: true,
		metrics:        metrics.NewMetrics(),
		requestTracker: &metrics.RequestTracker{},
		logger:         logging.New("armor-test"),
	}, nil
}
