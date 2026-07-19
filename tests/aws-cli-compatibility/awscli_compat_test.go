package awsclicompat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// endpointFlag returns the standard --endpoint-url argument pair.
func endpointFlag(endpoint string) []string {
	return []string{"--endpoint-url", endpoint}
}

// s3URL builds an s3:// URL for a key in the test bucket.
func s3URL(key string) string { return "s3://" + testBucket + "/" + key }

// TestAWSCLI_PutGetRoundTrip verifies the core compatibility contract: an
// object uploaded with `aws s3 cp` downloads byte-identically, and head-object
// returns its metadata.
func TestAWSCLI_PutGetRoundTrip(t *testing.T) {
	requireAWSCLI(t)
	endpoint := startArmorServer(t)
	env := awsEnv(t, endpoint, false)
	work := t.TempDir()

	payload := []byte("ARMOR aws-cli compatibility round-trip\n")
	want := writeFile(t, work, "in.txt", payload)
	key := "roundtrip/in.txt"

	// Upload.
	mustRun(t, "aws", env, append([]string{"s3", "cp", want, s3URL(key)}, endpointFlag(endpoint)...)...)

	// head-object returns metadata without failing.
	headOut := mustRun(t, "aws", env, append([]string{
		"s3api", "head-object", "--bucket", testBucket, "--key", key,
	}, endpointFlag(endpoint)...)...)
	if !strings.Contains(headOut, "ContentLength") {
		t.Fatalf("head-object response missing ContentLength:\n%s", headOut)
	}

	// Download to a fresh path and compare.
	got := filepath.Join(work, "out.txt")
	mustRun(t, "aws", env, append([]string{"s3", "cp", s3URL(key), got}, endpointFlag(endpoint)...)...)
	assertFilesEqual(t, want, got)
}

// TestAWSCLI_ListAndDelete verifies `aws s3 ls` lists uploaded objects and
// `aws s3 rm` deletes them (after which ls reports the object gone).
func TestAWSCLI_ListAndDelete(t *testing.T) {
	requireAWSCLI(t)
	endpoint := startArmorServer(t)
	env := awsEnv(t, endpoint, false)
	work := t.TempDir()

	uploadOne := func(name, key string) {
		src := writeFile(t, work, name, []byte("contents of "+name))
		mustRun(t, "aws", env, append([]string{"s3", "cp", src, s3URL(key)}, endpointFlag(endpoint)...)...)
	}
	uploadOne("a.txt", "lsrm/a.txt")
	uploadOne("b.txt", "lsrm/b.txt")

	ls := func(prefix string) string {
		return mustRun(t, "aws", env, append([]string{"s3", "ls", s3URL(prefix)}, endpointFlag(endpoint)...)...)
	}
	// lsAllowEmpty lists a prefix but tolerates the exit-code-1 + empty-output
	// case aws-cli v1 returns for a prefix with zero objects (e.g. one that was
	// just cleared by a recursive rm). That exit code is aws-cli's own
	// "directory does not exist" policy — identical to its behavior against real
	// S3 — and is *not* an ARMOR error: the same empty <ListBucketResult> XML
	// yields exit 0 when listing an empty bucket root. A non-zero exit paired
	// with *non-empty* output is a real error and still fatals.
	lsAllowEmpty := func(prefix string) string {
		out, err := run(t, "aws", env, append([]string{"s3", "ls", s3URL(prefix)}, endpointFlag(endpoint)...)...)
		if err != nil && strings.TrimSpace(out) != "" {
			t.Fatalf("aws s3 ls %s failed: %v\n%s", s3URL(prefix), err, out)
		}
		return out
	}

	out := ls("lsrm/")
	if !strings.Contains(out, "a.txt") || !strings.Contains(out, "b.txt") {
		t.Fatalf("ls did not list both objects:\n%s", out)
	}

	// Delete a single object, then confirm it is gone.
	mustRun(t, "aws", env, append([]string{"s3", "rm", s3URL("lsrm/a.txt")}, endpointFlag(endpoint)...)...)
	out = ls("lsrm/")
	if strings.Contains(out, "a.txt") {
		t.Fatalf("a.txt still listed after delete:\n%s", out)
	}
	if !strings.Contains(out, "b.txt") {
		t.Fatalf("b.txt missing after deleting a.txt:\n%s", out)
	}

	// Recursive delete clears the rest.
	mustRun(t, "aws", env, append([]string{"s3", "rm", s3URL("lsrm/"), "--recursive"}, endpointFlag(endpoint)...)...)
	// After recursive delete the prefix has no objects. aws-cli v1 returns exit
	// code 1 with empty output for such a now-empty prefix (see lsAllowEmpty), so
	// assert on what matters — the remaining object is gone — rather than on the
	// exit code.
	out = lsAllowEmpty("lsrm/")
	if strings.Contains(out, "b.txt") {
		t.Fatalf("b.txt still listed after recursive delete:\n%s", out)
	}
}

