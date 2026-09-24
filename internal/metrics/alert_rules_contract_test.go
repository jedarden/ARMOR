package metrics

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// alert_rules_contract_test.go pins the alert rules shipped alongside the
// canary and restore-verifier metric families (ADR-002 multipart visibility,
// ADR-004 restorability). The rules themselves live in
// jedarden/declarative-config as
// k8s/<cluster>/armor/restore-verifier-monitoring.yaml.disabled — the
// cluster-blind in-repo fixture this test parses is
// testdata/restore-verifier-monitoring.yaml, a verbatim copy of that
// manifest. A change to a shipped expression, hold duration, severity, or
// label must update the fixture, the expected table below, and
// docs/observability-contract.md in the same change.
//
// The human-readable contract is docs/observability-contract.md.

const alertFixturePath = "testdata/restore-verifier-monitoring.yaml"

// obsDocRelPath is docs/observability-contract.md relative to this package.
const obsDocRelPath = "../../docs/observability-contract.md"

// collapseWS reduces an expression to its token stream so YAML block scalars
// and single-line renderings compare equal regardless of formatting.
func collapseWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

type shippedAlert struct {
	Alert  string            `yaml:"alert"`
	Expr   string            `yaml:"expr"`
	For    string            `yaml:"for"`
	Labels map[string]string `yaml:"labels"`
}

type ruleGroup struct {
	Name     string         `yaml:"name"`
	Interval string         `yaml:"interval"`
	Rules    []shippedAlert `yaml:"rules"`
}

type monitoringDoc struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Groups []ruleGroup `yaml:"groups"`
		Selector struct {
			MatchExpressions []struct {
				Key      string   `yaml:"key"`
				Operator string   `yaml:"operator"`
				Values   []string `yaml:"values"`
			} `yaml:"matchExpressions"`
		} `yaml:"selector"`
		Endpoints []struct {
			Port     string `yaml:"port"`
			Path     string `yaml:"path"`
			Interval string `yaml:"interval"`
		} `yaml:"endpoints"`
	} `yaml:"spec"`
}

// loadMonitoringDocs decodes the multi-document fixture.
func loadMonitoringDocs(t *testing.T) []monitoringDoc {
	t.Helper()
	raw, err := os.ReadFile(alertFixturePath)
	if err != nil {
		t.Fatalf("read alert fixture: %v", err)
	}
	var docs []monitoringDoc
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var d monitoringDoc
		if err := dec.Decode(&d); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode alert fixture: %v", err)
		}
		if d.Kind == "" {
			continue // trailing document separator
		}
		docs = append(docs, d)
	}
	if len(docs) == 0 {
		t.Fatal("alert fixture decoded to zero documents")
	}
	return docs
}

// expectedShippedAlerts is the shipped rule set — the alerting contract. The
// multipart canary rule is ADR-002's instrumented blind spot: it keys on a
// gauge ARMOR exports, so a multipart-specific regression pages even while
// the small-object canary stays green.
var expectedShippedAlerts = []struct {
	name      string
	expr      string // whitespace-collapsed
	forStr    string
	component string
}{
	{
		name:      "ArmorRestoreVerificationStale",
		expr:      "time() - armor_last_verified_restore_timestamp > 12 * 3600",
		forStr:    "10m",
		component: "restore-verifier",
	},
	{
		name:      "ArmorRestoreVerificationFailures",
		expr:      "sum by (bucket) ( rate(armor_restore_verification_failures_total[1h]) ) > 0",
		forStr:    "5m",
		component: "restore-verifier",
	},
	{
		name:      "ArmorRestoreVerificationLowObjectRatio",
		expr:      "armor_verified_object_ratio < 0.95",
		forStr:    "5m",
		component: "restore-verifier",
	},
	{
		name:      "ArmorRestoreVerificationDualPathDivergence",
		expr:      "sum by (bucket) ( increase(armor_restore_verification_failures_total[1h]) ) > 0",
		forStr:    "5m",
		component: "restore-verifier",
	},
	{
		name:      "ArmorMultipartCanaryUnhealthy",
		expr:      "armor_multipart_canary_healthy == 0",
		forStr:    "10m",
		component: "armor-canary",
	},
}

// seriesRefRe matches the armor_-prefixed series a rule expression consumes.
var seriesRefRe = regexp.MustCompile(`armor_[a-z0-9_]+`)

