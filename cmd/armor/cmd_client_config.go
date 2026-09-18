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
	// client-config specific flags, on client-config's own flag set
	clientConfigFlags := flag.NewFlagSet("client-config", flag.ExitOnError)
	clientConfigFlags.StringVar(&forFlag, "for", "", "Tool to generate config for: aws-cli, rclone, boto3, duckdb, litestream, barman (required)")
	clientConfigFlags.StringVar(&endpointFlag, "endpoint", "", "ARMOR endpoint URL (e.g., http://localhost:9000) (required)")
	clientConfigFlags.StringVar(&bucketFlag, "bucket", "", "Bucket name (optional, for inclusion in config)")
	clientConfigFlags.StringVar(&credentialFlag, "credential", "", "Named credential to reference (optional, for inclusion in config)")

	registerCommand(Command{
		Name:        "client-config",
		Description: "Generate known-good client configuration for S3-compatible tools (aws-cli, rclone, boto3, duckdb, litestream, barman)",
		Flags:       clientConfigFlags,
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

// exit is os.Exit by default; tests override it (to a panic-and-recover
// stand-in) so a bad-flags path can be asserted without killing the test
// binary. Shared across the package's subcommands (e.g. cmd_migrate.go),
// not just client-config's own.
var exit = os.Exit

// multipartClientNotes is the per-client behavior line embedded in every
// client-config output: what the tool's default concurrency does against each
// write format, so the "clients work without modification" claim is stated
// (and kept honest) in the config the operator actually pastes.
var multipartClientNotes = map[string]string{
	"aws-cli":    "aws s3 cp default concurrency works unmodified: the short final part usually completes first; on v2 it is deferred with 503 SlowDown and the CLI retries transparently. No multipart_chunksize tuning required.",
	"rclone":     "rclone's default --s3-upload-concurrency 4 works unmodified; --s3-chunk-size tuning is optional on v2, not required.",
	"boto3":      "boto3's TransferConfig defaults work unmodified: parts upload concurrently out-of-order, and the SDK retries 5xx (which covers v2's SlowDown deferral).",
	"duckdb":     "DuckDB/httpfs is a reader (GET / Range / HEAD) and is unaffected by the multipart upload contract on either format.",
	"litestream": "litestream's multipart snapshots work unmodified on both formats; no serial knob exists or is needed.",
	"barman":     "barman's tar-aligned parts (chunk_size + N*512) are accepted: ADR-011 non-uniform mode on v2; no contract on v3.",
}

// multipartContractBlock renders the multipart part-size and ordering
// contract the server's configured write format currently enforces, together
// with how this client behaves against it — the per-tool twin of
// docs/multipart-client-compatibility.md. comment is the tool's comment
// prefix ("#" for ini/env configs, "--" for DuckDB SQL).
func multipartContractBlock(formatVersion int, tool, comment string) string {
	var sb strings.Builder
	if formatVersion == 2 {
		fmt.Fprintf(&sb, "%s Multipart (write format v2, legacy; ADR-015): part 1 pins the uniform\n", comment)
		fmt.Fprintf(&sb, "%s part size P; non-final parts must be exactly P, the final part any size.\n", comment)
		fmt.Fprintf(&sb, "%s A part numbered >1 arriving before part 1 is deferred with retryable\n", comment)
		fmt.Fprintf(&sb, "%s 503 SlowDown -- default client retry clears it. No tuning required.\n", comment)
	} else {
		fmt.Fprintf(&sb, "%s Multipart (write format v3, ARMOR default): no part-order or part-size\n", comment)
		fmt.Fprintf(&sb, "%s contract -- any part sizes, any order, any concurrency. Only B2's own\n", comment)
		fmt.Fprintf(&sb, "%s rule applies: non-final parts must be at least 5 MiB.\n", comment)
	}
	if note := multipartClientNotes[tool]; note != "" {
		fmt.Fprintf(&sb, "%s %s\n", comment, note)
	}
	fmt.Fprintf(&sb, "%s Tested compatibility matrix: docs/multipart-client-compatibility.md\n", comment)
	return sb.String()
}

// clientConfig generates and prints a known-good configuration for the specified tool
func clientConfig(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		exit(2)
	}

	// Validate required flags
	if forFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -for is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		fmt.Fprintf(os.Stderr, "Available tools: aws-cli, rclone, boto3, duckdb, litestream, barman\n")
		exit(2)
	}

	if endpointFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -endpoint is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor client-config -for <tool> -endpoint <url> [flags]\n")
		exit(2)
	}

	// Load config to get format write version
	cfg, err := config.Load()
	if err != nil {
		// If config fails to load, we can still generate configs but warn about format version
		fmt.Fprintf(os.Stderr, "Warning: could not load ARMOR config: %v\n", err)
		fmt.Fprintf(os.Stderr, "Warning: format write version unknown; assuming version 3 constraints\n")
		cfg = &config.Config{}
		cfg.FormatWriteVersion = 3 // Default
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
		exit(2)
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
	fmt.Fprintf(&sb, "endpoint_url = %s\n", endpoint)

	// AWS CLI always needs path-style addressing with ARMOR (B2 doesn't support virtual-hosted style)
	sb.WriteString("s3 =\n")
	sb.WriteString("    addressing_style = path\n")

	// Region is required by AWS CLI but unused by ARMOR - use a placeholder
	sb.WriteString("region = us-east-1\n\n")

	sb.WriteString("# Credentials (set these via 'aws configure set ...' or environment variables)\n")
	if credential != "" {
		fmt.Fprintf(&sb, "# Using named credential: %s\n", credential)
		sb.WriteString("# Set these environment variables instead:\n")
		fmt.Fprintf(&sb, "export AWS_ACCESS_KEY_ID=$(armor get-credential %s key)\n", credential)
		fmt.Fprintf(&sb, "export AWS_SECRET_ACCESS_KEY=$(armor get-credential %s secret)\n", credential)
	} else {
		sb.WriteString("# ARMOR_AUTH_ACCESS_KEY_ID\n")
		sb.WriteString("# ARMOR_AUTH_SECRET_KEY\n")
	}
	sb.WriteString("\n")

	sb.WriteString("# Test the configuration:\n")
	sb.WriteString("# aws --profile armor s3 ls")
	if bucket != "" {
		fmt.Fprintf(&sb, " s3://%s", bucket)
	}
	sb.WriteString("\n\n")

	sb.WriteString(multipartContractBlock(formatVersion, "aws-cli", "#"))

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

	fmt.Fprintf(&sb, "[%s]\n", remoteName)
	sb.WriteString("type = s3\n")
	fmt.Fprintf(&sb, "endpoint = %s\n", endpoint)
	sb.WriteString("provider = Other\n")
	sb.WriteString("s3_force_path_style = true\n")
	sb.WriteString("region = us-east-1\n")
	sb.WriteString("location_constraint = \n")
	sb.WriteString("acl = private\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		fmt.Fprintf(&sb, "# Using named credential: %s\n", credential)
		sb.WriteString("# Set access_key_id and secret_access_key from the credential:\n")
		sb.WriteString("# armor get-credential ") // In a real implementation, this would be a subcommand
	} else {
		sb.WriteString("# Set these to your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY values:\n")
	}
	sb.WriteString("access_key_id = YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("secret_access_key = YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Test the configuration:\n")
	fmt.Fprintf(&sb, "# rclone lsd %s:\n", remoteName)
	if bucket != "" {
		fmt.Fprintf(&sb, "# rclone ls %s:%s\n", remoteName, bucket)
	}
	sb.WriteString("\n")

	sb.WriteString(multipartContractBlock(formatVersion, "rclone", "#"))

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
	fmt.Fprintf(&sb, "endpoint_url = %q\n", endpoint)
	sb.WriteString("\n")

	sb.WriteString("# Credentials (set these environment variables)\n")
	if credential != "" {
		fmt.Fprintf(&sb, "# Using named credential: %s\n", credential)
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
		fmt.Fprintf(&sb, "# bucket = %q\n", bucket)
	} else {
		sb.WriteString("# bucket = 'your-bucket'\n")
	}
	sb.WriteString("# response = s3.list_objects_v2(Bucket=bucket)\n")
	sb.WriteString("# for obj in response.get('Contents', []):\n")
	sb.WriteString("#     print(obj['Key'])\n")
	sb.WriteString("\n")

	sb.WriteString(multipartContractBlock(formatVersion, "boto3", "#"))

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
	fmt.Fprintf(&sb, "SET s3_endpoint = '%s';\n", endpoint)
	sb.WriteString("SET region = 'us-east-1';  -- Required but unused by ARMOR\n\n")

	sb.WriteString("# Configure credentials\n")
	if credential != "" {
		fmt.Fprintf(&sb, "-- Using named credential: %s\n", credential)
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
		fmt.Fprintf(&sb, "-- SELECT * FROM read_parquet('s3://%s/path/to/file.parquet');\n", bucket)
	} else {
		sb.WriteString("-- SELECT * FROM read_parquet('s3://bucket/path/to/file.parquet');\n")
	}
	sb.WriteString("\n")

	// DuckDB accesses objects read-only via range GETs (httpfs), so the
	// multipart upload contract never applies to it — the emitted block says
	// so explicitly rather than leaving the operator to infer it.
	sb.WriteString(multipartContractBlock(formatVersion, "duckdb", "--"))

	return sb.String()
}

// litestreamConfig generates Litestream configuration
func litestreamConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Litestream configuration for ARMOR\n")
	sb.WriteString("# Add to litestream.yml or use as environment variables\n\n")

	sb.WriteString("# Litestream environment variables\n")
	fmt.Fprintf(&sb, "LITESTREAM_ENDPOINT=%s\n", endpoint)
	sb.WriteString("LITESTREAM_REGION=us-east-1  # Required but unused by ARMOR\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		fmt.Fprintf(&sb, "# Using named credential: %s\n", credential)
	}
	sb.WriteString("# Set these from your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY\n")
	sb.WriteString("LITESTREAM_ACCESS_KEY_ID=YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("LITESTREAM_SECRET_ACCESS_KEY=YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Or in litestream.yml:\n")
	sb.WriteString("dbs:\n")
	sb.WriteString("  - path: /path/to/db.sqlite\n")
	sb.WriteString("    replicas:\n")
	sb.WriteString("      - type: s3\n")
	fmt.Fprintf(&sb, "        endpoint: %s\n", endpoint)
	sb.WriteString("        bucket: YOUR_BUCKET\n")
	sb.WriteString("        region: us-east-1\n")
	sb.WriteString("        access-key-id: YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("        secret-access-key: YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString(multipartContractBlock(formatVersion, "litestream", "#"))

	return sb.String()
}

// barmanConfig generates Barman Cloud configuration
func barmanConfig(endpoint, bucket, credential string, formatVersion int) string {
	var sb strings.Builder

	sb.WriteString("# Barman Cloud configuration for ARMOR\n")
	sb.WriteString("# For PostgreSQL backup via barman-cloud-backup / barman-cloud-wal-archive\n\n")

	sb.WriteString("# Environment variables for Barman Cloud\n")
	fmt.Fprintf(&sb, "export AWS_ENDPOINT_URL=%s\n", endpoint)
	sb.WriteString("export AWS_REGION=us-east-1  # Required but unused by ARMOR\n\n")

	sb.WriteString("# Credentials\n")
	if credential != "" {
		fmt.Fprintf(&sb, "# Using named credential: %s\n", credential)
	}
	sb.WriteString("# Set these from your ARMOR_AUTH_ACCESS_KEY_ID and ARMOR_AUTH_SECRET_KEY\n")
	sb.WriteString("export AWS_ACCESS_KEY_ID=YOUR_ACCESS_KEY_ID\n")
	sb.WriteString("export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_ACCESS_KEY\n\n")

	sb.WriteString("# Barman cloud commands\n")
	sb.WriteString("# barman-cloud-backup list <server> <bucket>\n")
	sb.WriteString("# barman-cloud-backup backup <server> <bucket> <postgres_data_dir>\n")
	sb.WriteString("# barman-cloud-wal-archive --endpoint-url $AWS_ENDPOINT_URL <bucket> <server>\n")
	sb.WriteString("# barman-cloud-wal-restore --endpoint-url $AWS_ENDPOINT_URL <bucket> <server> <wal_file> <dest_dir>\n\n")

	sb.WriteString(multipartContractBlock(formatVersion, "barman", "#"))

	return sb.String()
}
