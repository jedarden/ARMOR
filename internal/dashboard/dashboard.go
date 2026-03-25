// Package dashboard provides a web dashboard for ARMOR.
package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
)

// Dashboard provides the web dashboard handlers.
type Dashboard struct {
	backend  backend.Backend
	bucket   string
	metrics  *metrics.Metrics
	template *template.Template
}

// New creates a new Dashboard.
func New(b backend.Backend, bucket string, m *metrics.Metrics) *Dashboard {
	d := &Dashboard{
		backend: b,
		bucket:  bucket,
		metrics: m,
	}
	d.template = template.Must(template.New("dashboard").Parse(dashboardHTML))
	return d
}

// Handler returns the main dashboard handler.
func (d *Dashboard) Handler() http.HandlerFunc {
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
	objects := make([]ObjectInfo, 0, len(result.Objects))

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
			"key":             key,
			"size":            info.Size,
			"content_type":    info.ContentType,
			"etag":            info.ETag,
			"last_modified":   info.LastModified.Format(time.RFC3339),
			"is_armor":        info.IsARMOREncrypted,
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
        }
        header h1 { font-size: 24px; margin-bottom: 5px; }
        header .subtitle { opacity: 0.8; font-size: 14px; }
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
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>ARMOR Dashboard</h1>
            <div class="subtitle">S3-compatible transparent encryption proxy</div>
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

    <script>
    function showDetail(key) {
        fetch('/dashboard/object?key=' + encodeURIComponent(key))
            .then(r => r.json())
            .then(data => {
                let msg = 'Object: ' + data.key + '\n\n';
                msg += 'Size: ' + data.size + ' bytes\n';
                msg += 'Type: ' + data.content_type + '\n';
                msg += 'ETag: ' + data.etag + '\n';
                msg += 'Modified: ' + data.last_modified + '\n\n';
                if (data.armor) {
                    msg += 'ARMOR Encryption:\n';
                    msg += '  Plaintext size: ' + data.armor.plaintext_size + ' bytes\n';
                    msg += '  Block size: ' + data.armor.block_size + '\n';
                    msg += '  Key ID: ' + data.armor.key_id + '\n';
                    msg += '  SHA256: ' + data.armor.sha256 + '\n';
                }
                alert(msg);
            })
            .catch(err => alert('Failed to load object details: ' + err));
    }
    </script>
</body>
</html>
`