// TestAWSCLI_Sync verifies `aws s3 sync` mirrors a local directory tree to
// ARMOR and back, preserving contents.
func TestAWSCLI_Sync(t *testing.T) {
	requireAWSCLI(t)
	endpoint := startArmorServer(t)
	env := awsEnv(t, endpoint, false)
	work := t.TempDir()

	srcDir := filepath.Join(work, "src")
	files := map[string][]byte{
		"top.txt":           randomData(1024),
		"nested/deep.txt":   randomData(4096),
		"nested/more/x.bin": randomData(16 * 1024),
	}
	for name, data := range files {
		writeFile(t, srcDir, name, data)
	}

	// Sync up.
	mustRun(t, "aws", env, append([]string{"s3", "sync", srcDir, s3URL("sync/")}, endpointFlag(endpoint)...)...)

	// Sync down into a fresh directory and compare every file.
	dstDir := filepath.Join(work, "dst")
	mustRun(t, "aws", env, append([]string{"s3", "sync", s3URL("sync/"), dstDir}, endpointFlag(endpoint)...)...)
	for name := range files {
		assertFilesEqual(t, filepath.Join(srcDir, name), filepath.Join(dstDir, name))
	}
}

// TestAWSCLI_MultipartUpload verifies a multipart upload (default aws-cli
// concurrency) round-trips byte-identically. This is the small-scale analogue
// of the ADR-005 acceptance criterion.
func TestAWSCLI_MultipartUpload(t *testing.T) {
	requireAWSCLI(t)
	endpoint := startArmorServer(t)
	env := awsEnv(t, endpoint, true) // low multipart threshold
	work := t.TempDir()

	// 9 MiB with an 8 MiB threshold => two parts (8 MiB + 1 MiB).
	want := writeFile(t, work, "big.bin", randomData(9*1024*1024))
	key := "multipart/big.bin"

	mustRun(t, "aws", env, append([]string{"s3", "cp", want, s3URL(key)}, endpointFlag(endpoint)...)...)

	got := filepath.Join(work, "big.out")
	mustRun(t, "aws", env, append([]string{"s3", "cp", s3URL(key), got}, endpointFlag(endpoint)...)...)
	assertFilesEqual(t, want, got)
}

// TestAWSCLI_CopyObject verifies server-side copy (`aws s3 cp s3://a s3://b`)
// produces a second decryptable object identical to the source.
func TestAWSCLI_CopyObject(t *testing.T) {
	requireAWSCLI(t)
	endpoint := startArmorServer(t)
	env := awsEnv(t, endpoint, false)
	work := t.TempDir()

	want := writeFile(t, work, "src.txt", randomData(32*1024))
	srcKey, dstKey := "copy/src.txt", "copy/dst.txt"
	mustRun(t, "aws", env, append([]string{"s3", "cp", want, s3URL(srcKey)}, endpointFlag(endpoint)...)...)

	// Server-side copy.
	mustRun(t, "aws", env, append([]string{"s3", "cp", s3URL(srcKey), s3URL(dstKey)}, endpointFlag(endpoint)...)...)

	// Download the copy and confirm it matches the original file.
	got := filepath.Join(work, "dst.txt")
	mustRun(t, "aws", env, append([]string{"s3", "cp", s3URL(dstKey), got}, endpointFlag(endpoint)...)...)
	assertFilesEqual(t, want, got)
}

// TestRclone_CopyRoundTrip verifies `rclone copy` pushes a tree to ARMOR and
// pulls it back unchanged, using an S3 remote configured against the
// in-process server.
func TestRclone_CopyRoundTrip(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()

	srcDir := filepath.Join(work, "src")
	files := map[string][]byte{
		"one.txt":     []byte("rclone one"),
		"sub/two.dat": randomData(64 * 1024),
	}
	for name, data := range files {
		writeFile(t, srcDir, name, data)
	}

	base := []string{"--config", conf}

	// Push to ARMOR.
	mustRun(t, "rclone", nil, append(append([]string{},
		base...), "copy", srcDir, remote+":"+testBucket+"/rclone/")...)

	// Pull into a fresh directory and compare.
	dstDir := filepath.Join(work, "dst")
	mustRun(t, "rclone", nil, append(append([]string{},
		base...), "copy", remote+":"+testBucket+"/rclone/", dstDir)...)

	for name := range files {
		assertFilesEqual(t, filepath.Join(srcDir, name), filepath.Join(dstDir, name))
	}
	// Sanity: files actually landed on the server (dst is non-empty).
	if ents, err := os.ReadDir(dstDir); err != nil || len(ents) == 0 {
		t.Fatalf("rclone pulled no files into %s", dstDir)
	}
}
