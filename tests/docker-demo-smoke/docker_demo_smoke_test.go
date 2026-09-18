// Package dockerdemosmoke contains the smoke test for the demo workflow
// documented in README.md ("Local demo (Docker only)").
//
// The README promises that a fresh reader can start the published demo image
// with Docker alone and drive it with the official AWS CLI container: run the
// demo, s3 ls (connectivity), s3 mb s3://demo-bucket, s3 ls s3://demo-bucket,
// then remove the container. This suite replays those commands verbatim and
// fails when they stop working — which has already happened once, when the
// subcommand dispatcher rejected the demo flags the README passes (the
// published 0.1.1969 image exited 2 on "docker run ... demo --listen ...").
//
// The image under test is pinned, never floating: by default it is
// ghcr.io/jedarden/armor:<contents of the repo's VERSION file>, exactly the
// tag the README points at. ARMOR_SMOKE_IMAGE overrides the reference wholesale.
//
// The image is made available in one of two ways:
//
//   - default: used as-is when present locally, otherwise built from the
//     repo's Dockerfile with `--build-arg VERSION=<VERSION file>` (the same
//     invocation as `make docker`). This keeps the suite runnable on machines
//     with no registry access at all.
//   - ARMOR_SMOKE_PULL=1: `docker pull` the pinned reference instead, which
//     validates that the tag published to GHCR actually serves the
//     documented workflow. This mode fails when the tag is missing (ARMOR's
//     image publishing can lag the VERSION file) or when the published
//     build regresses.
//
// Like tests/aws-cli-compatibility, every test skips cleanly — via t.Skip,
// not failure — when Docker is unavailable or under -short, so plain
// `go test ./...` and `make test` stay green on machines without a daemon.
package dockerdemosmoke

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The demo credentials and networking documented in README.md. These are
// fixed, non-secret demo values printed by the demo subcommand itself; they
// are intentionally not read from the environment.
const (
	readmeAWSCLIImage = "amazon/aws-cli:2.29.0" // pinned by README.md
	readmeAccessKey   = "armor"
	readmeSecretKey   = "armor-demo-secret"
	readmeRegion      = "us-east-1"
	readmeS3Port      = "9000"
	readmeAdminPort   = "9001"
	readmeDemoBucket  = "demo-bucket"
	defaultContainer  = "armor-demo-smoke"
	defaultHostPort   = "9000"
	// defaultAdminHostPort pairs with defaultHostPort so the default port
	// mappings reproduce the README's "-p 9000:9000 -p 9001:9001" exactly.
	defaultAdminHostPort = "9001"
	containerStartWait   = 60 * time.Second
	awsCLICommandWait    = 120 * time.Second
	imagePullWait        = 10 * time.Minute
	imageBuildWait       = 30 * time.Minute // the Dockerfile runs the release gate
)

