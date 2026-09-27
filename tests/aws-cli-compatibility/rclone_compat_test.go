// rclone compatibility leg — the real rclone binary against the real ARMOR
// request pipeline, through an S3 remote rendered into a private rclone.conf.
//
// rclone is the sync tool in the documented client matrix, and this leg mirrors
// the coverage shapes the always-running protocol conformance tests pin at the
// wire level (protocol_conformance_test.go): authentication accept-and-reject,
// single-PUT writes, listing, byte-range reads, overwrite and delete, and a
// forced multipart upload. Every invocation authenticates with the harness's
// synthetic, ephemeral pair (or the endpoint-mode server's own demo pair)
// delivered through the rendered config file; real deployments keep their
// ARMOR client credentials in OpenBao and nothing here ever reads one.
package awsclicompat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRcloneConf renders an rclone.conf with an S3 remote named "armor"
// pointing at endpoint and authenticated with the given credential pair, and
// returns (configPath, remoteName). rcloneConf is the harness-credential form
// the legs use; the authentication leg renders corrupted-pair variants through
// this form to pin ARMOR's rejection codes as rclone surfaces them.
func writeRcloneConf(t *testing.T, endpoint, accessKey, secretKey string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	confPath := filepath.Join(dir, "rclone.conf")
	body := fmt.Sprintf("[armor]\ntype = s3\nprovider = Other\nendpoint = %s\n"+
		"access_key_id = %s\nsecret_access_key = %s\nregion = %s\n"+
		"force_path_style = true\nno_check_bucket = true\n",
		endpoint, accessKey, secretKey, testRegion)
	if err := os.WriteFile(confPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write rclone config: %v", err)
	}
	return confPath, "armor"
}

// rcloneBase prefixes every invocation with --config so the test never reads
// (or writes) a user's own remotes.
func rcloneBase(conf string) []string { return []string{"--config", conf} }

// rcloneArgs appends args to a base slice without mutating the base.
func rcloneArgs(base []string, args ...string) []string {
	return append(append([]string{}, base...), args...)
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

	base := rcloneBase(conf)

	// Push to ARMOR.
	mustRun(t, "rclone", nil, rcloneArgs(base, "copy", srcDir, remote+":"+compatBucket(t)+"/rclone/")...)

	// Pull into a fresh directory and compare.
	dstDir := filepath.Join(work, "dst")
	mustRun(t, "rclone", nil, rcloneArgs(base, "copy", remote+":"+compatBucket(t)+"/rclone/", dstDir)...)

	for name := range files {
		assertFilesEqual(t, filepath.Join(srcDir, name), filepath.Join(dstDir, name))
	}
	// Sanity: files actually landed on the server (dst is non-empty).
	if ents, err := os.ReadDir(dstDir); err != nil || len(ents) == 0 {
		t.Fatalf("rclone pulled no files into %s", dstDir)
	}
}

// TestRclone_Authentication verifies the credential contract as rclone
// experiences it: a correctly configured remote lists the bucket, a corrupted
// secret fails with SignatureDoesNotMatch, and an unknown access key fails
// with InvalidAccessKeyId — the same standard error codes
// protocol_conformance_test.go pins at the wire level.
func TestRclone_Authentication(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	bucket := compatBucket(t)
	accessKey, secretKey := compatCredentials(t)

	conf, remote := rcloneConf(t, endpoint)

	// Positive control: the configured pair lists the bucket.
	mustRun(t, "rclone", nil, rcloneArgs(rcloneBase(conf), "lsf", remote+":"+bucket)...)

	cases := []struct {
		name       string
		accessKey  string
		secretKey  string
		wantErrSub string
	}{
		{"wrong secret", accessKey, secretKey + "-corrupted", "signaturedoesnotmatch"},
		{"unknown access key", "ARMORUNKNOWNKEY", secretKey, "invalidaccesskeyid"},
	}
	for _, tc := range cases {
		badConf, badRemote := writeRcloneConf(t, endpoint, tc.accessKey, tc.secretKey)
		out, err := run(t, "rclone", nil, rcloneArgs(rcloneBase(badConf), "lsf", badRemote+":"+bucket)...)
		if err == nil {
			t.Fatalf("%s: rclone lsf against bucket %s exited 0; want rejection:\n%s", tc.name, bucket, out)
		}
		if !strings.Contains(strings.ToLower(out), tc.wantErrSub) {
			t.Fatalf("%s: rclone output missing %s:\n%s", tc.name, tc.wantErrSub, out)
		}
	}
}

