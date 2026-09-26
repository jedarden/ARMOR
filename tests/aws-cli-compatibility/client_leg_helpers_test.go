// Shared gates and helpers for the client compatibility legs.
//
// The suite started with the aws/rclone CLI legs (requireAWSCLI /
// requireRclone in harness_test.go); litestream, barman-cloud, DuckDB and
// boto3 follow the identical contract, so their binary/module presence gates
// live here as one generic pair:
//
//   - outside -short with the client absent → clean skip, so `go test ./...`
//     stays green on machines without the tool;
//   - under ARMOR_COMPAT_ENDPOINT (the per-release armor-build gate) with the
//     client absent → fatal, because the gate installs every client before
//     running: a missing binary there is a broken gate, not a spare environment.
//
// Every leg uses only synthetic, ephemeral credentials minted by the harness
// (or the compat server's own demo pair in endpoint mode). Real deployments
// keep their ARMOR client credentials in OpenBao and deliver them by
// reference; nothing here ever reads one.
package awsclicompat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// requireClientBin is the generic presence gate for the litestream,
// barman-cloud and DuckDB legs (see the package comment for the skip/fatal
// split). installHint is surfaced in the skip/fatal message.
func requireClientBin(t *testing.T, name, installHint string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("skipping %s compatibility test in -short mode", name)
	}
	if _, err := exec.LookPath(name); err != nil {
		if isCompatEndpointMode() {
			t.Fatalf("%s not installed on PATH but required in ARMOR_COMPAT_ENDPOINT mode (%s)", name, installHint)
		}
		t.Skipf("%s not installed on PATH — skipping compatibility test (%s)", name, installHint)
	}
}

// requirePythonModule gates the boto3 leg on a python3 that can import the
// given module, with the same skip/fatal split as requireClientBin.
func requirePythonModule(t *testing.T, module, installHint string) {
	t.Helper()
	if testing.Short() {
		t.Skipf("skipping %s compatibility test in -short mode", module)
	}
	if _, err := exec.LookPath("python3"); err != nil {
		if isCompatEndpointMode() {
			t.Fatalf("python3 not installed on PATH but required in ARMOR_COMPAT_ENDPOINT mode")
		}
		t.Skip("python3 not installed on PATH — skipping compatibility test")
	}
	if out, err := run(t, "python3", nil, "-c", "import "+module); err != nil {
		if isCompatEndpointMode() {
			t.Fatalf("python3 cannot import %s but it is required in ARMOR_COMPAT_ENDPOINT mode (%s): %v\n%s",
				module, installHint, err, out)
		}
		t.Skipf("python3 cannot import %s — skipping compatibility test (%s)", module, installHint)
	}
}

// compatCredentials returns the synthetic in-process pair or the caller's
// endpoint-mode pair. Callers pass it through a process environment or a
// mode-0600 temporary client config; it is never put in a command argument.
func compatCredentials(t *testing.T) (accessKey, secretKey string) {
	t.Helper()
	accessKey, secretKey = testAccessKey, testSecretKey
	if isCompatEndpointMode() {
		_, accessKey, secretKey = compatEndpointConfig()
		if accessKey == "" || secretKey == "" {
			t.Fatalf("ARMOR_COMPAT_ENDPOINT requires ARMOR_COMPAT_ACCESS_KEY and ARMOR_COMPAT_SECRET_KEY")
		}
	}
	return accessKey, secretKey
}

// renderFixture loads a checked-in client configuration template and replaces
// only the explicitly supplied placeholders.  The templates intentionally
// contain no credentials; the rendered copy lives in t.TempDir and receives
// the ephemeral pair from the harness (or the endpoint-mode environment).
func renderFixture(t *testing.T, name string, replacements map[string]string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read client fixture %s: %v", name, err)
	}
	rendered := string(body)
	for key, value := range replacements {
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", value)
	}
	if strings.Contains(rendered, "{{") {
		t.Fatalf("client fixture %s has an unresolved placeholder", name)
	}
	return []byte(rendered)
}

// skipOrFailEnv returns the value of an environment variable, or — when it is
// unset — skips (outside endpoint mode) or fatals (in endpoint mode) with
// reason. The barman leg uses it for the PostgreSQL connection info: the leg
// needs a reachable PostgreSQL to back up, and only the caller's environment
// knows where one lives.
func skipOrFailEnv(t *testing.T, name, reason string) string {
	t.Helper()
	v := os.Getenv(name)
	if v != "" {
		return v
	}
	if isCompatEndpointMode() {
		t.Fatalf("%s is required in ARMOR_COMPAT_ENDPOINT mode: %s", name, reason)
	}
	t.Skipf("%s not set — skipping compatibility test (%s)", name, reason)
	return ""
}

// daemon is a long-running client process (litestream replicate) started in
// the background, with its output captured to a file the test can inspect.
type daemon struct {
	cmd     *exec.Cmd
	logPath string
	stopped bool
}

// startDaemon launches name args with env, writing combined output to a
// per-test log file. Unlike run/mustRun it does not wait: the caller stops it
// via daemon.stop (or the test's cleanup).
func startDaemon(t *testing.T, name string, env []string, args ...string) *daemon {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), name+".log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create %s: %v", logPath, err)
	}
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdout = f
	cmd.Stderr = f
	t.Logf("daemon: %s %s (log %s)", name, strings.Join(args, " "), logPath)
	if err := cmd.Start(); err != nil {
		f.Close()
		t.Fatalf("start %s: %v", name, err)
	}
	d := &daemon{cmd: cmd, logPath: logPath}
	t.Cleanup(func() { d.stop(t) })
	return d
}

// stop interrupts the daemon and waits for it to exit, tolerating an already
// exited process. Log output on a non-zero exit is surfaced through t.Log to
// keep diagnosis in the test record.
func (d *daemon) stop(t *testing.T) {
	t.Helper()
	if d.stopped {
		return
	}
	d.stopped = true
	_ = d.cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- d.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = d.cmd.Process.Kill()
		<-done
		t.Logf("%s did not exit within 30s of SIGINT; killed", d.cmd.Path)
	}
}

// daemonLog returns the daemon's captured output.
func (d *daemon) daemonLog(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(d.logPath)
	if err != nil {
		t.Fatalf("read %s: %v", d.logPath, err)
	}
	return string(b)
}
