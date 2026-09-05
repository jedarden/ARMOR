package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/dashboard"
)

// TestDashboardEndToEndWithARMORServer drives the dashboard through the
// production Server mux. The objects are created through ARMOR's authenticated
// S3 PUT path, then the admin dashboard is queried over HTTP using its real
// route wiring. A filesystem backend keeps this test deterministic and free of
// cloud credentials while still exercising the complete server instance.
func TestDashboardEndToEndWithARMORServer(t *testing.T) {
	const (
		bucket   = "dashboard-e2e"
		access   = "dashboard-access"
		secret   = "dashboard-secret"
		user     = "dashboard-admin"
		password = "dashboard-password"
	)

	basePath := t.TempDir()
	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: basePath})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	if err := fsBackend.CreateBucket(context.Background(), bucket); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}

	cfg := &config.Config{
		Bucket:    bucket,
		B2Region:  "us-east-005",
		BlockSize: 65536,
		MEK:       bytes.Repeat([]byte{0x11}, 32),
		NamedKeys: map[string][]byte{"archive": bytes.Repeat([]byte{0x22}, 32)},
		KeyRoutes: []config.KeyRoute{{Prefix: "archive/", KeyName: "archive"}},
		Credentials: map[string]*config.Credential{
			access: {AccessKey: access, SecretKey: secret},
		},
		DashboardUser: user,
		DashboardPass: password,
	}

	armorServer, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}
	armorServer.dashboard = dashboard.NewWithAuth(
		fsBackend, bucket, armorServer.metrics, user, password, "", nil, "", false)

	s3Server := httptest.NewServer(armorServer.Handler())
	adminServer := httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(func() {
		adminServer.Close()
		s3Server.Close()
	})

	dashboardSignedPut(t, s3Server.Client(), s3Server.URL, bucket, "public/report.txt", []byte("default-key"), access, secret)
	dashboardSignedPut(t, s3Server.Client(), s3Server.URL, bucket, "archive/report.txt", []byte("archive-key"), access, secret)
	if err := fsBackend.Put(context.Background(), bucket, "plain.txt", strings.NewReader("not encrypted"), int64(len("not encrypted")), map[string]string{
		"Content-Type": "text/plain",
	}); err != nil {
		t.Fatalf("create plaintext fixture: %v", err)
	}

	resp := dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		resp.Body.Close()
		t.Fatalf("unauthenticated dashboard status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard", user, password)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("authenticated dashboard status = %d, want 200: %s", resp.StatusCode, body)
	}
	html, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	htmlText := string(html)
	for _, want := range []string{
		"public/",
		"archive/",
		"plain.txt",
		`class="plain-badge"`,
		"Cache Hit Rate",
		"setInterval(refreshMetrics, 30000)",
	} {
		if !strings.Contains(htmlText, want) {
			t.Errorf("dashboard HTML missing %q", want)
		}
	}

	resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard/api/list", user, password)
	var list dashboard.ListAPIResponse
	decodeDashboardJSON(t, resp, &list)
	if len(list.Objects) != 1 {
		t.Fatalf("root list returned %d objects, want 1 plaintext object", len(list.Objects))
	}
	if len(list.CommonPrefixes) != 2 || !containsString(list.CommonPrefixes, "public/") || !containsString(list.CommonPrefixes, "archive/") {
		t.Fatalf("root list common prefixes = %v, want public/ and archive/", list.CommonPrefixes)
	}
	listByKey := make(map[string]dashboard.ListObject, len(list.Objects))
	for _, object := range list.Objects {
		listByKey[object.Key] = object
	}
	if object := listByKey["plain.txt"]; object.Encrypted || object.KeyID != "" {
		t.Errorf("plaintext object list entry = %+v, want unencrypted without key ID", object)
	}

	for _, fixture := range []struct {
		prefix string
		keyID  string
	}{
		{prefix: "public/", keyID: "default"},
		{prefix: "archive/", keyID: "archive"},
	} {
		resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard?prefix="+fixture.prefix, user, password)
		folderHTML, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(folderHTML), "ARMOR ["+fixture.keyID+"]") {
			t.Fatalf("prefix %q HTML missing ARMOR [%s]: status=%d body=%s", fixture.prefix, fixture.keyID, resp.StatusCode, folderHTML)
		}

		resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard/api/list?prefix="+fixture.prefix, user, password)
		var prefixedList dashboard.ListAPIResponse
		decodeDashboardJSON(t, resp, &prefixedList)
		if len(prefixedList.Objects) != 1 || !prefixedList.Objects[0].Encrypted || prefixedList.Objects[0].KeyID != fixture.keyID {
			t.Fatalf("prefix %q list = %+v, want one encrypted object with key %s", fixture.prefix, prefixedList.Objects, fixture.keyID)
		}
	}

	resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard/encryption-stats", user, password)
	var stats dashboard.EncryptionStatsResponse
	decodeDashboardJSON(t, resp, &stats)
	if stats.EncryptedCount != 2 || stats.PlaintextCount != 1 || stats.TotalCount != 3 {
		t.Fatalf("encryption stats = %+v, want 2 encrypted, 1 plaintext, 3 total", stats)
	}
	if stats.KeyUsage["default"] != 1 || stats.KeyUsage["archive"] != 1 {
		t.Fatalf("key usage = %v, want default=1 and archive=1", stats.KeyUsage)
	}

	resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard/object?key=archive/report.txt", user, password)
	var detail struct {
		Armor struct {
			KeyID string `json:"key_id"`
		} `json:"armor"`
	}
	decodeDashboardJSON(t, resp, &detail)
	if detail.Armor.KeyID != "archive" {
		t.Fatalf("object detail key_id = %q, want archive", detail.Armor.KeyID)
	}

	resp = dashboardRequest(t, adminServer.Client(), adminServer.URL+"/dashboard?prefix=archive/", user, password)
	prefixedHTML, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(prefixedHTML), "archive/report.txt") || strings.Contains(string(prefixedHTML), "public/report.txt") {
		t.Fatalf("prefixed dashboard did not show only archive objects: status=%d body=%s", resp.StatusCode, prefixedHTML)
	}
}