// TestRclone_SinglePutGet verifies the single-request write path: `rclone
// copyto` (one file, below the multipart cutoff) uploads a single PUT and
// `rclone cat` reads back the identical bytes; the listing reports the
// uploaded size.
func TestRclone_SinglePutGet(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()
	base := rcloneBase(conf)
	bucket := compatBucket(t)

	payload := randomData(96 * 1024)
	src := writeFile(t, work, "obj.bin", payload)
	key := "rclone-singleput/obj.bin"
	target := remote + ":" + bucket + "/" + key

	mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", src, target)...)

	out := mustRun(t, "rclone", nil, rcloneArgs(base, "cat", target)...)
	if !bytes.Equal([]byte(out), payload) {
		t.Fatalf("rclone cat returned %d bytes with different content, want %d", len(out), len(payload))
	}

	// The listing reports the object at its uploaded size.
	out = mustRun(t, "rclone", nil, rcloneArgs(base, "lsjson", "--files-only", remote+":"+bucket+"/"+filepath.Dir(key))...)
	var entries []rcloneListEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse rclone lsjson output: %v\n%s", err, out)
	}
	if len(entries) != 1 || entries[0].Path != "obj.bin" || entries[0].Size != int64(len(payload)) {
		t.Fatalf("listing did not report obj.bin at %d bytes: %s", len(payload), out)
	}
}

// rcloneListEntry is one record of `rclone lsjson` output.
type rcloneListEntry struct {
	Path string `json:"Path"`
	Size int64  `json:"Size"`
}

// rcloneList runs `rclone lsjson --recursive --files-only` against path and
// returns the entries keyed by their path relative to the listed remote.
// --files-only drops directory records, whose Size is -1.
func rcloneList(t *testing.T, base []string, path string) map[string]int64 {
	t.Helper()
	out := mustRun(t, "rclone", nil, rcloneArgs(base, "lsjson", "--recursive", "--files-only", path)...)
	var entries []rcloneListEntry
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("parse rclone lsjson output: %v\n%s", err, out)
	}
	got := make(map[string]int64, len(entries))
	for _, e := range entries {
		got[e.Path] = e.Size
	}
	return got
}

// TestRclone_Listing verifies listing semantics as rclone renders them: an
// uploaded tree comes back with its exact keys and sizes, and listing a
// sub-prefix scopes the result to that subtree.
func TestRclone_Listing(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()
	base := rcloneBase(conf)
	bucket := compatBucket(t)

	prefix := "rclone-list"
	files := map[string][]byte{
		"alpha.txt":    []byte("alpha"),
		"beta.bin":     randomData(20000),
		"nested/gamma": randomData(333),
	}
	for name, data := range files {
		src := writeFile(t, work, name, data)
		mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", src, remote+":"+bucket+"/"+prefix+"/"+name)...)
	}

	got := rcloneList(t, base, remote+":"+bucket+"/"+prefix)
	if len(got) != len(files) {
		t.Fatalf("lsjson listed %d keys, want %d: %v", len(got), len(files), got)
	}
	for name, data := range files {
		if size, ok := got[name]; !ok {
			t.Errorf("lsjson missing %s", name)
		} else if size != int64(len(data)) {
			t.Errorf("lsjson size for %s = %d, want %d", name, size, len(data))
		}
	}

	// A sub-prefix listing returns only the nested subtree, with paths
	// relative to the listed directory.
	got = rcloneList(t, base, remote+":"+bucket+"/"+prefix+"/nested")
	if len(got) != 1 {
		t.Fatalf("lsjson on the nested prefix returned %d keys, want 1: %v", len(got), got)
	}
	if _, ok := got["gamma"]; !ok {
		t.Fatalf("nested listing missing gamma: %v", got)
	}
}

// TestRclone_RangeReads verifies byte-range reads through `rclone cat`:
// a bounded window, a window crossing the 64 KiB seekable-encryption block
// boundary, and the first bytes via --head — each byte-exact.
func TestRclone_RangeReads(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()
	base := rcloneBase(conf)
	bucket := compatBucket(t)

	// 200,000 bytes spans three 64 KiB blocks, matching the wire-level pin in
	// protocol_conformance_test.go.
	payload := randomData(200_000)
	src := writeFile(t, work, "range.bin", payload)
	target := remote + ":" + bucket + "/rclone-range/range.bin"
	mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", src, target)...)

	cat := func(extra ...string) string {
		t.Helper()
		return mustRun(t, "rclone", nil, rcloneArgs(append(append([]string{}, base...), extra...), "cat", target)...)
	}
	assertSlice := func(label string, got string, from, to int) {
		t.Helper()
		wantSlice := payload[from:to]
		if !bytes.Equal([]byte(got), wantSlice) {
			t.Fatalf("%s: got %d bytes (content mismatch), want %d", label, len(got), len(wantSlice))
		}
	}

	assertSlice("offset/count mid-block", cat("--offset", "1000", "--count", "500"), 1000, 1500)
	assertSlice("offset/count crossing block boundary",
		cat("--offset", "65530", "--count", "21"), 65530, 65551)
	assertSlice("--head", cat("--head", "17"), 0, 17)
}

