// Package main provides fleet monitoring for armor-fleet.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// FleetMonitor polls ARMOR instances through SEAM and aggregates status.
type FleetMonitor struct {
	targets      []Target
	seamToken    string
	pollInterval int
	mu           sync.RWMutex
	status       map[string]*TargetStatus
	client       *http.Client
}

// TargetStatus represents the status of a single ARMOR instance.
type TargetStatus struct {
	Name             string            `json:"name"`
	Cluster          string            `json:"cluster"`
	Namespace        string            `json:"namespace"`
	Service          string            `json:"service"`
	Version          string            `json:"version"`
	CanaryHealthy    bool              `json:"canary_healthy"`
	CanaryMessage    string            `json:"canary_message"`
	MultipartCanary  bool              `json:"multipart_canary"`
	RestoreVerifierGauges map[string]string `json:"restore_verifier_gauges,omitempty"`
	ErrorRate        float64           `json:"error_rate"`
	LastSeen         time.Time         `json:"last_seen"`
	Reachable        bool              `json:"reachable"`
	Error            string            `json:"error,omitempty"`
}

// Prometheus metrics
var (
	fleetUpTargets = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "armor_fleet_up_targets",
		Help: "Number of reachable ARMOR targets",
	})
	fleetDownTargets = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "armor_fleet_down_targets",
		Help: "Number of unreachable ARMOR targets",
	})
)

// NewFleetMonitor creates a new fleet monitor.
func NewFleetMonitor(targets []Target, seamToken string, pollInterval int) *FleetMonitor {
	return &FleetMonitor{
		targets:      targets,
		seamToken:    seamToken,
		pollInterval: pollInterval,
		status:       make(map[string]*TargetStatus),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start begins polling targets in the background.
func (fm *FleetMonitor) Start() {
	// Initial poll
	fm.pollAll()

	// Background polling
	go func() {
		ticker := time.NewTicker(time.Duration(fm.pollInterval) * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fm.pollAll()
		}
	}()
}

// GetStatus returns a snapshot of all target statuses.
func (fm *FleetMonitor) GetStatus() map[string]*TargetStatus {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make(map[string]*TargetStatus, len(fm.status))
	for k, v := range fm.status {
		result[k] = v
	}
	return result
}

// pollAll polls all targets and updates status.
func (fm *FleetMonitor) pollAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var upCount, downCount float64

	for _, target := range fm.Targets() {
		status := fm.pollTarget(ctx, target)
		fm.mu.Lock()
		fm.status[target.Name] = status
		fm.mu.Unlock()

		if status.Reachable {
			upCount++
		} else {
			downCount++
		}
	}

	fleetUpTargets.Set(upCount)
	fleetDownTargets.Set(downCount)
}

// pollTarget polls a single target's endpoints.
func (fm *FleetMonitor) pollTarget(ctx context.Context, target Target) *TargetStatus {
	status := &TargetStatus{
		Name:      target.Name,
		Cluster:   target.Cluster,
		Namespace: target.Namespace,
		Service:   target.Service,
		LastSeen:  time.Now(),
	}

	// Build SEAM proxy base URL
	baseURL := fmt.Sprintf("https://seam-rs-manager.tail1b1987.ts.net/k8s/%s/api/v1/namespaces/%s/services/%s:%d/proxy",
		target.Cluster, target.Namespace, target.Service, target.AdminPort)

	// Poll /version
	if err := fm.pollVersion(ctx, baseURL, status); err != nil {
		status.Reachable = false
		status.Error = fmt.Sprintf("version check failed: %v", err)
		return status
	}

	// Poll /armor/canary
	if err := fm.pollCanary(ctx, baseURL, status); err != nil {
		status.Reachable = false
		status.Error = fmt.Sprintf("canary check failed: %v", err)
		return status
	}

	// Poll /metrics
	if err := fm.pollMetrics(ctx, baseURL, status); err != nil {
		status.Reachable = false
		status.Error = fmt.Sprintf("metrics check failed: %v", err)
		return status
	}

	status.Reachable = true
	return status
}

// pollVersion fetches version information.
func (fm *FleetMonitor) pollVersion(ctx context.Context, baseURL string, status *TargetStatus) error {
	url := baseURL + "/version"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+fm.seamToken)

	resp, err := fm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var versionInfo struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
		return err
	}

	status.Version = versionInfo.Version
	return nil
}

// pollCanary fetches canary status.
func (fm *FleetMonitor) pollCanary(ctx context.Context, baseURL string, status *TargetStatus) error {
	url := baseURL + "/armor/canary"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+fm.seamToken)

	resp, err := fm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	var canaryInfo struct {
		Healthy         bool   `json:"healthy"`
		Message         string `json:"message"`
		MultipartCanary bool   `json:"multipart_canary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&canaryInfo); err != nil {
		return err
	}

	status.CanaryHealthy = canaryInfo.Healthy
	status.CanaryMessage = canaryInfo.Message
	status.MultipartCanary = canaryInfo.MultipartCanary
	return nil
}

// pollMetrics fetches and parses Prometheus metrics.
func (fm *FleetMonitor) pollMetrics(ctx context.Context, baseURL string, status *TargetStatus) error {
	url := baseURL + "/metrics"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+fm.seamToken)

	resp, err := fm.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	// Parse metrics line by line
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse armor_errors_total for error rate
		if strings.HasPrefix(line, "armor_errors_total") {
			status.ErrorRate = parseMetricValue(line)
		}

		// Parse restore-verifier gauges
		if strings.HasPrefix(line, "armor_restore_verifier_") {
			name, value := parseGaugeMetric(line)
			if name != "" {
				if status.RestoreVerifierGauges == nil {
					status.RestoreVerifierGauges = make(map[string]string)
				}
				status.RestoreVerifierGauges[name] = value
			}
		}
	}

	return nil
}

// Targets returns the list of monitored targets.
func (fm *FleetMonitor) Targets() []Target {
	return fm.targets
}

// parseMetricValue extracts the numeric value from a Prometheus metric line.
func parseMetricValue(line string) float64 {
	// Format: metric_name{labels} value
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	value, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return 0
	}
	return value
}

// parseGaugeMetric extracts the gauge name and value from a restore-verifier metric.
func parseGaugeMetric(line string) (name, value string) {
	// Format: armor_restore_verifier_{name}{labels} gauge value
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return "", ""
	}

	// Extract metric name (first part before any labels)
	metricPart := parts[0]
	if !strings.HasPrefix(metricPart, "armor_restore_verifier_") {
		return "", ""
	}

	// Get the part after the prefix
	name = strings.TrimPrefix(metricPart, "armor_restore_verifier_")
	if idx := strings.Index(name, "{"); idx >= 0 {
		name = name[:idx]
	}

	value = parts[len(parts)-1]
	return name, value
}

