package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureInitOutput(t *testing.T, f func()) (string, string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = outW, errW
	f()
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	var out, diagnostic bytes.Buffer
	io.Copy(&out, outR)
	io.Copy(&diagnostic, errR)
	outR.Close()
	errR.Close()
	return out.String(), diagnostic.String()
}

func initTestValues() initValues {
	return initValues{
		region:         "us-east-005",
		bucket:         "init-test-bucket",
		b2AccessKeyID:  "b2-key-id",
		b2Credential:   "b2-application-key-value",
		cfDomain:       "objects.example.test",
		authAccessKey:  "armor-client-key",
		authCredential: "armor-client-secret-value",
	}
}

func envTestValue(t *testing.T, contents, key string) string {
	t.Helper()
	for _, line := range strings.Split(contents, "\n") {
		_, value, found := envAssignment(line, key)
		if found {
			return value
		}
	}
	t.Fatalf("%s missing from environment file:\n%s", key, contents)
	return ""
}

func TestInitializeEnvFileGeneratesProtectedSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	values := initTestValues()
	if err := initializeEnvFile(path, values); err != nil {
		t.Fatalf("initializeEnvFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat environment file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("environment file mode = %04o, want 0600", got)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read environment file: %v", err)
	}
	mek := envTestValue(t, string(contents), "ARMOR_MEK")
	admin := envTestValue(t, string(contents), "ARMOR_ADMIN_TOKEN")
	for name, value := range map[string]string{"ARMOR_MEK": mek, "ARMOR_ADMIN_TOKEN": admin} {
		if len(value) != 64 {
			t.Errorf("%s length = %d, want 64", name, len(value))
		}
		if _, err := hex.DecodeString(value); err != nil {
			t.Errorf("%s is not hex: %v", name, err)
		}
	}
	if mek == admin {
		t.Error("MEK and admin token must be independently generated")
	}
}

func TestInitializeEnvFilePreservesExistingSecretValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	oldMEK := strings.Repeat("a", 64)
	oldAdmin := strings.Repeat("b", 64)
	contents := "ARMOR_MEK=" + oldMEK + "\nARMOR_ADMIN_TOKEN=\"" + oldAdmin + "\"\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write existing environment file: %v", err)
	}

	if err := initializeEnvFile(path, initTestValues()); err != nil {
		t.Fatalf("initializeEnvFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read environment file: %v", err)
	}
	if !strings.Contains(string(got), "ARMOR_MEK="+oldMEK) || !strings.Contains(string(got), oldAdmin) {
		t.Fatalf("existing secret values were not preserved:\n%s", got)
	}
	if mode := fileMode(t, path); mode != 0o600 {
		t.Fatalf("existing environment file mode = %04o, want 0600", mode)
	}
}

func TestRunInitDoesNotPrintCredentialValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	old := struct {
		path, region, bucket, b2ID, cf, authID, b2File, authFile string
		nonInteractive                                           bool
	}{initEnvFileFlag, initRegionFlag, initBucketFlag, initB2AccessKeyIDFlag, initCFDomainFlag, initAuthAccessKeyFlag, initB2CredentialFileFlag, initAuthCredentialFileFlag, initNonInteractiveFlag}
	t.Cleanup(func() {
		initEnvFileFlag, initRegionFlag, initBucketFlag, initB2AccessKeyIDFlag = old.path, old.region, old.bucket, old.b2ID
		initCFDomainFlag, initAuthAccessKeyFlag = old.cf, old.authID
		initB2CredentialFileFlag, initAuthCredentialFileFlag, initNonInteractiveFlag = old.b2File, old.authFile, old.nonInteractive
	})
	initEnvFileFlag = path
	initRegionFlag, initBucketFlag = "us-east-005", "init-test-bucket"
	initB2AccessKeyIDFlag, initCFDomainFlag = "b2-key-id", "objects.example.test"
	initAuthAccessKeyFlag = "armor-client-key"
	initB2CredentialFileFlag, initAuthCredentialFileFlag = "", ""
	initNonInteractiveFlag = true
	t.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", "b2-credential-not-for-output")
	t.Setenv("ARMOR_AUTH_SECRET_KEY", "auth-credential-not-for-output")

	stdout, stderr := captureInitOutput(t, func() {
		runInit(flag.NewFlagSet("init-test", flag.ContinueOnError))
	})
	for _, value := range []string{"b2-credential-not-for-output", "auth-credential-not-for-output"} {
		if strings.Contains(stdout+stderr, value) {
			t.Errorf("credential value appeared in command output: %q", value)
		}
	}
	if !strings.Contains(stdout, "back up ARMOR_MEK") {
		t.Errorf("MEK backup warning missing from stdout: %q", stdout)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