// TestShippedAlertRulesContract pins the shipped PrometheusRule: one rule
// group, the exact alert set (no fewer, no extras), and per alert the
// expression, hold duration, and labels the published contract promises.
func TestShippedAlertRulesContract(t *testing.T) {
	docs := loadMonitoringDocs(t)

	var rule *monitoringDoc
	scrapeDocs := 0
	for i := range docs {
		switch docs[i].Kind {
		case "PrometheusRule":
			if rule != nil {
				t.Fatalf("fixture carries more than one PrometheusRule")
			}
			rule = &docs[i]
		case "ServiceMonitor":
			scrapeDocs++
		default:
			t.Errorf("unexpected fixture document kind %q", docs[i].Kind)
		}
	}
	if rule == nil {
		t.Fatal("fixture carries no PrometheusRule")
	}
	if rule.APIVersion != "monitoring.coreos.com/v1" {
		t.Errorf("PrometheusRule apiVersion = %q, want monitoring.coreos.com/v1", rule.APIVersion)
	}
	if rule.Metadata.Name != "restore-verifier-alerts" || rule.Metadata.Namespace != "armor" {
		t.Errorf("PrometheusRule metadata = %s/%s, want armor/restore-verifier-alerts",
			rule.Metadata.Namespace, rule.Metadata.Name)
	}
	if scrapeDocs != 2 {
		t.Errorf("fixture carries %d ServiceMonitor documents, want 2", scrapeDocs)
	}

	if len(rule.Spec.Groups) != 1 {
		t.Fatalf("PrometheusRule has %d groups, want 1", len(rule.Spec.Groups))
	}
	group := rule.Spec.Groups[0]
	if group.Name != "restore-verifier" {
		t.Errorf("rule group name = %q, want restore-verifier", group.Name)
	}
	if group.Interval != "30s" {
		t.Errorf("rule group evaluation interval = %q, want 30s", group.Interval)
	}

	byName := make(map[string]shippedAlert, len(group.Rules))
	for _, r := range group.Rules {
		if r.Alert == "" {
			t.Errorf("rule group carries a rule with no alert name (expr %q)", collapseWS(r.Expr))
			continue
		}
		if _, dup := byName[r.Alert]; dup {
			t.Errorf("alert %s appears more than once", r.Alert)
		}
		byName[r.Alert] = r
	}

	for _, want := range expectedShippedAlerts {
		got, ok := byName[want.name]
		if !ok {
			t.Errorf("shipped alert %s missing from the rule group", want.name)
			continue
		}
		delete(byName, want.name)
		if collapseWS(got.Expr) != want.expr {
			t.Errorf("alert %s expr = %q, want %q", want.name, collapseWS(got.Expr), want.expr)
		}
		d, err := time.ParseDuration(got.For)
		if err != nil {
			t.Errorf("alert %s for = %q is not a duration: %v", want.name, got.For, err)
		}
		wantD, _ := time.ParseDuration(want.forStr)
		if d != wantD {
			t.Errorf("alert %s for = %q, want %q", want.name, got.For, want.forStr)
		}
		if got.Labels["severity"] != "critical" {
			t.Errorf("alert %s severity = %q, want critical", want.name, got.Labels["severity"])
		}
		if got.Labels["component"] != want.component {
			t.Errorf("alert %s component = %q, want %q", want.name, got.Labels["component"], want.component)
		}
	}
	for name := range byName {
		t.Errorf("undocumented shipped alert %s; update expectedShippedAlerts and docs/observability-contract.md", name)
	}
}

// TestAlertRulesReferenceExportedSeries wires the alert contract to the
// metric contract: every armor_* series a shipped expression consumes must be
// a series this package actually exports. A rename here that misses the
// alert rules fails this test instead of silently disarming the alert.
func TestAlertRulesReferenceExportedSeries(t *testing.T) {
	docs := loadMonitoringDocs(t)

	exported := make(map[string]bool)
	for name := range canaryFamily {
		exported[name] = true
	}
	for name := range restoreVerifierFamily {
		exported[name] = true
	}

	var refs []string
	for _, d := range docs {
		if d.Kind != "PrometheusRule" {
			continue
		}
		for _, g := range d.Spec.Groups {
			for _, r := range g.Rules {
				refs = append(refs, seriesRefRe.FindAllString(r.Expr, -1)...)
			}
		}
	}
	if len(refs) == 0 {
		t.Fatal("no armor_* series referenced by any shipped alert")
	}
	for _, ref := range refs {
		if !exported[ref] {
			t.Errorf("alert expression references %s, which no contract family exports", ref)
		}
	}
}

