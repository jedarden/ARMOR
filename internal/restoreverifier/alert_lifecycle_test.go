package restoreverifier

// This is the hermetic end-to-end alert lifecycle acceptance test. It keeps
// the real restore-verifier and metrics code in the loop, then uses HTTP test
// endpoints for the three external monitoring boundaries:
//
//   verifier -> Prometheus exposition -> vmalert -> Alertmanager -> ntfy
//
// The test endpoints implement only the wire behavior needed here. In
// particular, the vmalert double loads the shipped rule fixture and honors its
// expression and `for` hold, while the Alertmanager double fingerprints alert
// labels and suppresses repeated firing notifications. This makes the test
// deterministic and safe to run without monitoring credentials or a live
// notification topic.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/metrics"
	"gopkg.in/yaml.v3"
)

type lifecycleAlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type lifecycleRuleDoc struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Groups []struct {
			Name  string               `yaml:"name"`
			Rules []lifecycleAlertRule `yaml:"rules"`
		} `yaml:"groups"`
	} `yaml:"spec"`
}

// lifecycleAlert is the subset of the Alertmanager webhook contract needed by
// this test. vmalert sends an empty EndsAt while firing and a populated EndsAt
// when resolving an alert.
type lifecycleAlert struct {
	Status      string            `json:"status,omitempty"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

type lifecycleNtfyWebhook struct {
	Alerts []lifecycleAlert `json:"alerts"`
}

type lifecycleNtfy struct {
	mu     sync.Mutex
	alerts []lifecycleAlert
	bodies []string
	server *httptest.Server
}

func newLifecycleNtfy() *lifecycleNtfy {
	n := &lifecycleNtfy{}
	n.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload lifecycleNtfyWebhook
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid webhook", http.StatusBadRequest)
			return
		}
		body := ""
		if raw, err := json.Marshal(payload); err == nil {
			body = string(raw)
		}
		n.mu.Lock()
		n.alerts = append(n.alerts, payload.Alerts...)
		n.bodies = append(n.bodies, body)
		n.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	return n
}

func (n *lifecycleNtfy) close() {
	n.server.Close()
}

func (n *lifecycleNtfy) snapshot() ([]lifecycleAlert, []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	alerts := append([]lifecycleAlert(nil), n.alerts...)
	bodies := append([]string(nil), n.bodies...)
	return alerts, bodies
}

type lifecycleAlertmanager struct {
	mu        sync.Mutex
	ntfyURL   string
	server    *httptest.Server
	posts     int
	active    map[string]bool
	delivered []lifecycleAlert
}

func newLifecycleAlertmanager(ntfyURL string) *lifecycleAlertmanager {
	am := &lifecycleAlertmanager{
		ntfyURL: ntfyURL,
		active:  make(map[string]bool),
	}
	am.server = httptest.NewServer(http.HandlerFunc(am.handle))
	return am
}

func (am *lifecycleAlertmanager) close() {
	am.server.Close()
}

func (am *lifecycleAlertmanager) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var incoming []lifecycleAlert
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "invalid alert payload", http.StatusBadRequest)
		return
	}

	var deliver []lifecycleAlert
	am.mu.Lock()
	am.posts += len(incoming)
	for _, alert := range incoming {
		key := alertFingerprint(alert.Labels)
		resolved := !alert.EndsAt.IsZero()
		if resolved {
			if !am.active[key] {
				continue
			}
			delete(am.active, key)
			alert.Status = "resolved"
		} else {
			if am.active[key] {
				// Alertmanager accepts the repeat but does not re-notify ntfy.
				continue
			}
			am.active[key] = true
			alert.Status = "firing"
		}
		am.delivered = append(am.delivered, alert)
		deliver = append(deliver, alert)
	}
	am.mu.Unlock()

	if len(deliver) > 0 {
		payload := lifecycleNtfyWebhook{Alerts: deliver}
		body, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "encode ntfy payload", http.StatusInternalServerError)
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, am.ntfyURL, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "build ntfy request", http.StatusInternalServerError)
			return
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			http.Error(w, "deliver ntfy notification", http.StatusBadGateway)
			return
		}
		_ = response.Body.Close()
		if response.StatusCode/100 != 2 {
			http.Error(w, "ntfy rejected notification", http.StatusBadGateway)
			return
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (am *lifecycleAlertmanager) snapshot() (posts int, delivered []lifecycleAlert) {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.posts, append([]lifecycleAlert(nil), am.delivered...)
}

func alertFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(labels[key])
		b.WriteByte('\x00')
	}
	return b.String()
}

// lifecycleVmalert evaluates the shipped low-ratio rule and posts the same
// alert on every firing evaluation. Alertmanager, not vmalert, owns repeat
// notification suppression; that distinction is part of this lifecycle test.
type lifecycleVmalert struct {
	rule            lifecycleAlertRule
	hold            time.Duration
	alertmanagerURL string
	pendingSince    time.Time
	firing          bool
}

func newLifecycleVmalert(t *testing.T, alertmanagerURL string) *lifecycleVmalert {
	t.Helper()
	rule := loadLifecycleRule(t, "ArmorRestoreVerificationLowObjectRatio")
	if strings.Join(strings.Fields(rule.Expr), " ") != "armor_verified_object_ratio < 0.95" {
		t.Fatalf("shipped low-ratio expression = %q, want armor_verified_object_ratio < 0.95", rule.Expr)
	}
	hold, err := time.ParseDuration(rule.For)
	if err != nil {
		t.Fatalf("parse shipped low-ratio hold %q: %v", rule.For, err)
	}
	return &lifecycleVmalert{rule: rule, hold: hold, alertmanagerURL: alertmanagerURL}
}

func loadLifecycleRule(t *testing.T, name string) lifecycleAlertRule {
	t.Helper()
	path := filepath.Join("..", "metrics", "testdata", "restore-verifier-monitoring.yaml")
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open alert contract fixture: %v", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	for {
		var doc lifecycleRuleDoc
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode alert contract fixture: %v", err)
		}
		if doc.Kind != "PrometheusRule" {
			continue
		}
		for _, group := range doc.Spec.Groups {
			for _, rule := range group.Rules {
				if rule.Alert == name {
					return rule
				}
			}
		}
	}
	t.Fatalf("alert %s missing from shipped contract fixture", name)
	return lifecycleAlertRule{}
}

func (v *lifecycleVmalert) evaluate(t *testing.T, exposition, bucket string, at time.Time) {
	t.Helper()
	ratio := lifecycleMetricValue(t, exposition, "armor_verified_object_ratio", bucket)
	bad := ratio < 0.95
	if !bad {
		if v.firing {
			v.post(t, lifecycleAlert{
				Labels:      v.labels(bucket),
				Annotations: v.annotations(bucket, ratio),
				StartsAt:    v.pendingSince,
				EndsAt:      at,
			})
			v.firing = false
		}
		v.pendingSince = time.Time{}
		return
	}
	if v.pendingSince.IsZero() {
		v.pendingSince = at
	}
	if at.Sub(v.pendingSince) < v.hold {
		return
	}
	v.firing = true
	v.post(t, lifecycleAlert{
		Status:      "firing",
		Labels:      v.labels(bucket),
		Annotations: v.annotations(bucket, ratio),
		StartsAt:    v.pendingSince,
	})
}

func (v *lifecycleVmalert) labels(bucket string) map[string]string {
	labels := make(map[string]string, len(v.rule.Labels)+2)
	for key, value := range v.rule.Labels {
		labels[key] = value
	}
	labels["alertname"] = v.rule.Alert
	labels["bucket"] = bucket
	return labels
}

func (v *lifecycleVmalert) annotations(bucket string, ratio float64) map[string]string {
	annotations := make(map[string]string, len(v.rule.Annotations))
	for key, value := range v.rule.Annotations {
		value = strings.ReplaceAll(value, "{{ $labels.bucket }}", bucket)
		value = strings.ReplaceAll(value, "{{ $value | humanizePercentage }}", strconv.FormatFloat(ratio*100, 'f', 0, 64)+"%")
		annotations[key] = value
	}
	return annotations
}

func (v *lifecycleVmalert) post(t *testing.T, alert lifecycleAlert) {
	t.Helper()
	body, err := json.Marshal([]lifecycleAlert{alert})
	if err != nil {
		t.Fatalf("encode vmalert alert: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, v.alertmanagerURL+"/api/v2/alerts", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build Alertmanager request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post vmalert alert: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		t.Fatalf("Alertmanager status = %d, want 2xx", response.StatusCode)
	}
}

func lifecycleMetricValue(t *testing.T, exposition, name, bucket string) float64 {
	t.Helper()
	needle := name + "{"
	for _, line := range strings.Split(exposition, "\n") {
		if !strings.HasPrefix(line, needle) || !strings.Contains(line, `bucket="`+bucket+`"`) {
			continue
		}
		parts := strings.SplitN(line, "} ", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed Prometheus series %q", line)
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			t.Fatalf("parse Prometheus value in %q: %v", line, err)
		}
		return value
	}
	t.Fatalf("Prometheus series %s{bucket=%q} absent from exposition:\n%s", name, bucket, exposition)
	return 0
}

func newAlertLifecycleVerifier(t *testing.T, m *metrics.Metrics, escalator *Escalator) (*Verifier, *fakeBackend, string, string) {
	t.Helper()
	const (
		bucket    = "alert-lifecycle-bucket"
		key       = "backups/customer.sqlite"
		blockSize = 4096
	)
	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")
	ciphertext, metadata := armorEncrypt(t, mek, blockSize, plaintext)
	info := &backend.ObjectInfo{
		Key:          key,
		Size:         int64(len(plaintext)),
		LastModified: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC),
		Metadata:     metadata,
	}
	fb := &fakeBackend{
		ciphertext:  ciphertext,
		plaintext:   plaintext,
		info:        info,
		listObjects: []backend.ObjectInfo{*info},
	}
	v := New(fb, mek, nil, blockSize, nil, Config{
		Buckets:    []BucketConfig{{Bucket: bucket, Enabled: true}},
		Interval:   time.Hour,
		Metrics:    m,
		Escalator:  escalator,
		RunTimeout: time.Minute,
	})
	return v, fb, bucket, key
}

func TestRestoreVerifierFailureAlertLifecycleEndToEnd(t *testing.T) {
	metricsState := metrics.NewMetrics()
	filer := &recordingFiler{}
	escalator := NewEscalator(EscalatorConfig{
		Filer:      filer,
		Deployment: "restore-verifier-lifecycle",
		StatePath:  filepath.Join(t.TempDir(), "escalation-state.json"),
	})
	v, fb, bucket, objectKey := newAlertLifecycleVerifier(t, metricsState, escalator)

	ntfy := newLifecycleNtfy()
	t.Cleanup(ntfy.close)
	am := newLifecycleAlertmanager(ntfy.server.URL)
	t.Cleanup(am.close)
	vmalert := newLifecycleVmalert(t, am.server.URL)

	ctx := context.Background()
	v.runVerification(ctx)
	healthy, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("healthy status: %v", err)
	}
	if healthy.VerifiedObjectRatio != 1 || len(healthy.RecentResults) != 1 || healthy.RecentResults[0].Status != StatusPass {
		t.Fatalf("healthy baseline = ratio %.2f, results %#v; want one passing object", healthy.VerifiedObjectRatio, healthy.RecentResults)
	}
	if got := filer.count(); got != 0 {
		t.Fatalf("healthy baseline filed %d escalation beads, want 0", got)
	}

	// Corrupt the stored ciphertext. The real verifier must fail before it can
	// claim a successful restore, publish the failure gauges, and file exactly
	// one escalation with the normal ARMOR path named as the failing path.
	fb.corrupt.Store(true)
	v.runVerification(ctx)
	failed, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("failed status: %v", err)
	}
	if len(failed.RecentResults) < 2 || failed.RecentResults[len(failed.RecentResults)-1].Status != StatusRestoreError || failed.RecentResults[len(failed.RecentResults)-1].Path != PathARMOR {
		t.Fatalf("latest failure result = %#v; want restore_error on armor path", failed.RecentResults[len(failed.RecentResults)-1])
	}
	if failed.VerifiedObjectRatio != 0 || failed.LastSuccess.IsZero() {
		t.Fatalf("failure state ratio=%v last_success=%v; want ratio 0 and last proven success retained", failed.VerifiedObjectRatio, failed.LastSuccess)
	}

	exposition := metricsState.PrometheusFormat()
	if got := lifecycleMetricValue(t, exposition, "armor_verified_object_ratio", bucket); got != 0 {
		t.Errorf("failure ratio metric = %v, want 0", got)
	}
	if got := lifecycleMetricValue(t, exposition, "armor_restore_verification_failures_total", bucket); got != 1 {
		t.Errorf("failure counter metric = %v, want 1", got)
	}
	if got := lifecycleMetricValue(t, exposition, "armor_last_verified_restore_timestamp", bucket); got == 0 {
		t.Error("last verified restore metric reset to zero after failure; it must retain the last proven success")
	}

	if got := filer.count(); got != 1 {
		t.Fatalf("first failed run filed %d escalation beads, want 1", got)
	}
	filer.mu.Lock()
	payload := filer.filed[0]
	filer.mu.Unlock()
	body := payload.Body()
	for _, want := range []string{
		"- **Bucket:** " + bucket,
		"- **Object key:** " + objectKey,
		"- **Deployment:** restore-verifier-lifecycle",
		"- **Path:** armor",
		"- **Failure class:** restore_error",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("escalation body missing documented evidence %q:\n%s", want, body)
		}
	}
	// The payload contains evidence and digests, never the verifier's key
	// material. Keep this assertion explicit so future evidence additions do
	// not accidentally turn a bead into a credential-bearing artifact.
	secretMarker := "restore-verifier-test-secret-must-not-leak"
	wrappedDEK := fb.info.Metadata["x-amz-meta-armor-wrapped-dek"]
	for name, text := range map[string]string{
		"title": payload.Title(),
		"body":  body,
	} {
		if strings.Contains(text, secretMarker) || strings.Contains(text, wrappedDEK) {
			t.Errorf("escalation %s contains secret marker", name)
		}
	}

	// vmalert sees the low-ratio metric, waits for the shipped five-minute
	// hold, and then sends the alert through the HTTP Alertmanager boundary.
	base := time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC)
	vmalert.evaluate(t, exposition, bucket, base)
	vmalert.evaluate(t, exposition, bucket, base.Add(4*time.Minute))
	if alerts, _ := ntfy.snapshot(); len(alerts) != 0 {
		t.Fatalf("alert fired before the shipped for=%s hold: %#v", vmalert.rule.For, alerts)
	}
	vmalert.evaluate(t, exposition, bucket, base.Add(5*time.Minute))
	// A second evaluation simulates vmalert's next tick. Alertmanager receives
	// it, but its stable label fingerprint must prevent a duplicate ntfy page.
	vmalert.evaluate(t, exposition, bucket, base.Add(6*time.Minute))
	posts, delivered := am.snapshot()
	if posts != 2 {
		t.Fatalf("Alertmanager received %d vmalert alert payload(s), want 2 evaluations", posts)
	}
	if len(delivered) != 1 || delivered[0].Status != "firing" {
		t.Fatalf("Alertmanager delivered %#v, want one firing notification after deduplication", delivered)
	}
	if alerts, bodies := ntfy.snapshot(); len(alerts) != 1 || alerts[0].Status != "firing" {
		t.Fatalf("ntfy received %#v, want one firing notification; bodies=%v", alerts, bodies)
	} else {
		for _, body := range bodies {
			if strings.Contains(body, wrappedDEK) || strings.Contains(body, secretMarker) {
				t.Fatal("ntfy notification contains verifier key material")
			}
		}
	}

	// Repair the object. A real passing verification clears the active bead
	// key, changes the ratio back above the rule threshold, and resolves the
	// same alert fingerprint through Alertmanager to ntfy.
	fb.corrupt.Store(false)
	v.runVerification(ctx)
	recovered, err := v.GetBucketStatus(bucket)
	if err != nil {
		t.Fatalf("recovered status: %v", err)
	}
	if recovered.VerifiedObjectRatio != 1 || recovered.RecentResults[len(recovered.RecentResults)-1].Status != StatusPass {
		t.Fatalf("recovered state = ratio %.2f latest result %#v; want pass and ratio 1", recovered.VerifiedObjectRatio, recovered.RecentResults[len(recovered.RecentResults)-1])
	}
	if got := filer.count(); got != 1 {
		t.Fatalf("recovery changed escalation count to %d, want the original one active bead", got)
	}
	recoveredExposition := metricsState.PrometheusFormat()
	vmalert.evaluate(t, recoveredExposition, bucket, base.Add(7*time.Minute))
	posts, delivered = am.snapshot()
	if posts != 3 || len(delivered) != 2 || delivered[1].Status != "resolved" {
		t.Fatalf("recovery notifications: posts=%d delivered=%#v; want one resolved delivery", posts, delivered)
	}
	if alerts, bodies := ntfy.snapshot(); len(alerts) != 2 || alerts[1].Status != "resolved" {
		t.Fatalf("ntfy recovery = %#v; want firing then resolved, bodies=%v", alerts, bodies)
	} else {
		for _, body := range bodies {
			if strings.Contains(body, wrappedDEK) || strings.Contains(body, secretMarker) {
				t.Fatal("recovery notification contains verifier key material")
			}
		}
	}

	// The same object failing again after recovery is a new active failure: the
	// escalation key was cleared on the pass, so it files once more rather than
	// suppressing a genuine regression.
	fb.corrupt.Store(true)
	v.runVerification(ctx)
	if got := filer.count(); got != 2 {
		t.Fatalf("post-recovery regression filed %d beads, want exactly 2 total", got)
	}
	filer.mu.Lock()
	secondPayload := filer.filed[1]
	filer.mu.Unlock()
	if secondPayload.ObjectKey != objectKey || secondPayload.Bucket != bucket || secondPayload.Path != PathARMOR {
		t.Fatalf("post-recovery escalation lost evidence: %#v", secondPayload)
	}
	if strings.Contains(secondPayload.Body(), secretMarker) || strings.Contains(secondPayload.Body(), wrappedDEK) {
		t.Fatal("post-recovery escalation contains secret marker")
	}
}