// TestDockerDemoWorkflow replays README.md "Local demo (Docker only)" from
// start to finish and fails if any documented command no longer works.
func TestDockerDemoWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("docker demo smoke test needs Docker; skipped in -short mode")
	}
	dockerBin, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("docker not found in PATH; the README demo workflow cannot be exercised here")
	}
	if out, err := exec.Command(dockerBin, "info").CombinedOutput(); err != nil {
		t.Skipf("docker daemon unreachable (docker info: %v): %s", err, firstLine(string(out)))
	}

	image := pinnedImage(t)
	ensureImage(t, dockerBin, image)
	ensureAWSCLIImage(t, dockerBin)

	container := envDefault("ARMOR_SMOKE_CONTAINER", defaultContainer)
	hostPort := envDefault("ARMOR_SMOKE_HOST_PORT", defaultHostPort)
	adminHostPort := envDefault("ARMOR_SMOKE_ADMIN_HOST_PORT", defaultAdminHostPort)

	// The README's cleanup step ("docker rm -f armor-demo") is registered
	// first so the demo container never outlives the test, even when a
	// documented step fails midway. A leftover container from an aborted
	// earlier run is removed up front for the same reason.
	removeContainer(t, dockerBin, container)
	t.Cleanup(func() { removeContainer(t, dockerBin, container) })

	// README: docker run -d --name armor-demo -p 9000:9000 -p 9001:9001
	//   ghcr.io/jedarden/armor:<version> demo --listen 0.0.0.0:9000 --admin-listen 0.0.0.0:9001
	runArgs := []string{
		"run", "-d", "--name", container,
		"-p", hostPort + ":" + readmeS3Port,
		"-p", adminHostPort + ":" + readmeAdminPort,
		image, "demo",
		"--listen", "0.0.0.0:" + readmeS3Port,
		"--admin-listen", "0.0.0.0:" + readmeAdminPort,
	}
	if out, err := dockerRun(t, dockerBin, imagePullWait, runArgs...); err != nil {
		t.Fatalf("README demo start command failed: docker %s\nerror: %v\noutput: %s",
			strings.Join(runArgs, " "), err, out)
	}

	// awsCLI runs one documented AWS CLI container invocation. The README
	// joins the demo container's network namespace
	// (--network container:armor-demo) and addresses it on 127.0.0.1, so no
	// host port mapping is involved on the client side.
	awsCLI := func(args ...string) (string, error) {
		awsArgs := []string{
			"run", "--rm", "--network", "container:" + container,
			"-e", "AWS_ACCESS_KEY_ID=" + readmeAccessKey,
			"-e", "AWS_SECRET_ACCESS_KEY=" + readmeSecretKey,
			"-e", "AWS_DEFAULT_REGION=" + readmeRegion,
			readmeAWSCLIImage,
			"--endpoint-url", "http://127.0.0.1:" + readmeS3Port,
		}
		awsArgs = append(awsArgs, args...)
		out, err := dockerRun(t, dockerBin, awsCLICommandWait, awsArgs...)
		if err != nil {
			return out, fmt.Errorf("aws %s: %w\noutput: %s", strings.Join(args, " "), err, out)
		}
		return out, nil
	}

	// README step 1: the connectivity check ("succeeds even when the demo
	// bucket is empty"). The documented reader runs it once the server is
	// up; poll briefly so container startup latency does not flake, and
	// bail out early with demo logs if the container died underneath us.
	var lastErr error
	deadline := time.Now().Add(containerStartWait)
	for {
		if _, err := awsCLI("s3", "ls"); err == nil {
			break
		} else {
			lastErr = err
		}
		if !containerRunning(t, dockerBin, container) {
			t.Fatalf("demo container exited during startup; README connectivity check never succeeded\nlast error: %v\ndemo logs:\n%s",
				lastErr, containerLogs(t, dockerBin, container))
		}
		if time.Now().After(deadline) {
			t.Fatalf("README connectivity check (aws s3 ls) did not succeed within %s\nlast error: %v",
				containerStartWait, lastErr)
		}
		time.Sleep(2 * time.Second)
	}

	// README step 2: create the bucket used by the demo.
	out, err := awsCLI("s3", "mb", "s3://"+readmeDemoBucket)
	if err != nil {
		t.Fatalf("README create-bucket command failed: %v", err)
	}
	// aws s3 mb echoes the bucket it created; a silent success would not
	// prove the bucket exists.
	if !strings.Contains(out, readmeDemoBucket) {
		t.Fatalf("create-bucket output does not mention %s; got: %q", readmeDemoBucket, out)
	}

	// README step 3: list the demo bucket.
	if _, err := awsCLI("s3", "ls", "s3://"+readmeDemoBucket); err != nil {
		t.Fatalf("README list-bucket command failed: %v", err)
	}
}

// pinnedImage returns the image reference under test: ARMOR_SMOKE_IMAGE when
// set, otherwise ghcr.io/jedarden/armor tagged with the repo's VERSION file —
// the pinned tag README.md directs readers to, never a floating reference.
func pinnedImage(t *testing.T) string {
	t.Helper()
	if ref := os.Getenv("ARMOR_SMOKE_IMAGE"); ref != "" {
		return ref
	}
	root := repoRoot(t)
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("cannot read the repo VERSION file to pin the demo image: %v", err)
	}
	v := strings.TrimSpace(string(version))
	if v == "" {
		t.Fatal("repo VERSION file is empty; refusing to run the demo workflow against an unpinned image")
	}
	return "ghcr.io/jedarden/armor:" + v
}

