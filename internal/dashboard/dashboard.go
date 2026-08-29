// Package dashboard provides a web dashboard for ARMOR.
package dashboard

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
	"expvar"
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
	backend         backend.Backend
	bucket          string
	metrics         *metrics.Metrics
	template        *template.Template
	auth            *AuthMiddleware
	dashboardCred   *DashboardCredential // Named credential for S3 operations
	serverBaseURL   string                // Base URL for S3 endpoint proxying
}

// DashboardCredential holds credential info for S3 operations
type DashboardCredential struct {
	Name      string
	AccessKey string
	SecretKey string
}

// New creates a new Dashboard.
func New(b backend.Backend, bucket string, m *metrics.Metrics) *Dashboard {
	return NewWithAuth(b, bucket, m, "", "", "", nil, "")
}

// NewWithAuth creates a new Dashboard with authentication.
func NewWithAuth(b backend.Backend, bucket string, m *metrics.Metrics, user, pass, token string, dashboardCred *DashboardCredential, serverBaseURL string) *Dashboard {
	d := &Dashboard{
		backend:       b,
		bucket:        bucket,
		metrics:       m,
		auth:          NewAuthMiddleware(user, pass, token),
		dashboardCred: dashboardCred,
		serverBaseURL: serverBaseURL,
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
				"status":  "none",
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
		continuationToken := r.URL.Query().Get("continuation_token")

		// List objects
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := d.backend.List(ctx, d.bucket, prefix, "/", continuationToken, 1000)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
			return
		}

		// Build page data
		data := d.buildPageData(result, prefix, continuationToken)

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
	Bucket          string
	Prefix          string
	Objects         []ObjectInfo
	CacheHits       string
	CacheMisses     string
	CacheHitRate    string
	Uptime          string
	Requests        string
	BytesUp         string
	BytesDown       string
	CanaryStatus    string
	CanaryCardClass string // "healthy", "unhealthy", or "" for not-yet-started
	Breadcrumbs     []Breadcrumb
	// Encryption statistics for the current listing
	EncryptedCount   int
	PlaintextCount   int
	TotalObjectCount int // EncryptedCount + PlaintextCount (excludes folders)
	FolderCount      int
	KeyIDs           []string
	EncryptedPct     string // e.g. "75.0" — used as CSS width and display value
	// Additional metrics exposed in the stats grid
	RangeBytesSaved    string
	KeyWrapOps         string
	KeyUnwrapOps       string
	RequestsInFlight   string
	ReplicationQueue   string
	ReplicationDropped string
	// Pagination fields
	NextToken          string
	ContinuationToken  string
	IsTruncated        bool
	// Dashboard credential for S3 operations
	DashboardCredential string
}

// Breadcrumb represents a navigation breadcrumb.
type Breadcrumb struct {
	Name string
	Path string
}

