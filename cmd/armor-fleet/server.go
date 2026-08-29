// Package main provides the HTTP server for armor-fleet.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server serves the fleet status HTTP endpoints.
type Server struct {
	monitor *FleetMonitor
	address string
}

// NewServer creates a new fleet server.
func NewServer(monitor *FleetMonitor, address string) *Server {
	return &Server{
		monitor: monitor,
		address: address,
	}
}

// Run starts the HTTP server.
func (s *Server) Run() error {
	mux := http.NewServeMux()

	// Fleet JSON endpoint
	mux.HandleFunc("/fleet.json", s.handleFleetJSON)

	// Metrics endpoint (Prometheus)
	mux.Handle("/metrics", promhttp.Handler())

	// Agentation.js endpoint
	mux.HandleFunc("/agentation.js", s.handleAgentationJS)

	// Root endpoint - HTML dashboard
	mux.HandleFunc("/", s.handleHTML)

	server := &http.Server{
		Addr:    s.address,
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", s.address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutdown signal received")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// handleFleetJSON serves the fleet status as JSON.
func (s *Server) handleFleetJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.monitor.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		log.Printf("Failed to encode fleet.json: %v", err)
	}
}

// handleHTML serves the fleet dashboard HTML.
func (s *Server) handleHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlTemplate)
}

// handleAgentationJS serves the Agentation toolbar script.
func (s *Server) handleAgentationJS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	fmt.Fprint(w, agentationJSScript)
}

// agentationJSScript is the Agentation toolbar JavaScript.
const agentationJSScript = `// Agentation toolbar placeholder
// This file serves as a placeholder for the Agentation visual feedback tool.
// In production, this would be replaced by the actual agentation.js implementation.

console.log('Agentation toolbar placeholder loaded');
`

// htmlTemplate is the self-contained HTML dashboard.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ARMOR Fleet Dashboard</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 2rem;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
        }

        h1 {
            color: white;
            text-align: center;
            margin-bottom: 2rem;
            font-size: 2.5rem;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.2);
        }

        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }

        .summary-card {
            background: white;
            padding: 1.5rem;
            border-radius: 12px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            text-align: center;
        }

        .summary-card h3 {
            font-size: 0.875rem;
            color: #666;
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        .summary-card .value {
            font-size: 2.5rem;
            font-weight: bold;
            color: #333;
        }

        .summary-card.up .value {
            color: #10b981;
        }

        .summary-card.down .value {
            color: #ef4444;
        }

        .targets-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
            gap: 1.5rem;
        }

        .target-card {
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
            overflow: hidden;
            transition: transform 0.2s, box-shadow 0.2s;
        }

        .target-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 12px rgba(0,0,0,0.15);
        }

        .target-header {
            padding: 1.25rem;
            background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);
            border-bottom: 1px solid #dee2e6;
        }

        .target-name {
            font-size: 1.25rem;
            font-weight: 600;
            color: #212529;
            margin-bottom: 0.25rem;
        }

        .target-location {
            font-size: 0.875rem;
            color: #6c757d;
        }

        .status-badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-top: 0.5rem;
        }

        .status-badge.reachable {
            background: #d1fae5;
            color: #065f46;
        }

        .status-badge.unreachable {
            background: #fee2e2;
            color: #991b1b;
        }

        .target-body {
            padding: 1.25rem;
        }

        .metric-row {
            display: flex;
            justify-content: space-between;
            padding: 0.75rem 0;
            border-bottom: 1px solid #f1f3f5;
        }

        .metric-row:last-child {
            border-bottom: none;
        }

        .metric-label {
            font-size: 0.875rem;
            color: #6c757d;
        }

        .metric-value {
            font-size: 0.875rem;
            font-weight: 600;
            color: #212529;
        }

        .metric-value.healthy {
            color: #10b981;
        }

        .metric-value.unhealthy {
            color: #ef4444;
        }

        .loading {
            text-align: center;
            padding: 3rem;
            color: white;
            font-size: 1.25rem;
        }

        .error-message {
            background: #fee2e2;
            color: #991b1b;
            padding: 0.75rem;
            border-radius: 6px;
            font-size: 0.875rem;
            margin-top: 0.75rem;
        }

        .last-updated {
            text-align: center;
            color: white;
            margin-top: 2rem;
            font-size: 0.875rem;
            opacity: 0.9;
        }

        .gauges-list {
            margin-top: 0.5rem;
        }

        .gauge-item {
            font-size: 0.75rem;
            padding: 0.25rem 0;
            color: #6c757d;
        }
    </style>
    <script type="importmap">
    {
        "imports": {
            "react": "https://esm.sh/react@18.3.1",
            "react-dom": "https://esm.sh/react-dom@18.3.1",
            "react-dom/client": "https://esm.sh/react-dom@18.3.1/client",
            "react/jsx-runtime": "https://esm.sh/react@18.3.1/jsx-runtime"
        }
    }
    </script>
    <script type="module" src="/agentation.js"></script>
