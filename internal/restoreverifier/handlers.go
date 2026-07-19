// Package restoreverifier provides HTTP handlers for restore verification status and control.
package restoreverifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// StatusHandler returns the current verification status for all buckets.
func (v *Verifier) StatusHandler(metrics *metrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := v.GetStatus()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, fmt.Sprintf("Failed to encode status: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// BucketStatusHandler returns the verification status for a specific bucket.
func (v *Verifier) BucketStatusHandler(metrics *metrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		bucket := r.URL.Query().Get("bucket")
		if bucket == "" {
			http.Error(w, "bucket parameter required", http.StatusBadRequest)
			return
		}

		status, err := v.GetBucketStatus(bucket)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			http.Error(w, fmt.Sprintf("Failed to encode status: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// TriggerHandler triggers an immediate verification run. The optional ?mode=
// query selects which restore path the on-demand run exercises:
//
//	mode=dual     (default) both the ARMOR read path and the armor-decrypt
//	              direct path, asserting they agree (ModeDual).
//	mode=dr-drill direct-only DR drill: MEK unwrap + raw B2 fetch + decrypt +
//	              checksum/artifact assertion, with the ARMOR read path
//	              deliberately excluded (ModeDRDrill). This is the on-demand form
//	              of the periodic drill — prove recovery with the server gone.
//
// An unknown mode is rejected with 400 rather than silently treated as dual, so
// a typo in an automation job fails loudly instead of running the wrong path.
func (v *Verifier) TriggerHandler(metrics *metrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		mode := Mode(r.URL.Query().Get("mode"))
		if mode == "" {
			mode = ModeDual
		}
		switch mode {
		case ModeDual, ModeDRDrill:
		default:
			http.Error(w, fmt.Sprintf("unknown mode %q (want 'dual' or 'dr-drill')", mode), http.StatusBadRequest)
			return
		}

		// Trigger verification in background
		go func() {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			switch mode {
			case ModeDRDrill:
				v.runDRDrill(ctx)
			default:
				v.runVerification(ctx)
			}

			// Update metrics
			metrics.SetRestoreVerifierLastCheckTime(startTime)
			metrics.SetRestoreVerifierLastError("")

			// Record per-bucket dual-path check counters only for a dual run.
			// The drill publishes its own armor_drill_* gauges from verifyBucket
			// (recordDRDrillRun), so it must not bump the dual-path
			// checks/verified/failed counters — that would conflate a direct-only
			// result with the ARMOR read path's health.
			if mode == ModeDual {
				status := v.GetStatus()
				for bucketName, bucketState := range status {
					metrics.RecordRestoreVerifierCheck(bucketName, time.Since(startTime), bucketState.FailedObjects == 0)
				}
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		if mode == ModeDRDrill {
			w.Write([]byte("DR-drill (direct-only) triggered\n"))
		} else {
			w.Write([]byte("Verification triggered\n"))
		}
	}
}

// HealthHandler returns the health status of the restore verifier.
func (v *Verifier) HealthHandler(metrics *metrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := v.GetStatus()

		// Check if any bucket has failed verification or is stale
		healthy := true
		now := time.Now()
		for _, bucketState := range status {
			// Bucket is unhealthy if:
			// 1. Has failed objects
			// 2. Hasn't been verified in 24 hours
			if bucketState.FailedObjects > 0 {
				healthy = false
				break
			}
			if bucketState.LastVerification.IsZero() || now.Sub(bucketState.LastVerification) > 24*time.Hour {
				healthy = false
				break
			}
		}

		if healthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Unhealthy"))
		}
	}
}

// ReadyHandler returns the readiness status of the restore verifier.
func (v *Verifier) ReadyHandler(metrics *metrics.Metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// The verifier is ready if it has at least one successful verification
		status := v.GetStatus()
		ready := false

		for _, bucketState := range status {
			if !bucketState.LastSuccess.IsZero() {
				ready = true
				break
			}
		}

		if ready {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not ready"))
		}
	}
}
