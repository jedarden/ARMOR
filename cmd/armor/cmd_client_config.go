// cmd_client_config.go implements the 'armor client-config' subcommand for generating tool-specific configurations
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jedarden/armor/internal/config"
)

func init() {
	registerCommand(Command{
		Name:        "client-config",
		Description: "Generate known-good client configuration for S3-compatible tools (aws-cli, rclone, boto3, duckdb, litestream, barman)",
		Func:        clientConfig,
	})
}

// client-config flags
var (
	forFlag        string
	endpointFlag   string
	bucketFlag     string
	credentialFlag string
)

func init() {
	// client-config specific flags
	flag.StringVar(&forFlag, "for", "", "Tool to generate config for: aws-cli, rclone, boto3, duckdb, litestream, barman (required)")
	flag.StringVar(&endpointFlag, "endpoint", "", "ARMOR endpoint URL (e.g., http://localhost:9000) (required)")
	flag.StringVar(&bucketFlag, "bucket", "", "Bucket name (optional, for inclusion in config)")
	flag.StringVar(&credentialFlag, "credential", "", "Named credential to reference (optional, for inclusion in config)")
}

// clientConfig generates and prints a known-good configuration for the specified tool
func clientConfig() {
	// Parse flags
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		os.Exit(2)
	}

	// Validate required flags
	if forFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -for is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		fmt.Fprintf(os.Stderr, "Available tools: aws-cli, rclone, boto3, duckdb, litestream, barman\n")
		os.Exit(2)
	}

	if endpointFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -endpoint is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		os.Exit(2)
	}

	// Load config to get format write version
	cfg, err := config.Load()
	if err != nil {
		// If config fails to load, we can still generate configs but warn about format version
		fmt.Fprintf(os.Stderr, "Warning: could not load ARMOR config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Warning: format write version unknown; assuming version 2 constraints\n")
		cfg = &config.Config{}
		cfg.FormatWriteVersion = 2 // Default
	}

	// Generate config for the requested tool
	var output string
	tool := strings.ToLower(forFlag)

	switch tool {
	case "aws-cli", "awscli":
		output = awsCLIConfig(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	case "rclone":
		output = rcloneConfig(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	case "boto3":
		output = boto3Config(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	case "duckdb":
		output = duckDBConfig(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	case "litestream":
		output = litestreamConfig(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	case "barman":
		output = barmanConfig(endpointFlag, bucketFlag, credentialFlag, cfg.FormatWriteVersion)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown tool %q\n", tool)
		fmt.Fprintf(os.Stderr, "Available tools: aws-cli, rclone, boto3, duckdb, litestream, barman\n")
		os.Exit(2)
	}

	fmt.Print(output)
}

// awsCLIConfig generates AWS CLI configuration
func awsCLIConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# AWS CLI configuration for ARMOR\n")
	sb.WriteString("# Run: aws configure --profile armor\n")
	sb.WriteString("# Then paste the appropriate values below\n\n")

	sb.WriteString("[profile armor]\n")
	sb.WriteString(fmt.Sprintf("endpoint_url = %s\n", endpoint))

	// AWS CLI always needs path-style addressing with ARMOR (B2 doesn't support virtual-hosted style)
	sb.WriteString("s3 =\n")
	sb.WriteString("    addressing_style = path\n")

	// Region is required by AWS CLI but unused by ARMOR - use a placeholder
	sb.WriteString("region = us-east-1\n\n")

	sb.WriteString("# Credentials (set these via 'aws configure set ...' or environment variables)\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("# Using named credential: %s\n", credential))
		sb.WriteString("# Set these environment variables instead:\n")
		sb.WriteString(fmt.Sprintf("export AWS_ACCESS_KEY_ID=$(armor get-credential %s key)\n", credential))
		sb.WriteString(fmt.Sprintf("export AWS_SECRET_ACCESS_KEY=$(armor get-credential %s secret)\n", credential))
	} else {
		sb.WriteString("# ARMOR_AUTH_ACCESS_KEY_ID\n")
		sb.WriteString("# ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("\n")

	sb.WriteString("# Test the configuration:\n")
	sb.WriteString("# aws --profile armor s3 ls")
	if bucket != "" {
		sb.WriteString(fmt.Sprintf(" s3://%s", bucket))
	}
	sb.WriteString("\n")

	// Format version 2 requires multipart constraints
	if formatVersion == 2 {
		sb.WriteString("\n# ARMOR format version 2: multipart upload constraints\n")
		sb.WriteString("# Minimum part size: 5 MB (S3 requirement)\n")
		sb.WriteString("# Part size must be a multiple of ARMOR block size (64 KB)\n")
		sb.WriteString("# Recommended configuration for multipart uploads:\n")
		sb.WriteString("aws configure set default.s3.max_bandwidth 104857600  # 100 MB/s throttle (optional)\n")
		sb.WriteString("aws configure set default.s3.multipart_threshold 67108864  # 64 MB (block-aligned: 64 * 1024 * 1024 bytes)\n")
		sb.WriteString("aws configure set default.s3.multipart_chunksize 67108864  # 64 MB (block-aligned)\n")
		sb.WriteString("\n")
		sb.WriteString("# Or via environment variables:\n")
		sb.WriteString("# export AWS_DEFAULT_S3_MULTIPART_THRESHOLD=67108864\n")
		sb.WriteString("# export AWS_DEFAULT_S3_MULTIPART_CHUNKSIZE=67108864\n")
	}

	return sb.String()
}

// rcloneConfig generates rclone configuration
func rcloneConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Rclone configuration for ARMOR\n")
	sb.WriteString("# Add this section to ~/.config/rclone/rclone.conf\n\n")

	remoteName := "armor"
	if bucket != "" {
		// Create a remote name that includes the bucket hint
		remoteName = fmt.Sprintf("%s-armor", bucket)
	}

	sb.WriteString(fmt.Sprintf("[%s]\n", remoteName))
	sb.WriteString("type = s3\n")
	sb.WriteString(fmt.Sprintf("endpoint = %s\n", endpoint))
	sb.WriteString("provider = Other\n")
	sb.WriteString("s3_force_path_style = true\n")
	sb.WriteString("region = us-east-1\n")
	sb.WriteString("location_constraint = \n")
	sb.WriteString("acl = private\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("# Using named credential: %s\n", credential))
		sb.WriteString("# Set access_key_id and secret_access_key from the credential:\n")
		sb.WriteString("# armor get-credential ") // In a real implementation, this would be a subcommand
	} else {
		sb.WriteString("# Set these to your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY values:\n")
	}
	sb.WriteString("access_key_id = YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("secret_access_key = YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Test the configuration:\n")
	sb.WriteString(fmt.Sprintf("# rclone lsd %s:\n", remoteName))
	if bucket != "" {
		sb.WriteString(fmt.Sprintf("# rclone ls %s:%s\n", remoteName, bucket))
	}
	sb.WriteString("\n")

	// Format version 2 requires multipart constraints
	if formatVersion == 2 {
		sb.WriteString("# ARMOR format version 2: multipart upload constraints\n")
		sb.WriteString("# Rclone uses a fixed 5 MB chunk size by default which is not block-aligned\n")
		sb.WriteString("# Override with a block-aligned chunk size (multiple of 64 KB):\n")
		sb.WriteString("# --s3-upload-concurrency 4 \\\n")
		sb.WriteString("# --s3-chunk-size 67108864 \\\n") // 64 MB, block-aligned
		sb.WriteString("# --s3-disable-checksum \\\n")   // ARMOR computes its own HMAC
		sb.WriteString("\n")
		sb.WriteString("# Example rclone command with these settings:\n")
		sb.WriteString(fmt.Sprintf("# rclone copy /local/path %s:%s --s3-chunk-size 67108864\n", remoteName, bucketOrPlaceholder(bucket)))
	}

	return sb.String()
}

// boto3Config generates boto3 configuration
func boto3Config(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Boto3 configuration for ARMOR\n")
	sb.WriteString("# Python code example\n\n")

	sb.WriteString("import boto3\n")
	sb.WriteString("import os\n\n")

	sb.WriteString("# ARMOR endpoint configuration\n")
	sb.WriteString(fmt.Sprintf("endpoint_url = %q\n", endpoint))
	sb.WriteString("\n")

	sb.WriteString("# Credentials (set these environment variables)\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("# Using named credential: %s\n", credential))
		sb.WriteString("# Get these from your ARMOR deployment:\n")
	} else {
		sb.WriteString("# ARMOR_AUTH_ACCESS_KEY_ID\n")
		sb.WriteString("# ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("access_key = os.environ.get('AWS_ACCESS_KEY_ID')\n")
	sb.WriteString("secret_key = os.environ.get('AWS_SECRET_ACCESS_KEY')\n")
	sb.WriteString("\n")

	sb.WriteString("# Create S3 client with path-style addressing\n")
	sb.WriteString("s3 = boto3.client('s3',\n")
	sb.WriteString("    endpoint_url=endpoint_url,\n")
	sb.WriteString("    aws_access_key_id=access_key,\n")
	sb.WriteString("    aws_secret_access_key=secret_key,\n")
	sb.WriteString("    region_name='us-east-1',  # Required but unused by ARMOR\n")
	sb.WriteString("    config=boto3.session.Config(s3={'addressing_style': 'path'})\n")
	sb.WriteString(")\n\n")

	sb.WriteString("# Example usage\n")
	if bucket != "" {
		sb.WriteString(fmt.Sprintf("# bucket = %q\n", bucket))
	} else {
		sb.WriteString("# bucket = 'your-bucket'\n")
	}
	sb.WriteString("# response = s3.list_objects_v2(Bucket=bucket)\n")
	sb.WriteString("# for obj in response.get('Contents', []):\n")
	sb.WriteString("#     print(obj['Key'])\n")
	sb.WriteString("\n")

	// Format version 2 requires multipart constraints
	if formatVersion == 2 {
		sb.WriteString("# ARMOR format version 2: multipart upload constraints\n")
		sb.WriteString("# Configure multipart upload with block-aligned part sizes\n")
		sb.WriteString("from boto3.s3.transfer import TransferConfig\n\n")

		sb.WriteString("# TransferConfig for block-aligned multipart uploads\n")
		sb.WriteString("transfer_config = TransferConfig(\n")
		sb.WriteString("    multipart_threshold=64 * 1024 * 1024,  # 64 MB (block-aligned)\n")
		sb.WriteString("    multipart_chunksize=64 * 1024 * 1024,  # 64 MB (block-aligned)\n")
		sb.WriteString("    max_concurrency=10,\n")
		sb.WriteString("    use_threads=True\n")
		sb.WriteString(")\n\n")

		sb.WriteString("# Use with upload_file:\n")
		sb.WriteString("# s3.upload_file('local_file', bucket, 'key', Config=transfer_config)\n")
	}

	return sb.String()
}

// duckDBConfig generates DuckDB configuration
func duckDBConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# DuckDB configuration for ARMOR\n")
	sb.WriteString("# SQL commands for httpfs extension\n\n")

	sb.WriteString("INSTALL httpfs;\n")
	sb.WriteString("LOAD httpfs;\n\n")

	sb.WriteString("# Set S3 endpoint and region\n")
	sb.WriteString(fmt.Sprintf("SET s3_endpoint = '%s';\n", endpoint))
	sb.WriteString("SET region = 'us-east-1';  -- Required but unused by ARMOR\n\n")

	sb.WriteString("# Configure credentials\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("-- Using named credential: %s\n", credential))
		sb.WriteString("-- Set these environment variables instead:\n")
		sb.WriteString("-- SET s3_access_key_id = $AWS_ACCESS_KEY_ID;\n")
		sb.WriteString("-- SET s3_secret_access_key = $AWS_SECRET_ACCESS_KEY;\n")
	} else {
		sb.WriteString("-- Set these to your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("SET s3_access_key_id = 'YOUR_ACCESS_KEY_ID';\n")
	sb.WriteString("SET s3_secret_access_key = 'YOUR_SECRET_ACCESS_KEY';\n\n")

	sb.WriteString("-- Enable path-style addressing (required for B2/ARMOR)\n")
	sb.WriteString("SET s3_use_ssl = true;\n")
	sb.WriteString("SET s3_url_style = 'path';  -- or 'virtual' for path-style\n\n")

	sb.WriteString("-- Example: Read a Parquet file\n")
	if bucket != "" {
		sb.WriteString(fmt.Sprintf("-- SELECT * FROM read_parquet('s3://%s/path/to/file.parquet');\n", bucket))
	} else {
		sb.WriteString("-- SELECT * FROM read_parquet('s3://bucket/path/to/file.parquet');\n")
	}
	sb.WriteString("\n")

	// Note: DuckDB doesn't do multipart uploads in the same way
	// It uses range reads for GET which work fine with ARMOR
	// For large exports, DuckDB splits into multiple files which are uploaded individually
	if formatVersion == 2 {
		sb.WriteString("-- ARMOR format version 2: Large file exports\n")
		sb.WriteString("-- DuckDB's COPY TO exports large files as single PUTs or multipart uploads\n")
		sb.WriteString("-- For very large files (>64 MB), DuckDB may use multipart uploads\n")
		sb.WriteString("-- ARMOR format version 2 requires block-aligned part sizes (multiple of 64 KB)\n")
		sb.WriteString("-- DuckDB's default part sizes may need adjustment for optimal compatibility\n")
		sb.WriteString("-- Consider exporting smaller chunks or using format version 3 for large exports\n")
	}

	return sb.String()
}

// litestreamConfig generates Litestream configuration
func litestreamConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Litestream configuration for ARMOR\n")
	sb.WriteString("# Add to litestream.yml or use as environment variables\n\n")

	sb.WriteString("# Litestream environment variables\n")
	sb.WriteString(fmt.Sprintf("LITESTREAM_ENDPOINT=%s\n", endpoint))
	sb.WriteString("LITESTREAM_REGION=us-east-1  # Required but unused by ARMOR\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("# Using named credential: %s\n", credential))
		sb.WriteString("# Set these from your ARMOR deployment:\n")
	} else {
		sb.WriteString("# Set these from your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("LITESTREAM_ACCESS_KEY_ID=YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("LITESTREAM_SECRET_ACCESS_KEY=YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Or in litestream.yml:\n")
	sb.WriteString("dbs:\n")
	sb.WriteString("  - path: /path/to/db.sqlite\n")
	sb.WriteString("    replicas:\n")
	sb.WriteString("      - type: s3\n")
	sb.WriteString(fmt.Sprintf("        endpoint: %s\n", endpoint))
	sb.WriteString("        bucket: YOUR_BUCKET\n")
	sb.WriteString("        region: us-east-1\n")
	sb.WriteString("        access-key-id: YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("        secret-access-key: YOUR_SECRET_ACCESS_KEY\n\n")

	// Format version 2 requires multipart constraints
	if formatVersion == 2 {
		sb.WriteString("# ARMOR format version 2: multipart upload constraints\n")
		sb.WriteString("# Litestream uploads WAL files as multipart uploads\n")
		sb.WriteString("# Default snapshot size is 4 MB which is below the 5 MB minimum\n")
		sb.WriteString("# Increase snapshot size to a block-aligned value:\n")
		sb.WriteString("#\n")
		sb.WriteString("# litestream.yml:\n")
		sb.WriteString("# dbs:\n")
		sb.WriteString("#   - path: /path/to/db.sqlite\n")
		sb.WriteString("#     snapshots:\n")
		sb.WriteString("#       - snapshot-interval: 1h\n")
		sb.WriteString("#         snapshot-size-mb: 64  # Block-aligned: 64 MB (multiple of 64 KB)\n")
		sb.WriteString("#\n")
		sb.WriteString("# Or via environment variable:\n")
		sb.WriteString("# export LITESTREAM_SNAPSHOT_SIZE_MB=64\n")
	}

	return sb.String()
}

// barmanConfig generates Barman configuration
func barmanConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Barman Cloud configuration for ARMOR\n")
	sb.WriteString("# For PostgreSQL backup via barman-cloud-backup / barman-cloud-wal-archive\n\n")

	sb.WriteString("# Environment variables for Barman Cloud\n")
	sb.WriteString(fmt.Sprintf("export AWS_ENDPOINT_URL=%s\n", endpoint))
	sb.WriteString("export AWS_REGION=us-east-1  # Required but unused by ARMOR\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		sb.WriteString(fmt.Sprintf("# Using named credential: %s\n", credential))
		sb.WriteString("# Set these from your ARMOR deployment:\n")
	} else {
		sb.WriteString("# Set these from your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Barman cloud commands\n")
	sb.WriteString("# barman-cloud-backup list <server> <bucket>\n")
	sb.WriteString("# barman-cloud-backup backup <server> <bucket> <postgres_data_dir>\n")
	sb.WriteString("# barman-cloud-wal-archive --endpoint-url $AWS_ENDPOINT_URL <bucket> <server>\n")
	sb.WriteString("# barman-cloud-wal-restore --endpoint-url $AWS_ENDPOINT_URL <bucket> <server> <wal_file> <dest_dir>\n")
	sb.WriteString("\n")

	// Format version 2 has special constraints for Barman
	if formatVersion == 2 {
		sb.WriteString("# ARMOR format version 2: Multipart upload constraints for Barman\n")
		sb.WriteString("#\n")
		sb.WriteString("# Barman's chunk size + 512-byte metadata pattern breaks uniform-part-size contract\n")
		sb.WriteString("# This causes InvalidPartSize errors when backups exceed the single-part threshold\n")
		sb.WriteString("#\n")
		sb.WriteString("# WORKAROUND (format version 2):\n")
		sb.WriteString("# 1. Use large chunk size to keep backups in single-part uploads:\n")
		sb.WriteString("#    barman-cloud-backup-backup ...\n")
		sb.WriteString(fmt.Sprintf("#      --endpoint-url=%s\n", endpoint))
		sb.WriteString("#      --chunk-size=1024  # 1024 MB (1 GB), block-aligned (1024 * 1024 * 1024 is multiple of 64 KB)\n")
		sb.WriteString("#\n")
		sb.WriteString("# 2. Limit backup size to stay below single-part threshold (~5 GB)\n")
		sb.WriteString("# 3. Or upgrade to ARMOR format version 3 which supports non-uniform part sizes\n")
		sb.WriteString("#\n")
		sb.WriteString("# Minimum chunk size: 5 MB (S3 requirement)\n")
		sb.WriteString("# Recommended: 1024 MB (1 GB) - block-aligned and keeps most backups in one part\n")
		sb.WriteString("\n")

		sb.WriteString("# For WAL archive (small files), default settings work fine:\n")
		sb.WriteString("# barman-cloud-wal-archive ... --jobs 1  # Sequential uploads avoid part-order issues\n")
	}

	return sb.String()
}

// bucketOrPlaceholder returns the bucket or a placeholder string
func bucketOrPlaceholder(bucket string) string {
	if bucket != "" {
		return bucket
	}
	return "your-bucket"
}