// TestMultipartCanaryAlertContract pins ADR-002's alert in full: it keys on
// the multipart health gauge (never the small-object canary's status, which
// has no gauge), holds for 10m, and files as critical under the armor-canary
// component.
func TestMultipartCanaryAlertContract(t *testing.T) {
	if typ, ok := canaryFamily["armor_multipart_canary_healthy"]; !ok || typ != "gauge" {
		t.Fatalf("armor_multipart_canary_healthy = (%q, %v); the alert target must remain an exported gauge", typ, ok)
	}

	docs := loadMonitoringDocs(t)
	var found int
	for _, d := range docs {
		if d.Kind != "PrometheusRule" {
			continue
		}
		for _, g := range d.Spec.Groups {
			for _, r := range g.Rules {
				if r.Alert != "ArmorMultipartCanaryUnhealthy" {
					continue
				}
				found++
				if collapseWS(r.Expr) != "armor_multipart_canary_healthy == 0" {
					t.Errorf("ArmorMultipartCanaryUnhealthy expr = %q, want %q",
						collapseWS(r.Expr), "armor_multipart_canary_healthy == 0")
				}
				if got, want := r.For, "10m"; got != want {
					t.Errorf("ArmorMultipartCanaryUnhealthy for = %q, want %q", got, want)
				}
				if got := r.Labels["component"]; got != "armor-canary" {
					t.Errorf("ArmorMultipartCanaryUnhealthy component = %q, want armor-canary", got)
				}
			}
		}
	}
	if found != 1 {
		t.Errorf("ArmorMultipartCanaryUnhealthy appears %d times, want exactly 1", found)
	}
}

// TestShippedScrapeConfigContract pins the scrape perimeter the alerts
// depend on: two ServiceMonitors, both scraping /metrics at 30s — the
// restore-verifier's `metrics` port and ARMOR's `admin-api` port. The
// multipart canary gauge is served by ARMOR, so the armor ServiceMonitor is
// what makes ArmorMultipartCanaryUnhealthy evaluable at all.
func TestShippedScrapeConfigContract(t *testing.T) {
	docs := loadMonitoringDocs(t)
	seen := make(map[string]monitoringDoc)
	for _, d := range docs {
		if d.Kind != "ServiceMonitor" {
			continue
		}
		seen[d.Metadata.Name] = d
	}
	if len(seen) != 2 {
		names := make([]string, 0, len(seen))
		for k := range seen {
			names = append(names, k)
		}
		sort.Strings(names)
		t.Fatalf("fixture carries %d ServiceMonitors (%v), want 2", len(seen), names)
	}

	for _, name := range []string{"restore-verifier", "armor"} {
		d, ok := seen[name]
		if !ok {
			t.Errorf("ServiceMonitor %q missing from fixture", name)
			continue
		}
		if d.Metadata.Namespace != "armor" {
			t.Errorf("ServiceMonitor %s namespace = %q, want armor", name, d.Metadata.Namespace)
		}
		if len(d.Spec.Endpoints) != 1 {
			t.Errorf("ServiceMonitor %s has %d endpoints, want 1", name, len(d.Spec.Endpoints))
			continue
		}
		ep := d.Spec.Endpoints[0]
		if ep.Path != "/metrics" {
			t.Errorf("ServiceMonitor %s scrapes %q, want /metrics", name, ep.Path)
		}
		if ep.Interval != "30s" {
			t.Errorf("ServiceMonitor %s scrape interval = %q, want 30s", name, ep.Interval)
		}
	}

	armorSM, ok := seen["armor"]
	if !ok {
		return
	}
	if got := armorSM.Spec.Endpoints[0].Port; got != "admin-api" {
		t.Errorf("armor ServiceMonitor scrapes port %q, want admin-api (the multipart gauge is exported by ARMOR, not the verifier)", got)
	}
	if got := armorSM.Spec.Selector.MatchExpressions; len(got) != 0 {
		t.Errorf("armor ServiceMonitor uses matchExpressions; only the restore-verifier selector needs the multi-verifier In list")
	}
}

// TestObservabilityDocDocumentsShippedAlerts keeps the published document in
// lockstep with the fixture: every shipped alert is listed in
// docs/observability-contract.md with its series, and the multipart canary
// alert carries its exact expression and hold duration there.
func TestObservabilityDocDocumentsShippedAlerts(t *testing.T) {
	raw, err := os.ReadFile(obsDocRelPath)
	if err != nil {
		t.Fatalf("read %s: %v", obsDocRelPath, err)
	}
	doc := string(raw)

	for _, want := range expectedShippedAlerts {
		if !strings.Contains(doc, "| `"+want.name+"` |") {
			t.Errorf("docs/observability-contract.md alert table does not list %s", want.name)
		}
		for _, ref := range seriesRefRe.FindAllString(want.expr, -1) {
			if !strings.Contains(doc, ref) {
				t.Errorf("docs/observability-contract.md does not mention %s, referenced by shipped alert %s", ref, want.name)
			}
		}
	}
	if !strings.Contains(doc, "armor_multipart_canary_healthy == 0") {
		t.Error("docs/observability-contract.md does not pin ArmorMultipartCanaryUnhealthy's expression")
	}
	if !strings.Contains(doc, "alert_rules_contract_test.go") {
		t.Error("docs/observability-contract.md contract-test list does not reference alert_rules_contract_test.go")
	}
}