// TestDashboardFailClosedWhenUnconfigured drives the production Server mux with
// none of ARMOR_DASHBOARD_USER/PASS/TOKEN set — the posture the iad-ci
// deployment was found in (bead armor-cfd49e41). The dashboard is exempt from
// the admin bearer-token gate only while it authenticates requests itself, so
// an unconfigured mount must not answer: every dashboard route returns 401
// anonymous, 403 when no admin token is configured either, and never 200.
func TestDashboardFailClosedWhenUnconfigured(t *testing.T) {
	const (
		bucket = "dashboard-unconfigured"
		access = "unconfigured-access"
		secret = "unconfigured-secret"
	)

	newServer := func(t *testing.T, adminToken string) *httptest.Server {
		t.Helper()
		basePath := t.TempDir()
		fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: basePath})
		if err != nil {
			t.Fatalf("create filesystem backend: %v", err)
		}
		if err := fsBackend.CreateBucket(context.Background(), bucket); err != nil {
			t.Fatalf("create test bucket: %v", err)
		}
		cfg := &config.Config{
			Bucket:     bucket,
			B2Region:   "us-east-005",
			BlockSize:  65536,
			MEK:        bytes.Repeat([]byte{0x11}, 32),
			AdminToken: adminToken,
			Credentials: map[string]*config.Credential{
				access: {AccessKey: access, SecretKey: secret},
			},
			// DashboardUser, DashboardPass and DashboardToken deliberately unset.
		}
		armorServer, err := NewWithBackend(cfg, fsBackend)
		if err != nil {
			t.Fatalf("create ARMOR server: %v", err)
		}
		// NewWithBackend omits the dashboard, so wire it the way New() does when
		// ARMOR_DASHBOARD_USER/PASS/TOKEN are all unset — which is the exact
		// posture under test.
		armorServer.dashboard = dashboard.NewWithAuth(
			fsBackend, bucket, armorServer.metrics, "", "", "", nil, "", false)

		ts := httptest.NewServer(armorServer.AdminHandler())
		t.Cleanup(ts.Close)
		return ts
	}

	// Every route the dashboard serves, including the ones that proxy or
	// trigger privileged admin operations.
	routes := []string{
		"/dashboard",
		"/dashboard/",
		"/dashboard?prefix=archive/",
		"/dashboard/object?key=a.txt",
		"/dashboard/metrics",
		"/dashboard/encryption-stats",
		"/dashboard/api/list",
		"/dashboard/credential-activity",
		"/dashboard/upload",
		"/dashboard/download",
		"/dashboard/delete",
		"/dashboard/presign",
		"/dashboard/admin/key/status",
		"/dashboard/admin/key/rotate",
	}

	t.Run("no dashboard auth and no admin token", func(t *testing.T) {
		ts := newServer(t, "")
		client := ts.Client()
		for _, route := range routes {
			resp, err := client.Get(ts.URL + route)
			if err != nil {
				t.Fatalf("GET %s: %v", route, err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("GET %s anonymous = %d, want 403 (fail closed)", route, resp.StatusCode)
			}
		}
	})

	t.Run("no dashboard auth with an admin token", func(t *testing.T) {
		const adminToken = "test-admin-token"
		ts := newServer(t, adminToken)
		client := ts.Client()

		for _, route := range routes {
			// Anonymous is rejected.
			resp, err := client.Get(ts.URL + route)
			if err != nil {
				t.Fatalf("GET %s: %v", route, err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("GET %s anonymous = %d, want 401", route, resp.StatusCode)
			}

			// A wrong token is rejected too.
			req, err := http.NewRequest(http.MethodGet, ts.URL+route, nil)
			if err != nil {
				t.Fatalf("build GET %s: %v", route, err)
			}
			req.Header.Set("Authorization", "Bearer not-the-token")
			resp, err = client.Do(req)
			if err != nil {
				t.Fatalf("GET %s with wrong token: %v", route, err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("GET %s with wrong token = %d, want 401", route, resp.StatusCode)
			}

			// The admin bearer token passes the gate — the fallback that keeps
			// the mount operable for an administrator. Only the gate's verdict
			// is asserted: a 403 past this point is the handler's own decision
			// (download without ARMOR_DASHBOARD_CREDENTIAL, method not
			// allowed, …), not the gate refusing.
			req.Header.Set("Authorization", "Bearer "+adminToken)
			resp, err = client.Do(req)
			if err != nil {
				t.Fatalf("GET %s with admin token: %v", route, err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				t.Errorf("GET %s with admin bearer = 401, want the gate to admit a valid administrator", route)
			}
		}
	})
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func dashboardSignedPut(t *testing.T, client *http.Client, baseURL, bucket, key string, body []byte, accessKey, secretKey string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, baseURL+"/"+bucket+"/"+key, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create signed PUT request: %v", err)
	}
	signRequest(t, req, body, accessKey, secretKey, time.Now().UTC())
	req.Header.Set("Content-Type", "text/plain")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send signed PUT request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("signed PUT %s status = %d: %s", key, resp.StatusCode, responseBody)
	}
}

func dashboardRequest(t *testing.T, client *http.Client, url, user, password string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create dashboard request: %v", err)
	}
	if user != "" {
		req.SetBasicAuth(user, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send dashboard request: %v", err)
	}
	return resp
}

func decodeDashboardJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("dashboard JSON status = %d, want 200: %s", resp.StatusCode, body)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode dashboard JSON: %v", err)
	}
}