// TestRclone_OverwriteDelete verifies overwrite and delete semantics: pushing
// new content to an existing key replaces the stored payload (a read sees only
// the new bytes at the new length), and `rclone deletefile` removes the object
// so a following listing of its prefix comes back empty.
func TestRclone_OverwriteDelete(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()
	base := rcloneBase(conf)
	bucket := compatBucket(t)

	prefix := "rclone-overwrite"
	key := prefix + "/obj.bin"
	target := remote + ":" + bucket + "/" + key

	first := writeFile(t, work, "first.bin", randomData(32*1024))
	secondPayload := randomData(96 * 1024)
	second := writeFile(t, work, "second.bin", secondPayload)

	mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", first, target)...)
	mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", second, target)...)

	// The read returns exactly the second payload — replaced, not appended.
	out := mustRun(t, "rclone", nil, rcloneArgs(base, "cat", target)...)
	if !bytes.Equal([]byte(out), secondPayload) {
		t.Fatalf("read after overwrite returned %d bytes (content mismatch), want the %d-byte replacement", len(out), len(secondPayload))
	}
	got := rcloneList(t, base, remote+":"+bucket+"/"+prefix)
	if len(got) != 1 || got["obj.bin"] != int64(len(secondPayload)) {
		t.Fatalf("listing after overwrite = %v, want only obj.bin at %d bytes", got, len(secondPayload))
	}

	// Delete removes the object; the prefix lists empty afterwards.
	mustRun(t, "rclone", nil, rcloneArgs(base, "deletefile", target)...)
	out = mustRun(t, "rclone", nil, rcloneArgs(base, "lsf", remote+":"+bucket+"/"+prefix+"/")...)
	if strings.TrimSpace(out) != "" {
		t.Fatalf("prefix not empty after deletefile:\n%s", out)
	}
}

// TestRclone_MultipartUpload verifies a forced multipart upload: a 12 MiB file
// with a 6 MiB cutoff and 5 MiB chunks uploads as three parts (5+5+2 MiB) —
// the small-scale analogue of the ADR-015 acceptance criterion — round-trips
// byte-identically, and the assembled object is range-readable across the
// first part boundary.
func TestRclone_MultipartUpload(t *testing.T) {
	requireRclone(t)
	endpoint := startArmorServer(t)
	conf, remote := rcloneConf(t, endpoint)
	work := t.TempDir()
	base := rcloneBase(conf)
	bucket := compatBucket(t)

	// 12 MiB over a 5 MiB chunk size => parts of 5+5+2 MiB; the first boundary
	// sits at exactly 5 MiB.
	payload := randomData(12 * 1024 * 1024)
	src := writeFile(t, work, "big.bin", payload)
	const partBoundary = 5 * 1024 * 1024
	key := "rclone-multipart/big.bin"
	target := remote + ":" + bucket + "/" + key

	mustRun(t, "rclone", nil, rcloneArgs(base,
		"copyto", "--s3-upload-cutoff", "6M", "--s3-chunk-size", "5M", src, target)...)

	// The download carries --ignore-checksum: ARMOR's completed-multipart ETag
	// is a bare-hex digest of the stored ciphertext, and rclone reads any bare
	// hex ETag as the content MD5 — real S3 marks multipart objects with the
	// "md5(md5s)-N" composite form, which tells rclone to skip that check. The
	// byte-for-byte comparison below is the actual integrity assertion; the
	// ETag format itself is tracked as separate server work.
	got := filepath.Join(work, "big.out")
	mustRun(t, "rclone", nil, rcloneArgs(base, "copyto", "--ignore-checksum", target, got)...)
	assertFilesEqual(t, src, got)

	// Range read across the part boundary (which is also a 64 KiB block
	// boundary) proves the assembled object is seekable, not just whole-file.
	out := mustRun(t, "rclone", nil, rcloneArgs(base,
		"cat", "--offset", fmt.Sprint(partBoundary-10), "--count", "21", target)...)
	if !bytes.Equal([]byte(out), payload[partBoundary-10:partBoundary+11]) {
		t.Fatalf("range read across the part boundary returned wrong content (%d bytes)", len(out))
	}
}
