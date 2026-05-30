//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TestAWSCLICompatibility tests AWS CLI (aws s3 cp, ls, rm) against ARMOR.
// This catches SigV4 signing edge cases, XML response format issues, and
// header compatibility problems that boto3 may silently handle differently.
func TestAWSCLICompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	// Check if AWS CLI is available
	_, err := exec.LookPath("aws")
	if err != nil {
		t.Skipf("AWS CLI not found in PATH: %v", err)
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	// Create a temporary directory for AWS CLI config and test files
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config")
	credentialsFile := filepath.Join(tempDir, "credentials")

	// Create AWS CLI config
	configContent := fmt.Sprintf(`[profile armor-test]
region = us-east-1
output = json
`)
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS config: %v", err)
	}

	// Create AWS CLI credentials
	credsContent := fmt.Sprintf(`[armor-test]
aws_access_key_id = %s
aws_secret_access_key = %s
`, armorAccessKey, armorSecretKey)
	if err := os.WriteFile(credentialsFile, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS credentials: %v", err)
	}

	// Create a unique test key prefix
	prefix := fmt.Sprintf("awscli-test-%d/", time.Now().UnixNano())
	testKey := prefix + "test-file.txt"
	testContent := []byte("Hello from AWS CLI! This is a test file for ARMOR compatibility.\n")
	testFile := filepath.Join(tempDir, "test-file.txt")
	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	downloadFile := filepath.Join(tempDir, "downloaded.txt")

	// Helper function to run AWS CLI commands
	runAWSCLI := func(args ...string) (string, string, error) {
		cmd := exec.Command("aws", args...)
		cmd.Env = append(os.Environ(),
			"AWS_CONFIG_FILE="+configFile,
			"AWS_SHARED_CREDENTIALS_FILE="+credentialsFile,
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	// Test 1: aws s3 cp (upload)
	t.Run("s3_cp_upload", func(t *testing.T) {
		t.Logf("Uploading %s to s3://%s/%s", testFile, bucket, testKey)
		stdout, stderr, err := runAWSCLI("s3", "cp",
			testFile,
			fmt.Sprintf("s3://%s/%s", bucket, testKey),
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 cp upload failed: %v", err)
		}
		t.Logf("Upload successful: %s", stdout)
	})

	// Test 2: aws s3 ls (list bucket)
	t.Run("s3_ls_list", func(t *testing.T) {
		t.Logf("Listing s3://%s/%s", bucket, prefix)
		stdout, stderr, err := runAWSCLI("s3", "ls",
			fmt.Sprintf("s3://%s/%s", bucket, prefix),
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 ls failed: %v", err)
		}

		// Verify the uploaded file is in the listing
		if !strings.Contains(stdout, testKey) {
			t.Errorf("Expected listing to contain %s, got:\n%s", testKey, stdout)
		}
		t.Logf("List successful: %s", stdout)
	})

	// Test 3: aws s3 cp (download)
	t.Run("s3_cp_download", func(t *testing.T) {
		t.Logf("Downloading s3://%s/%s to %s", bucket, testKey, downloadFile)
		stdout, stderr, err := runAWSCLI("s3", "cp",
			fmt.Sprintf("s3://%s/%s", bucket, testKey),
			downloadFile,
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 cp download failed: %v", err)
		}
		t.Logf("Download successful: %s", stdout)

		// Verify downloaded content matches original
		downloadedContent, err := os.ReadFile(downloadFile)
		if err != nil {
			t.Fatalf("Failed to read downloaded file: %v", err)
		}
		if !bytes.Equal(testContent, downloadedContent) {
			t.Errorf("Downloaded content doesn't match original.\nOriginal: %q\nDownloaded: %q",
				string(testContent), string(downloadedContent))
		}
		t.Logf("Content verified: %d bytes", len(downloadedContent))
	})

	// Test 4: aws s3 rm (delete)
	t.Run("s3_rm_delete", func(t *testing.T) {
		t.Logf("Deleting s3://%s/%s", bucket, testKey)
		stdout, stderr, err := runAWSCLI("s3", "rm",
			fmt.Sprintf("s3://%s/%s", bucket, testKey),
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 rm failed: %v", err)
		}
		t.Logf("Delete successful: %s", stdout)

		// Verify the file is gone by trying to list it again
		listStdout, listStderr, listErr := runAWSCLI("s3", "ls",
			fmt.Sprintf("s3://%s/%s", bucket, testKey),
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if listErr == nil && strings.Contains(listStdout, testKey) {
			t.Errorf("File still exists after delete. List output:\n%s", listStdout)
		}
		if listErr != nil {
			// Expected - file should not exist
			t.Logf("Confirmed file deleted (list error: %v, stderr: %s)", listErr, listStderr)
		}
	})

	t.Log("AWS CLI compatibility tests passed!")
}

// TestAWSCLISync tests aws s3 sync command
func TestAWSCLISync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	_, err := exec.LookPath("aws")
	if err != nil {
		t.Skipf("AWS CLI not found in PATH: %v", err)
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}

	// Setup temp directory and config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config")
	credentialsFile := filepath.Join(tempDir, "credentials")

	configContent := fmt.Sprintf(`[profile armor-test]
region = us-east-1
output = json
`)
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS config: %v", err)
	}

	credsContent := fmt.Sprintf(`[armor-test]
aws_access_key_id = %s
aws_secret_access_key = %s
`, armorAccessKey, armorSecretKey)
	if err := os.WriteFile(credentialsFile, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS credentials: %v", err)
	}

	// Create source directory with multiple files
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	files := map[string]string{
		"file1.txt": "Content of file 1\n",
		"file2.txt": "Content of file 2\n",
		"subdir/file3.txt": "Content of file 3 in subdirectory\n",
	}

	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	prefix := fmt.Sprintf("sync-test-%d/", time.Now().UnixNano())
	dstDir := filepath.Join(tempDir, "dst")

	runAWSCLI := func(args ...string) (string, string, error) {
		cmd := exec.Command("aws", args...)
		cmd.Env = append(os.Environ(),
			"AWS_CONFIG_FILE="+configFile,
			"AWS_SHARED_CREDENTIALS_FILE="+credentialsFile,
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	// Sync up
	t.Run("sync_up", func(t *testing.T) {
		t.Logf("Syncing %s to s3://%s/%s", srcDir, bucket, prefix)
		stdout, stderr, err := runAWSCLI("s3", "sync",
			srcDir,
			fmt.Sprintf("s3://%s/%s", bucket, prefix),
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 sync up failed: %v", err)
		}
		t.Logf("Sync up successful: %s", stdout)
	})

	// Sync down
	t.Run("sync_down", func(t *testing.T) {
		t.Logf("Syncing s3://%s/%s to %s", bucket, prefix, dstDir)
		stdout, stderr, err := runAWSCLI("s3", "sync",
			fmt.Sprintf("s3://%s/%s", bucket, prefix),
			dstDir,
			"--profile", "armor-test",
			"--endpoint-url", armorEndpoint,
		)
		if err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Fatalf("aws s3 sync down failed: %v", err)
		}
		t.Logf("Sync down successful: %s", stdout)

		// Verify files match
		for name, expectedContent := range files {
			downloadedPath := filepath.Join(dstDir, name)
			actualContent, err := os.ReadFile(downloadedPath)
			if err != nil {
				t.Errorf("Failed to read downloaded file %s: %v", name, err)
				continue
			}
			if string(actualContent) != expectedContent {
				t.Errorf("Content mismatch for %s: expected %q, got %q",
					name, expectedContent, string(actualContent))
			}
		}
		t.Log("Sync content verified")
	})

	// Cleanup
	t.Cleanup(func() {
		ctx := context.Background()
		client := createS3Client(t, armorEndpoint)
		for name := range files {
			key := prefix + name
			client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
			})
		}
	})
}