func (d *Dashboard) buildPageData(result *backend.ListResult, prefix string, continuationToken string) PageData {
	objects := make([]ObjectInfo, 0, len(result.CommonPrefixes)+len(result.Objects))
	var encCount, plainCount, folderCount int
	keyIDSet := make(map[string]struct{})

	// Add common prefixes (virtual folders) first so they appear at the top.
	for _, cp := range result.CommonPrefixes {
		folderCount++
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
		if info.IsFolder {
			folderCount++
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
			// ARMOR omits the metadata key ID for the default key. Keep the
			// dashboard's badge consistent with the JSON APIs by displaying the
			// effective key name in that case.
			info.KeyID = normalizedKeyID(info.KeyID)
			if !info.IsFolder {
				encCount++
				keyID := info.KeyID
				if keyID == "" {
					keyID = "default"
				}
				keyIDSet[keyID] = struct{}{}
			}
		} else {
			info.PlaintextSize = obj.Size
			info.PlaintextSizeH = formatBytes(obj.Size)
			if !info.IsFolder {
				plainCount++
			}
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

	// Build sorted key IDs list for the encryption coverage panel
	keyIDs := make([]string, 0, len(keyIDSet))
	for k := range keyIDSet {
		keyIDs = append(keyIDs, k)
	}
	sort.Strings(keyIDs)

	totalObjCount := encCount + plainCount
	encPct := "0"
	if totalObjCount > 0 {
		encPct = fmt.Sprintf("%.1f", float64(encCount)/float64(totalObjCount)*100)
	}

	canaryStatus, canaryCardClass := d.getCanaryStatus()
	return PageData{
		Bucket:             d.bucket,
		Prefix:             prefix,
		Objects:            objects,
		CacheHits:          cacheHits,
		CacheMisses:        cacheMisses,
		CacheHitRate:       hitRate,
		Uptime:             formatUptime(time.Since(d.metrics.StartTime())),
		Requests:           d.metrics.RequestsTotal.String(),
		BytesUp:            formatBytes(parseExpvarInt(d.metrics.BytesUploaded.String())),
		BytesDown:          formatBytes(parseExpvarInt(d.metrics.BytesDownloaded.String())),
		CanaryStatus:       canaryStatus,
		CanaryCardClass:    canaryCardClass,
		Breadcrumbs:        breadcrumbs,
		EncryptedCount:     encCount,
		PlaintextCount:     plainCount,
		TotalObjectCount:   totalObjCount,
		FolderCount:        folderCount,
		KeyIDs:             keyIDs,
		EncryptedPct:       encPct,
		RangeBytesSaved:    formatBytes(parseExpvarInt(d.metrics.RangeBytesSavedTotal.String())),
		KeyWrapOps:         d.metrics.KeyWrapOpsTotal.String(),
		KeyUnwrapOps:       d.metrics.KeyUnwrapOpsTotal.String(),
		RequestsInFlight:   d.metrics.RequestsInFlight.String(),
		ReplicationQueue:   d.metrics.ReplicationQueueDepth.String(),
		ReplicationDropped: d.metrics.ReplicationDroppedTotal.String(),
		// Pagination fields
		NextToken:         result.NextToken,
		ContinuationToken: continuationToken,
		IsTruncated:       result.IsTruncated,
			DashboardCredential: func() string {
				if d.dashboardCred != nil {
					return d.dashboardCred.Name
				}
				return ""
			}(),
	}
}

// getCanaryStatus returns (status string, CSS card class).
// CSS class is "healthy", "unhealthy", or "" for not-yet-started.
func (d *Dashboard) getCanaryStatus() (string, string) {
	// Use Value() to get the raw string; String() returns JSON-quoted form.
	lastCheck := d.metrics.CanaryLastCheckTime.Value()
	failures := parseExpvarInt(d.metrics.CanaryCheckFailures.String())

	if failures > 0 {
		errMsg := d.metrics.CanaryLastCheckError.Value()
		if errMsg != "" {
			return fmt.Sprintf("Unhealthy: %s", errMsg), "unhealthy"
		}
		return fmt.Sprintf("Unhealthy (failures: %d)", failures), "unhealthy"
	}
	if lastCheck == "" {
		return "Not started", ""
	}
	return fmt.Sprintf("Healthy (last: %s)", lastCheck), "healthy"
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
				keyID := normalizedKeyID(armorMeta.KeyID)
				detail["armor"] = map[string]interface{}{
					"plaintext_size": armorMeta.PlaintextSize,
					"block_size":     armorMeta.BlockSize,
					"key_id":         keyID,
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

// CredentialActivityHandler returns per-credential request activity.
// Combines credential list from /admin/creds with request counts from metrics.
func (d *Dashboard) CredentialActivityHandler(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return d.credentialActivityHandlerImpl(adminClient, adminURL)
}

// CredentialActivityHandlerWithAuth returns the credential activity handler with authentication.
func (d *Dashboard) CredentialActivityHandlerWithAuth(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return d.auth.Wrap(d.credentialActivityHandlerImpl(adminClient, adminURL))
}

// credentialActivityHandlerImpl is the actual implementation of the credential activity handler.
func (d *Dashboard) credentialActivityHandlerImpl(adminClient *http.Client, adminURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Fetch credentials from admin API
		credsURL := adminURL
		if credsURL == "" {
			credsURL = "http://127.0.0.1:9001/admin/creds"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, credsURL, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
			return
		}

		resp, err := adminClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to fetch credentials: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, fmt.Sprintf("Admin API returned status %d", resp.StatusCode), http.StatusBadGateway)
			return
		}

		var creds []struct {
			Name     string `json:"name"`
			ACLs     []struct {
				Prefix   string `json:"prefix"`
				ReadOnly bool   `json:"read_only"`
			} `json:"acls"`
			Source   string `json:"source"`
			LoadedAt string `json:"loaded_at"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode credentials: %v", err), http.StatusInternalServerError)
			return
		}

		// Build credential activity data
		type CredentialActivity struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			TotalReqs   int64   `json:"total_requests"`
			AllowCount  int64   `json:"allow_count"`
			DenyAuth    int64   `json:"deny_auth_count"`
			DenyACL     int64   `json:"deny_acl_count"`
		}

		activities := make([]CredentialActivity, 0, len(creds))

		// Collect credential names we know about
		credNames := make(map[string]bool, len(creds))
		for _, cred := range creds {
			credNames[cred.Name] = true
		}

		// Also include "unknown" for auth failures
		credNames["unknown"] = true

		// Aggregate metrics per credential
		for credName := range credNames {
			var totalReqs, allowCount, denyAuth, denyACL int64

			d.metrics.RequestsByCredentialTotal().Do(func(kv expvar.KeyValue) {
				// Parse the "access_key_id:verb:result" key format
				parts := strings.SplitN(kv.Key, ":", 3)
				if len(parts) == 3 {
					keyID := parts[0]
					result := parts[2]

					if keyID == credName {
						count := parseExpvarInt(kv.Value.String())
						totalReqs += count

						switch result {
						case "allow":
							allowCount += count
						case "deny-auth":
							denyAuth += count
						case "deny-acl":
							denyACL += count
						}
					}
				}
			})

			activity := CredentialActivity{
				Name:       credName,
				TotalReqs:  totalReqs,
				AllowCount: allowCount,
				DenyAuth:   denyAuth,
				DenyACL:    denyACL,
			}

			// Add source info for known credentials
			if credName != "unknown" {
				for _, cred := range creds {
					if cred.Name == credName {
						activity.Source = cred.Source
						break
					}
				}
			} else {
				activity.Source = "auth-failures"
			}

			activities = append(activities, activity)
		}

		// Sort by total requests descending, then by name
		sort.Slice(activities, func(i, j int) bool {
			if activities[i].TotalReqs != activities[j].TotalReqs {
				return activities[i].TotalReqs > activities[j].TotalReqs
			}
			return activities[i].Name < activities[j].Name
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(activities)
	}
}

// metricsHandlerImpl is the actual implementation of the metrics handler.
func (d *Dashboard) metricsHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := d.metrics
		cacheHits := parseExpvarInt(m.CacheHitsTotal.String())
		cacheMisses := parseExpvarInt(m.CacheMissesTotal.String())
		var cacheHitRatePct float64
		if cacheHits+cacheMisses > 0 {
			cacheHitRatePct = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
		}
		canaryStatus, canaryCardClass := d.getCanaryStatus()
		data := map[string]interface{}{
			"requests_total":          parseExpvarInt(m.RequestsTotal.String()),
			"requests_in_flight":      parseExpvarInt(m.RequestsInFlight.String()),
			"bytes_uploaded":          parseExpvarInt(m.BytesUploaded.String()),
			"bytes_downloaded":        parseExpvarInt(m.BytesDownloaded.String()),
			"bytes_fetched_from_b2":   parseExpvarInt(m.BytesFetchedFromB2.String()),
			"range_reads_total":       parseExpvarInt(m.RangeReadsTotal.String()),
			"range_bytes_saved":       parseExpvarInt(m.RangeBytesSavedTotal.String()),
			"cache_hits":              cacheHits,
			"cache_misses":            cacheMisses,
			"cache_hit_rate_pct":      cacheHitRatePct,
			"key_wrap_ops":            parseExpvarInt(m.KeyWrapOpsTotal.String()),
			"key_unwrap_ops":          parseExpvarInt(m.KeyUnwrapOpsTotal.String()),
			"canary_checks":           parseExpvarInt(m.CanaryChecksTotal.String()),
			"canary_failures":         parseExpvarInt(m.CanaryCheckFailures.String()),
			"canary_status":           canaryStatus,
			"canary_card_class":       canaryCardClass,
			"active_multipart":        parseExpvarInt(m.ActiveMultipartUploads.String()),
			"provenance_entries":      parseExpvarInt(m.ProvenanceEntriesTotal.String()),
			"replication_queue_depth": parseExpvarInt(m.ReplicationQueueDepth.String()),
			"replication_dropped":     parseExpvarInt(m.ReplicationDroppedTotal.String()),
			"replication_errors":      parseExpvarInt(m.ReplicationErrorsTotal.String()),
			"replication_retries":     parseExpvarInt(m.ReplicationRetriesTotal.String()),
			"uptime_seconds":          time.Since(m.StartTime()).Seconds(),
			"uptime_formatted":        formatUptime(time.Since(m.StartTime())),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

// KeyRotateHandler handles key rotation requests.
// It generates a new MEK and calls the admin API to perform rotation.
func (d *Dashboard) KeyRotateHandler(adminClient *http.Client, adminURL, adminToken string) http.HandlerFunc {
	return d.keyRotateHandlerImpl(adminClient, adminURL, adminToken)
}

// KeyRotateHandlerWithAuth handles key rotation requests with authentication.
func (d *Dashboard) KeyRotateHandlerWithAuth(adminClient *http.Client, adminURL, adminToken string) http.HandlerFunc {
	return d.auth.Wrap(d.keyRotateHandlerImpl(adminClient, adminURL, adminToken))
}

// keyRotateHandlerImpl is the actual implementation of the key rotation handler.
func (d *Dashboard) keyRotateHandlerImpl(adminClient *http.Client, adminURL, adminToken string) http.HandlerFunc {
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
		// Forward the admin bearer token so the loopback call passes the
		// /admin/key/rotate token gate (ARMOR_ADMIN_TOKEN). When unset, the
		// admin API rejects rotation (fail-closed); we send no header then.
		if adminToken != "" {
			req.Header.Set("Authorization", "Bearer "+adminToken)
		}

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

// EncryptionStatsResponse holds encryption statistics for a bucket listing.
type EncryptionStatsResponse struct {
	EncryptedCount  int            `json:"encrypted_count"`
	PlaintextCount  int            `json:"plaintext_count"`
	TotalCount      int            `json:"total_count"`
	EncryptedBytes  int64          `json:"encrypted_bytes"`
	PlaintextBytes  int64          `json:"plaintext_bytes"`
	KeyIDs          []string       `json:"key_ids"`
	KeyUsage        map[string]int `json:"key_usage"`
	CoveragePercent float64        `json:"coverage_percent"`
}

// EncryptionStatsHandler returns encryption statistics for the bucket.
func (d *Dashboard) EncryptionStatsHandler() http.HandlerFunc {
	return d.encryptionStatsHandlerImpl()
}

// EncryptionStatsHandlerWithAuth returns the encryption stats handler with authentication.
func (d *Dashboard) EncryptionStatsHandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.encryptionStatsHandlerImpl())
}

func (d *Dashboard) encryptionStatsHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Use no delimiter to list all objects (not just top-level) for accurate stats
		result, err := d.backend.List(ctx, d.bucket, prefix, "", "", 1000)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
			return
		}

		stats := EncryptionStatsResponse{
			KeyUsage: make(map[string]int),
		}

		for _, obj := range result.Objects {
			if strings.HasSuffix(obj.Key, "/") {
				continue
			}
			stats.TotalCount++
			if obj.IsARMOREncrypted {
				stats.EncryptedCount++
				stats.EncryptedBytes += obj.Size
				keyID := ""
				if armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata); ok {
					keyID = armorMeta.KeyID
				}
				keyID = normalizedKeyID(keyID)
				stats.KeyUsage[keyID]++
			} else {
				stats.PlaintextCount++
				stats.PlaintextBytes += obj.Size
			}
		}

		for k := range stats.KeyUsage {
			stats.KeyIDs = append(stats.KeyIDs, k)
		}
		sort.Strings(stats.KeyIDs)

		if stats.TotalCount > 0 {
			stats.CoveragePercent = float64(stats.EncryptedCount) / float64(stats.TotalCount) * 100
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// ListObject represents an object in the JSON list response.
type ListObject struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	LastModified string `json:"last_modified"`
	Encrypted    bool   `json:"encrypted"`
	KeyID        string `json:"key_id,omitempty"`
}

// ListAPIResponse holds the JSON response for the list endpoint.
type ListAPIResponse struct {
	Prefix             string       `json:"prefix"`
	Objects            []ListObject `json:"objects"`
	CommonPrefixes     []string     `json:"common_prefixes"`
	NextToken          string       `json:"next_token,omitempty"`
	ContinuationToken  string       `json:"continuation_token,omitempty"`
	IsTruncated        bool         `json:"is_truncated"`
}

// ListAPIHandler returns the JSON list handler.
func (d *Dashboard) ListAPIHandler() http.HandlerFunc {
	return d.listAPIHandlerImpl()
}

// ListAPIHandlerWithAuth returns the JSON list handler with authentication.
func (d *Dashboard) ListAPIHandlerWithAuth() http.HandlerFunc {
	return d.auth.Wrap(d.listAPIHandlerImpl())
}

// listAPIHandlerImpl is the actual implementation of the JSON list handler.
func (d *Dashboard) listAPIHandlerImpl() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		prefix := r.URL.Query().Get("prefix")
		continuationToken := r.URL.Query().Get("continuation_token")

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := d.backend.List(ctx, d.bucket, prefix, "/", continuationToken, 1000)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
			return
		}

		response := ListAPIResponse{
			Prefix:            prefix,
			Objects:           make([]ListObject, 0, len(result.Objects)),
			CommonPrefixes:    result.CommonPrefixes,
			NextToken:         result.NextToken,
			ContinuationToken: continuationToken,
			IsTruncated:       result.IsTruncated,
		}

		for _, obj := range result.Objects {
			listObj := ListObject{
				Key:          obj.Key,
				Size:         obj.Size,
				LastModified: obj.LastModified.Format(time.RFC3339),
				Encrypted:    obj.IsARMOREncrypted,
			}
			if obj.IsARMOREncrypted {
				if armorMeta, ok := backend.ParseARMORMetadata(obj.Metadata); ok {
					listObj.KeyID = normalizedKeyID(armorMeta.KeyID)
				}
				if listObj.KeyID == "" {
					listObj.KeyID = "default"
				}
			}
			response.Objects = append(response.Objects, listObj)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

	// UploadHandler handles file uploads through the dashboard using the dashboard credential.
	func (d *Dashboard) UploadHandler() http.HandlerFunc {
		return d.uploadHandlerImpl()
	}

	// UploadHandlerWithAuth returns the upload handler with dashboard authentication.
	func (d *Dashboard) UploadHandlerWithAuth() http.HandlerFunc {
		return d.auth.Wrap(d.uploadHandlerImpl())
	}

	// uploadHandlerImpl is the actual implementation of the upload handler.
	func (d *Dashboard) uploadHandlerImpl() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Check if dashboard credential is configured
			if d.dashboardCred == nil {
				http.Error(w, "Dashboard credential not configured - upload disabled", http.StatusForbidden)
				return
			}

			// Parse multipart form (max 100MB)
			if err := r.ParseMultipartForm(100 << 20); err != nil {
				http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
				return
			}

			// Get file from form
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get file from form: %v", err), http.StatusBadRequest)
				return
			}
			defer file.Close()

			// Get key from form (use filename if not provided)
			key := r.FormValue("key")
			if key == "" {
				key = header.Filename
			}

			// Get content type from form (use header if not provided)
			contentType := r.FormValue("content_type")
			if contentType == "" {
				contentType = header.Header.Get("Content-Type")
			}

			// We need to use the backend's Put method directly
			// This bypasses HTTP signing but uses the same backend logic
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			// Read file content
			fileData, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
				return
			}

			// Build metadata map from form values
			metadata := make(map[string]string)
			for k, v := range r.Form {
				if strings.HasPrefix(k, "x-amz-meta-") {
					if len(v) > 0 {
						metadata[k] = v[0]
					}
				}
			}

			// Use backend.Put to upload
			err = d.backend.Put(ctx, d.bucket, key, bytes.NewReader(fileData), int64(len(fileData)), metadata)
			if err != nil {
				http.Error(w, fmt.Sprintf("Upload failed: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"key":     key,
				"size":    len(fileData),
				"message": "File uploaded successfully",
			})
		}
	}

	// DownloadHandler handles file downloads through the dashboard using the dashboard credential.
	func (d *Dashboard) DownloadHandler() http.HandlerFunc {
		return d.downloadHandlerImpl()
	}

	// DownloadHandlerWithAuth returns the download handler with dashboard authentication.
	func (d *Dashboard) DownloadHandlerWithAuth() http.HandlerFunc {
		return d.auth.Wrap(d.downloadHandlerImpl())
	}

	// downloadHandlerImpl is the actual implementation of the download handler.
	func (d *Dashboard) downloadHandlerImpl() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Check if dashboard credential is configured
			if d.dashboardCred == nil {
				http.Error(w, "Dashboard credential not configured - download disabled", http.StatusForbidden)
				return
			}

			// Get key from query
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "key parameter required", http.StatusBadRequest)
				return
			}

			// Create S3 GET request signed with dashboard credential
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			// Use backend.Get to download
			reader, info, err := d.backend.Get(ctx, d.bucket, key)
			if err != nil {
				http.Error(w, fmt.Sprintf("Download failed: %v", err), http.StatusNotFound)
				return
			}
			defer reader.Close()

			// Set headers for download
			if info.ContentType != "" {
				w.Header().Set("Content-Type", info.ContentType)
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
			}
			w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", key))

			// Copy file to response
			_, err = io.Copy(w, reader)
			if err != nil {
				// Too late to send error code, headers already sent
				return
			}
		}
	}

	// DeleteHandler handles file deletion through the dashboard using the dashboard credential.
	func (d *Dashboard) DeleteHandler() http.HandlerFunc {
		return d.deleteHandlerImpl()
	}

	// DeleteHandlerWithAuth returns the delete handler with dashboard authentication.
	func (d *Dashboard) DeleteHandlerWithAuth() http.HandlerFunc {
		return d.auth.Wrap(d.deleteHandlerImpl())
	}

	// deleteHandlerImpl is the actual implementation of the delete handler.
	func (d *Dashboard) deleteHandlerImpl() http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete && r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Check if dashboard credential is configured
			if d.dashboardCred == nil {
				http.Error(w, "Dashboard credential not configured - delete disabled", http.StatusForbidden)
				return
			}

			// Get key from query (for DELETE) or form (for POST)
			var key string
			if r.Method == http.MethodDelete {
				key = r.URL.Query().Get("key")
			} else {
				key = r.FormValue("key")
			}

			if key == "" {
				http.Error(w, "key parameter required", http.StatusBadRequest)
				return
			}

			// Create S3 DELETE request signed with dashboard credential
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
			defer cancel()

			// Use backend.Delete to delete
			err := d.backend.Delete(ctx, d.bucket, key)
			if err != nil {
				http.Error(w, fmt.Sprintf("Delete failed: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "success",
				"key":     key,
				"message": "File deleted successfully",
			})
		}
	}
// Helper functions

func parseExpvarInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// normalizedKeyID returns the effective key name represented by ARMOR
// metadata. The default key is intentionally omitted from object metadata, so
// dashboard surfaces must render it explicitly rather than leaving the key
// name ambiguous.
func normalizedKeyID(keyID string) string {
	if keyID == "" {
		return "default"
	}
	return keyID
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
        .header-context { margin-top: 10px; font-size: 12px; color: #cbd5e1; }
        .header-context code { color: #fff; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
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
        .upload-btn {
            background: #10b981;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 6px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.2s;
            margin-left: 10px;
        }
        .upload-btn:hover { background: #059669; }
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
        .browser-toolbar {
            background: white;
            padding: 15px 20px;
            border-radius: 8px 8px 0 0;
            border-bottom: 1px solid #e2e8f0;
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 16px;
        }
        .browser-toolbar h2 { font-size: 18px; color: #1a1a2e; }
        .browser-toolbar p { color: #64748b; font-size: 13px; margin-top: 2px; }
        .browser-summary { color: #64748b; font-size: 13px; text-align: right; white-space: nowrap; }
        .browser-summary strong { color: #1e293b; }
        .refresh-link { color: #2563eb; margin-left: 12px; text-decoration: none; }
        .refresh-link:hover { text-decoration: underline; }
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
        .object-link {
            background: none;
            border: 0;
            color: #3b82f6;
            cursor: pointer;
            font: inherit;
            padding: 0;
            text-align: left;
        }
        .object-link:hover { text-decoration: underline; }
        .armor-badge {
            display: inline-block;
            background: #10b981;
            color: white;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
        }
        .plain-badge {
            display: inline-block;
            background: #f3f4f6;
            color: #4b5563;
            border: 1px solid #d1d5db;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
        }
        .folder-icon { margin-right: 5px; }
        .size-cell { font-family: monospace; color: #666; }
        .date-cell { color: #666; font-size: 13px; }
        .pagination {
            background: white;
            padding: 15px 20px;
            border-radius: 0 0 8px 8px;
            border-top: 1px solid #e2e8f0;
            display: flex;
            justify-content: space-between;
            align-items: center;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .pagination-info { color: #64748b; font-size: 13px; }
        .pagination-controls { display: flex; gap: 8px; }
        .pagination-btn {
            background: #f8fafc;
            color: #3b82f6;
            border: 1px solid #e2e8f0;
            padding: 8px 16px;
            border-radius: 6px;
            font-size: 13px;
            cursor: pointer;
            transition: background 0.2s;
        }
        .pagination-btn:hover:not(:disabled) { background: #e2e8f0; }
        .pagination-btn:disabled {
            color: #9ca3af;
            cursor: not-allowed;
            opacity: 0.5;
        }
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
        .encryption-panel {
            background: white;
            padding: 15px 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 15px;
        }
        .encryption-panel-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 8px;
        }
        .encryption-panel-title {
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
            color: #666;
        }
        .coverage-pct-label {
            font-size: 14px;
            font-weight: 600;
            color: #10b981;
        }
        .coverage-bar-track {
            width: 100%;
            height: 8px;
            background: #e5e7eb;
            border-radius: 4px;
            overflow: hidden;
            margin-bottom: 8px;
        }
        .coverage-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, #10b981, #059669);
            border-radius: 4px;
            transition: width 0.3s;
        }
        .coverage-legend {
            display: flex;
            gap: 15px;
            align-items: center;
            flex-wrap: wrap;
            font-size: 13px;
        }
        .legend-encrypted { color: #10b981; font-weight: 500; }
        .legend-plain { color: #9ca3af; }
        .legend-keys { color: #555; }
        .key-tag {
            display: inline-block;
            background: #eff6ff;
            color: #1d4ed8;
            border: 1px solid #bfdbfe;
            padding: 1px 7px;
            border-radius: 4px;
            font-size: 11px;
            font-weight: 500;
            margin-left: 4px;
        }
        .credential-panel {
            background: white;
            padding: 15px 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 15px;
        }
        .credential-panel-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
        }
        .credential-panel-title {
            font-size: 12px;
            font-weight: 600;
            text-transform: uppercase;
            color: #666;
        }
        .credential-table {
            width: 100%;
            border-collapse: collapse;
        }
        .credential-table th, .credential-table td {
            padding: 8px 12px;
            text-align: left;
            border-bottom: 1px solid #f1f5f9;
        }
        .credential-table th {
            font-size: 11px;
            text-transform: uppercase;
            color: #666;
            font-weight: 600;
        }
        .credential-table td {
            font-size: 13px;
        }
        .credential-table tr:last-child td {
            border-bottom: none;
        }
        .credential-name {
            font-family: monospace;
            font-weight: 500;
            color: #1a1a2e;
        }
        .credential-source {
            font-size: 12px;
            color: #64748b;
        }
        .credential-counts {
            font-family: monospace;
            color: #334155;
        }
        .empty-state { color: #64748b; padding: 42px 20px; text-align: center; }
        .empty-state strong { color: #334155; display: block; font-size: 16px; margin-bottom: 4px; }
        .metrics-updated { color: #94a3b8; font-size: 11px; margin-left: 8px; }
        .visually-hidden {
            clip: rect(0 0 0 0);
            clip-path: inset(50%);
            height: 1px;
            overflow: hidden;
            position: absolute;
            white-space: nowrap;
            width: 1px;
        }
        @media (max-width: 720px) {
            .container { padding: 10px; }
            header { align-items: flex-start; flex-direction: column; gap: 15px; }
            .browser-toolbar { align-items: flex-start; flex-direction: column; }
            .browser-summary { text-align: left; white-space: normal; }
            .objects-table { overflow-x: auto; }
            table { min-width: 680px; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div>
                <h1>ARMOR Dashboard</h1>
                <div class="subtitle">S3-compatible transparent encryption proxy</div>
                <div class="header-context">Bucket: <code>{{.Bucket}}</code></div>
            </div>
            <button class="rotate-btn" onclick="startKeyRotation()">Rotate Key</button>
        </header>
            {{if .DashboardCredential}}
            <button class="upload-btn" onclick="showUploadModal()">Upload File</button>
            {{end}}

        <div class="stats-grid">
            <div class="stat-card">
                <h3>Cache Hit Rate</h3>
                <div class="value" id="stat-cache-rate">{{.CacheHitRate}}</div>
            </div>
            <div class="stat-card">
                <h3>Cache Hits / Misses</h3>
                <div class="value" id="stat-cache-hits">{{.CacheHits}} / {{.CacheMisses}}</div>
            </div>
            <div class="stat-card">
                <h3>Total Requests</h3>
                <div class="value" id="stat-requests">{{.Requests}}</div>
            </div>
            <div class="stat-card">
                <h3>Bytes Uploaded</h3>
                <div class="value" id="stat-bytes-up">{{.BytesUp}}</div>
            </div>
            <div class="stat-card">
                <h3>Bytes Downloaded</h3>
                <div class="value" id="stat-bytes-down">{{.BytesDown}}</div>
            </div>
            <div class="stat-card">
                <h3>Uptime</h3>
                <div class="value" id="stat-uptime">{{.Uptime}}</div>
            </div>
            <div class="stat-card {{.CanaryCardClass}}" id="stat-canary-card">
                <h3>Canary Status</h3>
                <div class="value" id="stat-canary">{{.CanaryStatus}}</div>
            </div>
            <div class="stat-card">
                <h3>Encrypted Objects</h3>
                <div class="value" id="stat-encrypted">{{.EncryptedCount}} / {{.TotalObjectCount}}</div>
            </div>
            <div class="stat-card">
                <h3>Range Bytes Saved</h3>
                <div class="value" id="stat-range-saved">{{.RangeBytesSaved}}</div>
            </div>
            <div class="stat-card">
                <h3>Key Ops (W/U)</h3>
                <div class="value" id="stat-key-ops">{{.KeyWrapOps}} / {{.KeyUnwrapOps}}</div>
            </div>
            <div class="stat-card">
                <h3>Requests In Flight</h3>
                <div class="value" id="stat-in-flight">{{.RequestsInFlight}}</div>
            </div>
            <div class="stat-card">
                <h3>Replication Queue</h3>
                <div class="value" id="stat-replication-queue">{{.ReplicationQueue}}</div>
            </div>
            <div class="stat-card">
                <h3>Replication Dropped</h3>
                <div class="value" id="stat-replication-dropped">{{.ReplicationDropped}}</div>
            </div>
        </div>

        {{if gt .TotalObjectCount 0}}
        <div class="encryption-panel">
            <div class="encryption-panel-header">
                <span class="encryption-panel-title">Encryption Coverage</span>
                <span class="coverage-pct-label">{{.EncryptedPct}}%</span>
            </div>
            <div class="coverage-bar-track">
                <div class="coverage-bar-fill" style="width:{{.EncryptedPct}}%"></div>
            </div>
            <div class="coverage-legend">
                <span class="legend-encrypted">{{.EncryptedCount}} encrypted</span>
                <span class="legend-plain">{{.PlaintextCount}} plaintext</span>
                {{if .KeyIDs}}<span class="legend-keys">Keys: {{range .KeyIDs}}<span class="key-tag">{{.}}</span>{{end}}</span>{{end}}
            </div>
        </div>
        {{end}}

        <div class="credential-panel">
            <div class="credential-panel-header">
                <span class="credential-panel-title">Credential Activity (requests by access key)</span>
                <span style="font-size: 12px; color: #64748b;" id="credential-updated">Loading...</span>
            </div>
            <table class="credential-table">
                <thead>
                    <tr>
                        <th>Credential</th>
                        <th>Source</th>
                        <th>Total Requests</th>
                        <th>Allowed</th>
                        <th>Denied (Auth)</th>
                        <th>Denied (ACL)</th>
                    </tr>
                </thead>
                <tbody id="credential-table-body">
                    <tr>
                        <td colspan="6" style="text-align: center; color: #64748b; padding: 20px;">Loading credential activity...</td>
                    </tr>
                </tbody>
            </table>
        </div>

        <nav class="breadcrumbs" aria-label="Bucket path">
            {{range $i, $crumb := .Breadcrumbs}}{{if $i}}<span>›</span>{{end}}<a href="?prefix={{$crumb.Path}}">{{$crumb.Name}}</a>{{end}}
        </nav>

        <section aria-labelledby="browser-title">
        <div class="browser-toolbar">
            <div>
                <h2 id="browser-title">Bucket browser</h2>
                <p id="current-prefix">{{if .Prefix}}/{{.Prefix}}{{else}}Root prefix{{end}}</p>
            </div>
            <div class="browser-summary" aria-live="polite">
                <strong>{{.TotalObjectCount}}</strong> objects · <strong>{{.FolderCount}}</strong> folders
                <a class="refresh-link" href="?prefix={{.Prefix}}{{if .ContinuationToken}}&continuation_token={{.ContinuationToken}}{{end}}" aria-label="Refresh current prefix">Refresh</a>
            </div>
        </div>
        <div class="objects-table">
            <table>
                <caption class="visually-hidden">Objects in {{.Bucket}} under {{.Prefix}}</caption>
                <thead>
                    <tr>
                        <th>Key</th>
                        <th>Plaintext Size</th>
                        <th>Type</th>
                        <th>Last Modified</th>
                        <th>Encryption</th>
                        {{if .DashboardCredential}}
                        <th>Actions</th>
                        {{end}}
                    </tr>
                </thead>
                <tbody>
                    {{if not .Objects}}
                    <tr>
                        <td colspan="{{if .DashboardCredential}}6{{else}}5{{end}}">
                            <div class="empty-state">
                                <strong>This prefix is empty</strong>
                                There are no objects or folders to display here.
                            </div>
                        </td>
                    </tr>
                    {{end}}
                    {{range .Objects}}
                    <tr>
                        <td class="key-cell">
                            {{if .IsFolder}}
                                <span class="folder-icon">📁</span>
                                <a class="folder-link" href="?prefix={{.Key}}" aria-label="Open folder {{.Key}}">{{.Key}}</a>
                            {{else}}
                                <a class="object-link" href="#object-detail" data-object-key="{{.Key}}" onclick="showDetail(this.dataset.objectKey); return false;">{{.Key}}</a>
                            {{end}}
                        </td>
                        <td class="size-cell">{{.PlaintextSizeH}}</td>
                        <td>{{.ContentType}}</td>
                        <td class="date-cell">{{.LastModified}}</td>
                        <td>
                            {{if .IsARMOR}}
                                <span class="armor-badge" aria-label="ARMOR encrypted with key {{.KeyID}}">ARMOR [{{.KeyID}}]</span>
                            {{else if .IsFolder}}
                                —
                            {{else}}
                                <span class="plain-badge" aria-label="Unencrypted object">plain</span>
                            {{end}}
                        </td>
                        {{if .DashboardCredential}}
                        <td>
                            {{if not .IsFolder}}
                            <button class="btn btn-primary" onclick="downloadObject({{.Key}})" style="padding:4px 8px;font-size:12px;margin-right:4px">Download</button>
                            <button class="btn btn-secondary" onclick="deleteObject({{.Key}})" style="padding:4px 8px;font-size:12px;background:#ef4444;color:white">Delete</button>
                            {{else}}
                            —
                            {{end}}
                        </td>
                        {{end}}
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        <div class="pagination">
            <div class="pagination-info">
                Showing up to 1000 objects {{if .ContinuationToken}}(continuing from previous page){{end}}{{if .IsTruncated}} — more objects available{{end}}
            </div>
            <div class="pagination-controls">
                <button class="pagination-btn" onclick="navigatePrev()" {{if not .ContinuationToken}}disabled{{end}}>← Previous</button>
                <button class="pagination-btn" onclick="navigateNext()" {{if not .IsTruncated}}disabled{{end}}>Next →</button>
            </div>
        </div>
        </section>

        <footer>
            ARMOR — Transparent S3 Encryption • <a href="/metrics">Metrics</a> • <a href="/admin/key/verify">Key Status</a>
            <span class="metrics-updated" id="metrics-updated" aria-live="polite">Live metrics enabled</span>
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
    <div id="uploadModal" class="modal">
        <div class="modal-content" style="max-width:500px">
            <h2 class="modal-title">Upload File</h2>
            <div class="modal-body">
                <form id="uploadForm" enctype="multipart/form-data">
                    <div style="margin-bottom:15px">
                        <label style="display:block;margin-bottom:5px;font-weight:600">Select file:</label>
                        <input type="file" name="file" required style="width:100%">
                    </div>
                    <div style="margin-bottom:15px">
                        <label style="display:block;margin-bottom:5px;font-weight:600">Key (optional, defaults to filename):</label>
                        <input type="text" name="key" placeholder="path/to/file.txt" style="width:100%;padding:8px;border:1px solid #d1d5db;border-radius:4px">
                    </div>
                    <div style="margin-bottom:15px">
                        <label style="display:block;margin-bottom:5px;font-weight:600">Content-Type (optional):</label>
                        <input type="text" name="content_type" placeholder="application/octet-stream" style="width:100%;padding:8px;border:1px solid #d1d5db;border-radius:4px">
                    </div>
                    <div id="uploadStatus" class="status-message" style="display:none"></div>
                </form>
            </div>
            <div class="modal-buttons">
                <button class="btn btn-primary" onclick="submitUpload()">Upload</button>
                <button class="btn btn-secondary" onclick="closeUploadModal()">Cancel</button>
            </div>
        </div>
    </div>

    <script>
    // Pagination state
    const currentPrefix = {{if .Prefix}}'{{.Prefix}}'{{else}}''{{end}};
    const currentContinuationToken = {{if .ContinuationToken}}'{{.ContinuationToken}}'{{else}}null{{end}};
    const nextToken = {{if .NextToken}}'{{.NextToken}}'{{else}}null{{end}};

    function navigatePrev() {
        if (!currentContinuationToken) return;
        // To go back, we reload without a continuation token
        // This is a limitation of the forward-only token model
        window.location.href = '?prefix=' + encodeURIComponent(currentPrefix);
    }

    function navigateNext() {
        if (!nextToken) return;
        window.location.href = '?prefix=' + encodeURIComponent(currentPrefix) + '&continuation_token=' + encodeURIComponent(nextToken);
    }

    function row(label, value) {
        return '<div><strong style="color:#555;display:inline-block;width:160px">' + label + '</strong>' + escHtml(String(value)) + '</div>';
    }

    function escHtml(s) {
        return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
    }

    function fetchJSON(url, options) {
        return fetch(url, options).then(function(response) {
            if (!response.ok) {
                return response.text().then(function(body) {
                    throw new Error(body || ('Request failed (' + response.status + ')'));
                });
            }
            return response.json();
        });
    }

    function closeDetailModal() {
        document.getElementById('objectDetailModal').classList.remove('active');
    }

    function showDetail(key) {
        fetchJSON('/dashboard/object?key=' + encodeURIComponent(key))
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

    function showUploadModal() {
        document.getElementById('uploadModal').classList.add('active');
    }

    function closeUploadModal() {
        document.getElementById('uploadModal').classList.remove('active');
        document.getElementById('uploadForm').reset();
        document.getElementById('uploadStatus').style.display = 'none';
    }

    function submitUpload() {
        const form = document.getElementById('uploadForm');
        const formData = new FormData(form);
        const statusDiv = document.getElementById('uploadStatus');
        
        statusDiv.className = 'status-message status-info';
        statusDiv.textContent = 'Uploading...';
        statusDiv.style.display = 'block';
        
        fetch('/dashboard/upload', {
            method: 'POST',
            body: formData
        })
        .then(function(response) {
            if (!response.ok) {
                return response.text().then(function(body) {
                    throw new Error(body || 'Upload failed');
                });
            }
            return response.json();
        })
        .then(function(data) {
            statusDiv.className = 'status-message status-success';
            statusDiv.textContent = data.message || 'Upload successful!';
            setTimeout(function() {
                closeUploadModal();
                window.location.reload();
            }, 1500);
        })
        .catch(function(err) {
            statusDiv.className = 'status-message status-error';
            statusDiv.textContent = 'Error: ' + err.message;
        });
    }

    function downloadObject(key) {
        window.location.href = '/dashboard/download?key=' + encodeURIComponent(key);
    }

    function deleteObject(key) {
        if (!confirm('Are you sure you want to delete ' + key + '?')) return;
        
        fetch('/dashboard/delete?key=' + encodeURIComponent(key), {
            method: 'DELETE'
        })
        .then(function(response) {
            if (!response.ok) {
                return response.text().then(function(body) {
                    throw new Error(body || 'Delete failed');
                });
            }
            return response.json();
        })
        .then(function(data) {
            alert(data.message || 'Delete successful!');
            window.location.reload();
        })
        .catch(function(err) {
            alert('Delete error: ' + err.message);
        });
    }

    function fmtBytes(n) {
        n = parseInt(n, 10) || 0;
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n/1024).toFixed(1) + ' KB';
        if (n < 1073741824) return (n/1048576).toFixed(1) + ' MB';
        return (n/1073741824).toFixed(1) + ' GB';
    }

    function setText(id, val) {
        var el = document.getElementById(id);
        if (el) el.textContent = val;
    }

    function refreshMetrics() {
        fetchJSON('/dashboard/metrics')
            .then(function(data) {
                var hits = parseInt(data.cache_hits) || 0;
                var misses = parseInt(data.cache_misses) || 0;
                var total = hits + misses;
                setText('stat-cache-rate', total > 0 ? (hits/total*100).toFixed(1)+'%' : '0%');
                setText('stat-cache-hits', hits + ' / ' + misses);
                setText('stat-requests', data.requests_total || 0);
                setText('stat-bytes-up', fmtBytes(data.bytes_uploaded || 0));
                setText('stat-bytes-down', fmtBytes(data.bytes_downloaded || 0));
                if (data.uptime_formatted) setText('stat-uptime', data.uptime_formatted);
                setText('stat-range-saved', fmtBytes(data.range_bytes_saved || 0));
                setText('stat-key-ops', (data.key_wrap_ops || 0) + ' / ' + (data.key_unwrap_ops || 0));
                setText('stat-in-flight', data.requests_in_flight || 0);
                setText('stat-replication-queue', data.replication_queue_depth || 0);
                setText('stat-replication-dropped', data.replication_dropped || 0);
                if (data.canary_status) {
                    setText('stat-canary', data.canary_status);
                    var canaryCard = document.getElementById('stat-canary-card');
                    if (canaryCard) {
                        canaryCard.classList.remove('healthy', 'unhealthy');
                        if (data.canary_card_class) canaryCard.classList.add(data.canary_card_class);
                    }
                }
                var updated = document.getElementById('metrics-updated');
                if (updated) updated.textContent = 'Updated ' + new Date().toLocaleTimeString();
            })
            .catch(function(err) {
                var updated = document.getElementById('metrics-updated');
                if (updated) updated.textContent = 'Metrics unavailable';
                console.warn('Metrics refresh failed:', err);
            });
    }

    function refreshCredentialActivity() {
        fetchJSON('/dashboard/credential-activity')
            .then(function(data) {
                var tbody = document.getElementById('credential-table-body');
                if (!tbody) return;

                if (!data || data.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: #64748b; padding: 20px;">No credential data available</td></tr>';
                    return;
                }

                var html = '';
                data.forEach(function(cred) {
                    html += '<tr>';
                    html += '<td class="credential-name">' + escHtml(cred.name || 'unknown') + '</td>';
                    html += '<td class="credential-source">' + escHtml(cred.source || '—') + '</td>';
                    html += '<td class="credential-counts">' + (cred.total_requests || 0) + '</td>';
                    html += '<td class="credential-counts" style="color: #10b981;">' + (cred.allow_count || 0) + '</td>';
                    html += '<td class="credential-counts" style="color: #f59e0b;">' + (cred.deny_auth_count || 0) + '</td>';
                    html += '<td class="credential-counts" style="color: #ef4444;">' + (cred.deny_acl_count || 0) + '</td>';
                    html += '</tr>';
                });

                tbody.innerHTML = html;

                var updated = document.getElementById('credential-updated');
                if (updated) updated.textContent = 'Updated ' + new Date().toLocaleTimeString();
            })
            .catch(function(err) {
                var tbody = document.getElementById('credential-table-body');
                if (tbody) tbody.innerHTML = '<tr><td colspan="6" style="text-align: center; color: #ef4444; padding: 20px;">Failed to load credential activity</td></tr>';
                var updated = document.getElementById('credential-updated');
                if (updated) updated.textContent = 'Error loading data';
                console.warn('Credential activity refresh failed:', err);
            });
    }

    refreshMetrics();
    refreshCredentialActivity();
    // Auto-refresh metrics stats every 30 seconds without a full page reload
    setInterval(refreshMetrics, 30000);
    setInterval(refreshCredentialActivity, 30000);
    </script>
</body>
</html>
`