// ensureImage makes the pinned image available locally. With
// ARMOR_SMOKE_PULL=1 it pulls the published reference (and fails when the
// tag is not on GHCR); otherwise it uses an existing local image or
// builds one from the repo's Dockerfile, mirroring `make docker`.
func ensureImage(t *testing.T, dockerBin, image string) {
	t.Helper()
	if os.Getenv("ARMOR_SMOKE_PULL") == "1" {
		if out, err := dockerRun(t, dockerBin, imagePullWait, "pull", image); err != nil {
			t.Fatalf("ARMOR_SMOKE_PULL=1 but the pinned image %s is not pullable: %v\noutput: %s", image, err, out)
		}
		return
	}
	if out, err := dockerRun(t, dockerBin, 30*time.Second, "image", "inspect", image); err == nil {
		return
	} else {
		t.Logf("image %s not present locally (docker image inspect: %v); building it from the repo Dockerfile", image, firstLine(out))
	}
	root := repoRoot(t)
	buildArgs := []string{
		"build",
		"--build-arg", "VERSION=" + versionForBuild(t, root),
		"-t", image,
		"-f", filepath.Join(root, "Dockerfile"),
		root,
	}
	if out, err := dockerRun(t, dockerBin, imageBuildWait, buildArgs...); err != nil {
		t.Fatalf("building %s from the repo Dockerfile failed: %v\noutput: %s", image, err, tailLines(out, 30))
	}
}

// ensureAWSCLIImage makes the pinned AWS CLI client image available locally.
// `docker run` would pull it implicitly on first use, but inside the per-
// command timeout that pull competes with (awsCLICommandWait), so a cold
// image cache would fail the connectivity check spuriously. A pull failure
// is only logged here: when the image genuinely cannot be had, the first
// documented command surfaces the authoritative error.
func ensureAWSCLIImage(t *testing.T, dockerBin string) {
	t.Helper()
	if _, err := dockerRun(t, dockerBin, 30*time.Second, "image", "inspect", readmeAWSCLIImage); err == nil {
		return
	}
	if out, err := dockerRun(t, dockerBin, imagePullWait, "pull", readmeAWSCLIImage); err != nil {
		t.Logf("prefetching %s failed (%v: %s); continuing — the first aws command will retry the pull",
			readmeAWSCLIImage, err, firstLine(out))
	}
}

// versionForBuild returns the VERSION value handed to docker build. It
// defaults to the repo's VERSION file; ARMOR_SMOKE_VERSION overrides it for
// custom ARMOR_SMOKE_IMAGE builds.
func versionForBuild(t *testing.T, root string) string {
	t.Helper()
	if v := os.Getenv("ARMOR_SMOKE_VERSION"); v != "" {
		return v
	}
	raw, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("cannot read the repo VERSION file for the docker build: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

// repoRoot walks up from the test's working directory until it finds the
// repository root, identified by its VERSION file and Dockerfile.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("cannot resolve working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "VERSION")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no repository root (directory with VERSION and Dockerfile) above %s", dir)
		}
		dir = parent
	}
}

// dockerRun runs one docker command with a timeout and returns its combined
// output. DOCKER_HOST and the rest of the caller's environment pass through,
// so the standard docker CLI configuration applies.
func dockerRun(t *testing.T, dockerBin string, timeout time.Duration, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerBin, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("timed out after %s", timeout)
	}
	return string(out), err
}

// containerRunning reports whether the named container is currently running.
func containerRunning(t *testing.T, dockerBin, container string) bool {
	t.Helper()
	out, err := dockerRun(t, dockerBin, 30*time.Second,
		"inspect", "--format", "{{.State.Running}}", container)
	return err == nil && strings.TrimSpace(out) == "true"
}

// containerLogs returns the demo container's logs for failure diagnostics.
func containerLogs(t *testing.T, dockerBin, container string) string {
	t.Helper()
	out, _ := dockerRun(t, dockerBin, 30*time.Second, "logs", container)
	return tailLines(out, 40)
}

// removeContainer removes the demo container, tolerating absence.
func removeContainer(t *testing.T, dockerBin, container string) {
	t.Helper()
	_, _ = dockerRun(t, dockerBin, 60*time.Second, "rm", "-f", container)
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