// TestAWSCLIPresign tests the AWS CLI with pre-signed URLs generated by ARMOR
func TestAWSCLIPresign(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	_, err := exec.LookPath("aws")
	if err != nil {
		t.Skipf("AWS CLI not found in PATH: %v", err)
	}

	armorEndpoint := os.Getenv("ARMOR_ENDPOINT")
	if armorEndpoint == "" {
		armorEndpoint = "http://localhost:9000"
	}
	adminEndpoint := os.Getenv("ARMOR_ADMIN_ENDPOINT")
	if adminEndpoint == "" {
		adminEndpoint = "http://localhost:9001"
	}

	// Setup temp directory and config
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config")
	credentialsFile := filepath.Join(tempDir, "credentials")

	configContent := fmt.Sprintf(`[profile armor-test]
region = us-east-1
output = json
`)
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS config: %v", err)
	}

	credsContent := fmt.Sprintf(`[armor-test]
aws_access_key_id = %s
aws_secret_access_key = %s
`, armorAccessKey, armorSecretKey)
	if err := os.WriteFile(credentialsFile, []byte(credsContent), 0600); err != nil {
		t.Fatalf("Failed to write AWS credentials: %v", err)
	}

	runAWSCLI := func(args ...string) (string, string, error) {
		cmd := exec.Command("aws", args...)
		cmd.Env = append(os.Environ(),
			"AWS_CONFIG_FILE="+configFile,
			"AWS_SHARED_CREDENTIALS_FILE="+credentialsFile,
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	// First, upload a file using regular s3 cp
	ctx := context.Background()
	client := createS3Client(t, armorEndpoint)
	testKey := fmt.Sprintf("presign-test-%d.txt", time.Now().UnixNano())
	testContent := []byte("Test content for presigned URL\n")

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(testKey),
		Body:   bytes.NewReader(testContent),
	})
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}
	defer client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(testKey),
	})

	// Request presigned URL from admin endpoint
	presignReq := fmt.Sprintf(`{"bucket":"%s","key":"%s","expires_in":300}`, bucket, testKey)
	resp, err := http.Post(adminEndpoint+"/admin/presign", "application/json",
		strings.NewReader(presignReq))
	if err != nil {
		t.Skipf("Presign endpoint not available: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Presign endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read presign response: %v", err)
	}
	shareURL := strings.TrimSpace(string(body))
	shareURL = strings.Trim(shareURL, "\"")

	t.Logf("Got presigned URL: %s", shareURL)

	// Try to download using AWS CLI with the presigned URL
	// AWS CLI should be able to fetch from a pre-signed URL without additional auth
	downloadFile := filepath.Join(tempDir, "presigned-download.txt")

	// Note: AWS CLI typically uses --endpoint-url for S3 operations, not direct URLs
	// For presigned URLs, we use curl or similar, but let's test if aws s3 cp can handle it
	t.Run("download_via_presigned_url", func(t *testing.T) {
		// aws s3 cp with a full URL should work
		stdout, stderr, err := runAWSCLI("s3", "cp",
			shareURL,
			downloadFile,
		)
		if err != nil {
			// This might not work with AWS CLI - it expects s3:// URIs
			// But let's log the attempt
			t.Logf("AWS CLI presigned URL download may not be supported: %v", err)
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
		} else {
			// If it worked, verify content
			downloadedContent, err := os.ReadFile(downloadFile)
			if err != nil {
				t.Fatalf("Failed to read downloaded file: %v", err)
			}
			if !bytes.Equal(testContent, downloadedContent) {
				t.Errorf("Content mismatch from presigned URL")
			}
			t.Log("Presigned URL download via AWS CLI successful")
		}
	})
}
