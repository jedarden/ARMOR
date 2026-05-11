// Package dashboard provides a web dashboard for ARMOR.
package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// AuthMiddleware handles authentication for dashboard routes.
type AuthMiddleware struct {
	user  string
	pass  string
	token string
}

// NewAuthMiddleware creates a new authentication middleware.
// If both user/pass and token are empty, no authentication is performed.
func NewAuthMiddleware(user, pass, token string) *AuthMiddleware {
	return &AuthMiddleware{
		user:  user,
		pass:  pass,
		token: token,
	}
}

// Authenticate checks the request for valid authentication.
// Returns true if authenticated, false otherwise.
// When returning false, it also sets the WWW-Authenticate header.
func (a *AuthMiddleware) Authenticate(w http.ResponseWriter, r *http.Request) bool {
	// No auth configured - allow all
	if a.user == "" && a.token == "" {
		return true
	}

	// Check Bearer token first
	if a.token != "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == a.token {
				return true
			}
		}
	}

	// Check HTTP Basic Auth
	if a.user != "" && a.pass != "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Basic ") {
			encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
			decodedCreds, err := base64.StdEncoding.DecodeString(encodedCreds)
			if err == nil {
				creds := strings.SplitN(string(decodedCreds), ":", 2)
				if len(creds) == 2 && creds[0] == a.user && creds[1] == a.pass {
					return true
				}
			}
		}
	}

	// Authentication failed - set WWW-Authenticate header
	w.Header().Set("WWW-Authenticate", `Basic realm="ARMOR Dashboard"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

// Wrap wraps a handler with authentication middleware.
func (a *AuthMiddleware) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Authenticate(w, r) {
			return
		}
		h(w, r)
	}
}

// Dashboard provides the web dashboard handlers.
type Dashboard struct {
	backend  backend.Backend
	bucket   string
	metrics  *metrics.Metrics
	template *template.Template
	auth     *AuthMiddleware
}

// New creates a new Dashboard.
func New(b backend.Backend, bucket string, m *metrics.Metrics) *Dashboard {
	return NewWithAuth(b, bucket, m, "", "", "")
}

// NewWithAuth creates a new Dashboard with authentication.
func NewWithAuth(b backend.Backend, bucket string, m *metrics.Metrics, user, pass, token string) *Dashboard {
	d := &Dashboard{
		backend: b,
		bucket:  bucket,
		metrics: m,
		auth:    NewAuthMiddleware(user, pass, token),
	}
	d.template = template.Must(template.New("dashboard").Parse(dashboardHTML))
	return d
}

// KeyRotateStatusHandler returns the current key rotation status.
// This polls the rotation state file from B2 for progress information.
func (d *Dashboard) KeyRotateStatusHandler() http.HandlerFunc {
	return d.keyRotateStatusHandlerImpl()
}

// KeyRotateStatusHandlerWithAuth returns the key rotation status handler with authentication.
func (d *Dashboard) KeyRotateStatusHandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.keyRotateStatusHandlerImpl())
}

// keyRotateStatusHandlerImpl is the actual implementation of the key rotation status handler.
func (d *Dashboard) keyRotateStatusHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		// Try to read the rotation state file
		statePath := ".armor/rotation-state.json"
		reader, _, err := d.backend.GetDirect(ctx, d.bucket, statePath)
		if err != nil {
			// No rotation state file means no rotation in progress
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "none",
				"message": "No rotation in progress",
			})
			return
		}
		defer reader.Close()

		data, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read state: %v", err), http.StatusInternalServerError)
			return
		}

		// Return the raw state file content
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

// Handler returns the main dashboard handler.
func (d *Dashboard) Handler() http.HandlerFunc {
	return d.handlerImpl()
}

// HandlerWithAuth returns the main dashboard handler with authentication.
func (d *Dashboard) HandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.handlerImpl())
}

// handlerImpl is the actual implementation of the main dashboard handler.
func (d *Dashboard) handlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get prefix from query
		prefix := r.URL.Query().Get("prefix")

		// List objects
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := d.backend.List(ctx, d.bucket, prefix, "/", "", 1000)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
			return
		}

		// Build page data
		data := d.buildPageData(result, prefix)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := d.template.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// ObjectInfo represents an object in the dashboard.
type ObjectInfo struct {
	Key            string
	Size           int64
	PlaintextSize  int64
	LastModified   string
	IsARMOR        bool
	ContentType    string
	ETag           string
	KeyID          string
	BlockSize      int
	IsFolder       bool
	PlaintextSizeH string
}

// PageData holds data for the dashboard template.
type PageData struct {
	Bucket       string
	Prefix       string
	Objects      []ObjectInfo
	CacheHits    string
	CacheMisses  string
	CacheHitRate string
	Uptime       string
	Requests     string
	BytesUp      string
	BytesDown    string
	CanaryStatus string
	Breadcrumbs  []Breadcrumb
}

// Breadcrumb represents a navigation breadcrumb.
type Breadcrumb struct {
	Name string
	Path string
}

func (d *Dashboard) buildPageData(result *backend.ListResult, prefix string) PageData {
	objects := make([]ObjectInfo, 0, len(result.CommonPrefixes)+len(result.Objects))

	// Add common prefixes (virtual folders) first so they appear at the top.
	for _, cp := range result.CommonPrefixes {
		objects = append(objects, ObjectInfo{
			Key:            cp,
			IsFolder:       true,
			PlaintextSizeH: "—",
		})
	}

	for _, obj := range result.Objects {
		info := ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format("2006-01-02 15:04:05"),
			ContentType:  obj.ContentType,
			ETag:         obj.ETag,
			IsFolder:     strings.HasSuffix(obj.Key, "/"),
		}

		// Check if ARMOR-encrypted
		if obj.IsARMOREncrypted {
			info.IsARMOR = true
			// Size is already plaintext size for ARMOR objects
			info.PlaintextSize = obj.Size
			info.PlaintextSizeH = formatBytes(obj.Size)
			// Extract additional ARMOR metadata
			if armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata); ok {
				info.KeyID = armorMeta.KeyID
				info.BlockSize = armorMeta.BlockSize
			}
		} else {
			info.PlaintextSize = obj.Size
			info.PlaintextSizeH = formatBytes(obj.Size)
		}

		objects = append(objects, info)
	}

	// Build breadcrumbs
	breadcrumbs := []Breadcrumb{{Name: d.bucket, Path: ""}}
	if prefix != "" {
		parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
		path := ""
		for _, part := range parts {
			if part == "" {
				continue
			}
			path += part + "/"
			breadcrumbs = append(breadcrumbs, Breadcrumb{Name: part, Path: path})
		}
	}

	// Get metrics
	cacheHits := d.metrics.CacheHitsTotal.String()
	cacheMisses := d.metrics.CacheMissesTotal.String()
	hitRate := "0%"
	if hits, misses := parseExpvarInt(cacheHits), parseExpvarInt(cacheMisses); hits+misses > 0 {
		rate := float64(hits) / float64(hits+misses) * 100
		hitRate = fmt.Sprintf("%.1f%%", rate)
	}

	return PageData{
		Bucket:       d.bucket,
		Prefix:       prefix,
		Objects:      objects,
		CacheHits:    cacheHits,
		CacheMisses:  cacheMisses,
		CacheHitRate: hitRate,
		Uptime:       formatUptime(time.Since(d.metrics.StartTime())),
		Requests:     d.metrics.RequestsTotal.String(),
		BytesUp:      formatBytes(parseExpvarInt(d.metrics.BytesUploaded.String())),
		BytesDown:    formatBytes(parseExpvarInt(d.metrics.BytesDownloaded.String())),
		CanaryStatus: d.getCanaryStatus(),
		Breadcrumbs:  breadcrumbs,
	}
}

func (d *Dashboard) getCanaryStatus() string {
	lastCheck := d.metrics.CanaryLastCheckTime.String()
	failures := d.metrics.CanaryCheckFailures.String()

	if parseExpvarInt(failures) > 0 {
		return fmt.Sprintf("Unhealthy (failures: %s)", failures)
	}
	if lastCheck == "" {
		return "Not started"
	}
	return fmt.Sprintf("Healthy (last check: %s)", lastCheck)
}

// ObjectDetailHandler returns details for a specific object.
func (d *Dashboard) ObjectDetailHandler() http.HandlerFunc {
	return d.objectDetailHandlerImpl()
}

// ObjectDetailHandlerWithAuth returns the object detail handler with authentication.
func (d *Dashboard) ObjectDetailHandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.objectDetailHandlerImpl())
}

// objectDetailHandlerImpl is the actual implementation of the object detail handler.
func (d *Dashboard) objectDetailHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key parameter required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		info, err := d.backend.Head(ctx, d.bucket, key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get object: %v", err), http.StatusNotFound)
			return
		}

		// Build response
		detail := map[string]interface{}{
			"key":           key,
			"size":          info.Size,
			"content_type":  info.ContentType,
			"etag":          info.ETag,
			"last_modified": info.LastModified.Format(time.RFC3339),
			"is_armor":      info.IsARMOREncrypted,
		}

		if info.IsARMOREncrypted {
			armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
			if ok {
				detail["armor"] = map[string]interface{}{
					"plaintext_size": armorMeta.PlaintextSize,
					"block_size":     armorMeta.BlockSize,
					"key_id":         armorMeta.KeyID,
					"iv":             fmt.Sprintf("%x", armorMeta.IV),
					"wrapped_dek":    fmt.Sprintf("%x", armorMeta.WrappedDEK),
					"sha256":         armorMeta.PlaintextSHA,
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(detail)
	}
}

// MetricsHandler returns metrics in JSON format.
func (d *Dashboard) MetricsHandler() http.HandlerFunc {
	return d.metricsHandlerImpl()
}

// MetricsHandlerWithAuth returns the metrics handler with authentication.
func (d *Dashboard) MetricsHandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.metricsHandlerImpl())
}

// metricsHandlerImpl is the actual implementation of the metrics handler.
func (d *Dashboard) metricsHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := d.metrics
		data := map[string]interface{}{
			"requests_total":         parseExpvarInt(m.RequestsTotal.String()),
			"requests_in_flight":     parseExpvarInt(m.RequestsInFlight.String()),
			"bytes_uploaded":         parseExpvarInt(m.BytesUploaded.String()),
			"bytes_downloaded":       parseExpvarInt(m.BytesDownloaded.String()),
			"bytes_fetched_from_b2":  parseExpvarInt(m.BytesFetchedFromB2.String()),
			"range_reads_total":      parseExpvarInt(m.RangeReadsTotal.String()),
			"range_bytes_saved":      parseExpvarInt(m.RangeBytesSavedTotal.String()),
			"cache_hits":             parseExpvarInt(m.CacheHitsTotal.String()),
			"cache_misses":           parseExpvarInt(m.CacheMissesTotal.String()),
			"key_wrap_ops":           parseExpvarInt(m.KeyWrapOpsTotal.String()),
			"key_unwrap_ops":         parseExpvarInt(m.KeyUnwrapOpsTotal.String()),
			"canary_checks":          parseExpvarInt(m.CanaryChecksTotal.String()),
			"canary_failures":        parseExpvarInt(m.CanaryCheckFailures.String()),
			"active_multipart":       parseExpvarInt(m.ActiveMultipartUploads.String()),
			"provenance_entries":     parseExpvarInt(m.ProvenanceEntriesTotal.String()),
			"uptime_seconds":         time.Since(m.StartTime()).Seconds(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

// KeyRotateHandler handles key rotation requests.
// It generates a new MEK and calls the admin API to perform rotation.
func (d *Dashboard) KeyRotateHandler(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return d.keyRotateHandlerImpl(adminClient, adminURL)
}

// KeyRotateHandlerWithAuth handles key rotation requests with authentication.
func (d *Dashboard) KeyRotateHandlerWithAuth(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return d.auth.Wrap(d.keyRotateHandlerImpl(adminClient, adminURL))
}

// keyRotateHandlerImpl is the actual implementation of the key rotation handler.
func (d *Dashboard) keyRotateHandlerImpl(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate a new MEK (32 random bytes)
		newMEK := make([]byte, 32)
		if _, err := rand.Read(newMEK); err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate new MEK: %v", err), http.StatusInternalServerError)
			return
		}

		// Call the admin API's rotate endpoint
		rotateURL := adminURL
		if rotateURL == "" {
			rotateURL = "http://127.0.0.1:9001/admin/key/rotate"
		}

		// Send the new MEK as hex-encoded string
		mekHex := hex.EncodeToString(newMEK)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, rotateURL, strings.NewReader(mekHex))
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := adminClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to call admin API: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers and body
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// Helper functions

func parseExpvarInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

// The HTML template for the dashboard
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ARMOR Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: #f5f7fa;
            color: #333;
            line-height: 1.6;
        }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        header {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            color: white;
            padding: 20px;
            margin-bottom: 20px;
            border-radius: 8px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        header h1 { font-size: 24px; margin-bottom: 5px; }
        header .subtitle { opacity: 0.8; font-size: 14px; }
        .rotate-btn {
            background: #f59e0b;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.2s;
        }
        .rotate-btn:hover { background: #d97706; }
        .rotate-btn:disabled { background: #9ca3af; cursor: not-allowed; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }
        .stat-card {
            background: white;
            padding: 15px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-card h3 {
            font-size: 12px;
            text-transform: uppercase;
            color: #666;
            margin-bottom: 5px;
        }
        .stat-card .value {
            font-size: 24px;
            font-weight: 600;
            color: #1a1a2e;
        }
        .stat-card.healthy .value { color: #10b981; }
        .stat-card.unhealthy .value { color: #ef4444; }
        .breadcrumbs {
            background: white;
            padding: 10px 15px;
            border-radius: 8px;
            margin-bottom: 15px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .breadcrumbs a {
            color: #3b82f6;
            text-decoration: none;
        }
        .breadcrumbs a:hover { text-decoration: underline; }
        .breadcrumbs span { color: #666; margin: 0 5px; }
        .objects-table {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px 15px; text-align: left; }
        th {
            background: #f8fafc;
            font-weight: 600;
            font-size: 12px;
            text-transform: uppercase;
            color: #666;
            border-bottom: 1px solid #e2e8f0;
        }
        td { border-bottom: 1px solid #f1f5f9; }
        tr:hover { background: #f8fafc; }
        .key-cell {
            max-width: 400px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .key-cell a {
            color: #3b82f6;
            text-decoration: none;
        }
        .key-cell a:hover { text-decoration: underline; }
        .armor-badge {
            display: inline-block;
            background: #10b981;
            color: white;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
        }
        .folder-icon { margin-right: 5px; }
        .size-cell { font-family: monospace; color: #666; }
        .date-cell { color: #666; font-size: 13px; }
        footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 12px;
        }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }
        .modal.active { display: flex; }
        .modal-content {
            background: white;
            padding: 30px;
            border-radius: 8px;
            max-width: 500px;
            width: 90%;
            box-shadow: 0 4px 20px rgba(0,0,0,0.2);
        }
        .modal-title {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 15px;
            color: #1a1a2e;
        }
        .modal-body { margin-bottom: 20px; }
        .progress-bar {
            width: 100%;
            height: 8px;
            background: #e5e7eb;
            border-radius: 4px;
            overflow: hidden;
            margin-top: 10px;
        }
        .progress-fill {
            height: 100%;
            background: #10b981;
            transition: width 0.3s;
        }
        .modal-buttons {
            display: flex;
            gap: 10px;
            justify-content: flex-end;
        }
        .btn {
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            border: none;
        }
        .btn-primary { background: #3b82f6; color: white; }
        .btn-primary:hover { background: #2563eb; }
        .btn-secondary { background: #e5e7eb; color: #374151; }
        .btn-secondary:hover { background: #d1d5db; }
        .status-message {
            margin-top: 10px;
            padding: 10px;
            border-radius: 6px;
            font-size: 13px;
        }
        .status-success { background: #d1fae5; color: #065f46; }
        .status-error { background: #fee2e2; color: #991b1b; }
        .status-info { background: #dbeafe; color: #1e40af; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>ARMOR Dashboard</h1>
                <div class="subtitle">S3-compatible transparent encryption proxy</div>
            </div>
            <button class="rotate-btn" onclick="startKeyRotation()">Rotate Key</button>
        </header>

        <div class="stats-grid">
            <div class="stat-card">
                <h3>Cache Hit Rate</h3>
                <div class="value">{{.CacheHitRate}}</div>
            </div>
            <div class="stat-card">
                <h3>Cache Hits / Misses</h3>
                <div class="value">{{.CacheHits}} / {{.CacheMisses}}</div>
            </div>
            <div class="stat-card">
                <h3>Total Requests</h3>
                <div class="value">{{.Requests}}</div>
            </div>
            <div class="stat-card">
                <h3>Bytes Uploaded</h3>
                <div class="value">{{.BytesUp}}</div>
            </div>
            <div class="stat-card">
                <h3>Bytes Downloaded</h3>
                <div class="value">{{.BytesDown}}</div>
            </div>
            <div class="stat-card">
                <h3>Uptime</h3>
                <div class="value">{{.Uptime}}</div>
            </div>
            <div class="stat-card">
                <h3>Canary Status</h3>
                <div class="value">{{.CanaryStatus}}</div>
            </div>
        </div>

        <div class="breadcrumbs">
            {{range $i, $crumb := .Breadcrumbs}}{{if $i}}<span>›</span>{{end}}<a href="?prefix={{$crumb.Path}}">{{$crumb.Name}}</a>{{end}}
        </div>

        <div class="objects-table">
            <table>
                <thead>
                    <tr>
                        <th>Key</th>
                        <th>Plaintext Size</th>
                        <th>Type</th>
                        <th>Last Modified</th>
                        <th>Encryption</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Objects}}
                    <tr>
                        <td class="key-cell">
                            {{if .IsFolder}}
                                <span class="folder-icon">📁</span>
                                <a href="?prefix={{.Key}}">{{.Key}}</a>
                            {{else}}
                                <a href="#" onclick="showDetail('{{.Key}}')">{{.Key}}</a>
                            {{end}}
                        </td>
                        <td class="size-cell">{{.PlaintextSizeH}}</td>
                        <td>{{.ContentType}}</td>
                        <td class="date-cell">{{.LastModified}}</td>
                        <td>
                            {{if .IsARMOR}}
                                <span class="armor-badge">ARMOR {{if .KeyID}}[{{.KeyID}}]{{end}}</span>
                            {{else if .IsFolder}}
                                —
                            {{else}}
                                <span style="color:#999">plain</span>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>

        <footer>
            ARMOR — Transparent S3 Encryption • <a href="/metrics">Metrics</a> • <a href="/admin/key/verify">Key Status</a>
        </footer>
    </div>

    <div id="objectDetailModal" class="modal">
        <div class="modal-content" style="max-width:600px">
            <h2 class="modal-title">Object Details</h2>
            <div class="modal-body" id="objectDetailBody" style="font-family:monospace;font-size:13px;line-height:1.8"></div>
            <div class="modal-buttons">
                <button class="btn btn-secondary" onclick="closeDetailModal()">Close</button>
            </div>
        </div>
    </div>

    <div id="rotationModal" class="modal">
        <div class="modal-content">
            <h2 class="modal-title">Key Rotation</h2>
            <div class="modal-body">
                <div id="rotationStatus" class="status-message status-info">
                    Initiated key rotation...
                </div>
                <div class="progress-bar">
                    <div id="rotationProgressFill" class="progress-fill"></div>
                </div>
            </div>
            <div class="modal-buttons">
                <button class="btn btn-secondary" onclick="closeRotationModal()">Close</button>
            </div>
        </div>
    </div>

    <script>
    function row(label, value) {
        return '<div><strong style="color:#555;display:inline-block;width:160px">' + label + '</strong>' + escHtml(String(value)) + '</div>';
    }

    function escHtml(s) {
        return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }

    function closeDetailModal() {
        document.getElementById('objectDetailModal').classList.remove('active');
    }

    function showDetail(key) {
        fetch('/dashboard/object?key=' + encodeURIComponent(key))
            .then(r => r.json())
            .then(data => {
                let html = row('Key', data.key)
                         + row('Size', data.size + ' bytes')
                         + row('Content-Type', data.content_type || '—')
                         + row('ETag', data.etag || '—')
                         + row('Last Modified', data.last_modified || '—')
                         + row('Encrypted', data.is_armor ? 'Yes (ARMOR)' : 'No');
                if (data.armor) {
                    html += '<hr style="margin:10px 0;border:none;border-top:1px solid #e5e7eb">'
                          + row('Plaintext Size', data.armor.plaintext_size + ' bytes')
                          + row('Block Size', data.armor.block_size)
                          + row('Key ID', data.armor.key_id || 'default')
                          + row('SHA-256', data.armor.sha256 || '—');
                }
                document.getElementById('objectDetailBody').innerHTML = html;
                document.getElementById('objectDetailModal').classList.add('active');
            })
            .catch(err => {
                document.getElementById('objectDetailBody').innerHTML = '<span style="color:#991b1b">Failed to load: ' + escHtml(String(err)) + '</span>';
                document.getElementById('objectDetailModal').classList.add('active');
            });
    }

    let rotationInterval = null;

    function startKeyRotation() {
        const modal = document.getElementById('rotationModal');
        const statusMsg = document.getElementById('rotationStatus');
        const progressBar = document.getElementById('rotationProgress');
        const progressFill = document.getElementById('rotationProgressFill');
        const rotateBtn = document.querySelector('.rotate-btn');

        rotateBtn.disabled = true;
        modal.classList.add('active');
        statusMsg.className = 'status-message status-info';
        statusMsg.textContent = 'Initiating key rotation...';
        progressFill.style.width = '0%';

        fetch('/dashboard/admin/key/rotate', { method: 'POST' })
            .then(response => {
                if (!response.ok) {
                    return response.text().then(text => {
                        throw new Error(text || 'Rotation request failed');
                    });
                }
                // Start polling for progress
                rotationInterval = setInterval(pollRotationStatus, 2000);
                return null;
            })
            .catch(err => {
                statusMsg.className = 'status-message status-error';
                statusMsg.textContent = 'Error: ' + err.message;
                rotateBtn.disabled = false;
                if (rotationInterval) {
                    clearInterval(rotationInterval);
                    rotationInterval = null;
                }
            });
    }

    function pollRotationStatus() {
        const statusMsg = document.getElementById('rotationStatus');
        const progressFill = document.getElementById('rotationProgressFill');
        const rotateBtn = document.querySelector('.rotate-btn');

        fetch('/dashboard/admin/key/status')
            .then(r => r.json())
            .then(data => {
                if (data.status === 'none' || !data.status) {
                    // No state file yet - rotation may be initializing
                    return;
                }

                if (data.status === 'completed') {
                    clearInterval(rotationInterval);
                    rotationInterval = null;
                    statusMsg.className = 'status-message status-success';
                    statusMsg.textContent = 'Key rotation completed successfully!';
                    progressFill.style.width = '100%';
                    rotateBtn.disabled = false;
                    return;
                }

                if (data.status === 'failed') {
                    clearInterval(rotationInterval);
                    rotationInterval = null;
                    statusMsg.className = 'status-message status-error';
                    statusMsg.textContent = 'Rotation failed: ' + (data.error_message || 'Unknown error');
                    rotateBtn.disabled = false;
                    return;
                }

                if (data.status === 'in_progress') {
                    const total = data.total_objects || 0;
                    const processed = data.processed_objects || 0;
                    const percent = total > 0 ? Math.round((processed / total) * 100) : 0;

                    statusMsg.className = 'status-message status-info';
                    statusMsg.textContent = 'Rotating keys... ' + processed + ' / ' + total + ' objects (' + percent + '%)';
                    progressFill.style.width = percent + '%';
                }
            })
            .catch(err => {
                console.error('Failed to poll rotation status:', err);
            });
    }

    function closeRotationModal() {
        const modal = document.getElementById('rotationModal');
        modal.classList.remove('active');
        if (rotationInterval) {
            clearInterval(rotationInterval);
            rotationInterval = null;
        }
    }
    </script>
</body>
</html>
`