</head>
<body>
    <div class="container">
        <h1>ARMOR Fleet Dashboard</h1>

        <div class="summary">
            <div class="summary-card">
                <h3>Total Targets</h3>
                <div class="value" id="total-targets">-</div>
            </div>
            <div class="summary-card up">
                <h3>Reachable</h3>
                <div class="value" id="reachable-targets">-</div>
            </div>
            <div class="summary-card down">
                <h3>Unreachable</h3>
                <div class="value" id="unreachable-targets">-</div>
            </div>
        </div>

        <div class="targets-grid" id="targets-grid">
            <div class="loading">Loading fleet status...</div>
        </div>

        <div class="last-updated">
            Last updated: <span id="last-updated">-</span>
        </div>
    </div>

    <script>
        let targets = {};

        function formatTimestamp(isoString) {
            const date = new Date(isoString);
            return date.toLocaleString();
        }

        function renderTargets() {
            const grid = document.getElementById('targets-grid');
            const totalEl = document.getElementById('total-targets');
            const reachableEl = document.getElementById('reachable-targets');
            const unreachableEl = document.getElementById('unreachable-targets');
            const lastUpdatedEl = document.getElementById('last-updated');

            const targetNames = Object.keys(targets);
            if (targetNames.length === 0) {
                grid.innerHTML = '<div class="loading">No targets configured</div>';
                return;
            }

            let reachableCount = 0;
            let unreachableCount = 0;

            const cards = targetNames.map(name => {
                const t = targets[name];
                if (t.reachable) {
                    reachableCount++;
                } else {
                    unreachableCount++;
                }

                const statusClass = t.reachable ? 'reachable' : 'unreachable';
                const statusText = t.reachable ? 'Reachable' : 'Unreachable';

                let metricsHtml = '';
                if (t.reachable) {
                    metricsHtml = '<div class="metric-row">' +
                        '<span class="metric-label">Version</span>' +
                        '<span class="metric-value">' + escapeHtml(t.version || 'unknown') + '</span>' +
                        '</div>' +
                        '<div class="metric-row">' +
                        '<span class="metric-label">Canary Status</span>' +
                        '<span class="metric-value ' + (t.canary_healthy ? 'healthy' : 'unhealthy') + '">' +
                        (t.canary_healthy ? 'Healthy' : 'Unhealthy') +
                        '</span>' +
                        '</div>';

                    if (t.canary_message) {
                        metricsHtml += '<div class="metric-row">' +
                            '<span class="metric-label">Canary Message</span>' +
                            '<span class="metric-value">' + escapeHtml(t.canary_message) + '</span>' +
                            '</div>';
                    }

                    metricsHtml += '<div class="metric-row">' +
                        '<span class="metric-label">Multipart Canary</span>' +
                        '<span class="metric-value">' + (t.multipart_canary ? 'Yes' : 'No') + '</span>' +
                        '</div>';

                    if (t.error_rate !== undefined) {
                        metricsHtml += '<div class="metric-row">' +
                            '<span class="metric-label">Error Rate</span>' +
                            '<span class="metric-value">' + t.error_rate.toFixed(2) + '</span>' +
                            '</div>';
                    }

                    if (t.restore_verifier_gauges && Object.keys(t.restore_verifier_gauges).length > 0) {
                        metricsHtml += '<div class="metric-row">' +
                            '<span class="metric-label">Restore Verifier</span>' +
                            '<span class="metric-value">' +
                            '<div class="gauges-list">';
                        for (const [gaugeName, gaugeValue] of Object.entries(t.restore_verifier_gauges)) {
                            metricsHtml += '<div class="gauge-item">' + escapeHtml(gaugeName) + ': ' + escapeHtml(gaugeValue) + '</div>';
                        }
                        metricsHtml += '</div></span></div>';
                    }

                    metricsHtml += '<div class="metric-row">' +
                        '<span class="metric-label">Last Seen</span>' +
                        '<span class="metric-value">' + formatTimestamp(t.last_seen) + '</span>' +
                        '</div>';
                } else {
                    metricsHtml = '<div class="error-message">' + escapeHtml(t.error || 'Unknown error') + '</div>';
                }

                return '<div class="target-card">' +
                    '<div class="target-header">' +
                    '<div class="target-name">' + escapeHtml(t.name) + '</div>' +
                    '<div class="target-location">' + escapeHtml(t.cluster + '/' + t.namespace + '/' + t.service) + '</div>' +
                    '<span class="status-badge ' + statusClass + '">' + statusText + '</span>' +
                    '</div>' +
                    '<div class="target-body">' +
                    metricsHtml +
                    '</div>' +
                    '</div>';
            });

            grid.innerHTML = cards.join('');
            totalEl.textContent = targetNames.length;
            reachableEl.textContent = reachableCount;
            unreachableEl.textContent = unreachableCount;
            lastUpdatedEl.textContent = formatTimestamp(new Date().toISOString());
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function loadFleetStatus() {
            fetch('/fleet.json')
                .then(response => response.json())
                .then(data => {
                    targets = data;
                    renderTargets();
                })
                .catch(error => {
                    console.error('Failed to load fleet status:', error);
                    document.getElementById('targets-grid').innerHTML =
                        '<div class="loading">Failed to load fleet status</div>';
                });
        }

        // Initial load
        loadFleetStatus();

        // Refresh every 30 seconds
        setInterval(loadFleetStatus, 30000);
    </script>
</body>
</html>
`
