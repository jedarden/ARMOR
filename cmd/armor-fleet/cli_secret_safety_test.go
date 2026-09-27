//go:build !integration
// +build !integration

// Secret-safe output smoke for the fleet console's documented contract.
// docs/companion-cli-reference.md documents SEAM_TOKEN as the console's
// credential: it authenticates every poll, and the failed-poll path records
// the poll error in both the log and fleet.json. These tests pin that the
// token value itself never appears in either place — a poll error that
// echoed its Authorization header would leak the console's SEAM credential
// into the container log and every /fleet.json consumer.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestFleetCLIReferenceTokenNeverReachesOutput runs the documented startup
// smoke with a distinctive SEAM token, lets the first poll fail against the
// unreachable SEAM host, and asserts the token appears nowhere in the
// process's stderr or the served fleet.json before a clean SIGTERM shutdown.
func TestFleetCLIReferenceTokenNeverReachesOutput(t *testing.T) {
	const token = "smoke-fake-token-4leakcheck"

	addr := fleetFreePort(t)
	targets := fleetSmokeTargets(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, fleetBinary(t),
		"-targets", targets,
		"-listen", addr,
		"-interval", "3600",
		"-seam-token", token,
	)
	cmd.Env = fleetReferenceEnv(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start armor-fleet: %v", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	// The listener binds only after the synchronous first poll; the poll
	// fails (fake token), which is the documented not-fatal path.
	base := "http://" + addr
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Get(base + "/fleet.json")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("armor-fleet never opened its HTTP listener on %s\nstderr so far:\n%s", addr, stderr.String())
		}
		time.Sleep(200 * time.Millisecond)
	}

	// fleet.json records the failed poll with its error; the token must not
	// be part of what the console serves.
	resp, err := http.Get(base + "/fleet.json")
	if err != nil {
		t.Fatalf("GET /fleet.json: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /fleet.json = %d, want 200", resp.StatusCode)
	}
	if strings.Contains(string(body), token) {
		t.Errorf("fleet.json carries the SEAM token value; the console must never echo credential material:\n%s", body)
	}

	// SIGTERM: the documented graceful shutdown, exit 0.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()
	select {
	case err := <-waitErr:
		stopped = true
		code := 0
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("wait for armor-fleet: %v", err)
		}
		if code != 0 {
			t.Errorf("armor-fleet after SIGTERM: exit = %d, want 0\nstderr:\n%s", code, stderr.String())
		}
	case <-time.After(45 * time.Second):
		t.Fatalf("armor-fleet did not shut down within 45s of SIGTERM\nstderr:\n%s", stderr.String())
	}

	if strings.Contains(stderr.String(), token) {
		t.Errorf("armor-fleet stderr carries the SEAM token value; the console must never echo credential material:\n%s", stderr.String())
	}
}
