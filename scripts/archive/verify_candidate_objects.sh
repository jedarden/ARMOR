#!/usr/bin/env bash
# Verify candidate objects via armor-decrypt to identify corruption
set -euo pipefail

INPUT_FILE="/home/coding/ARMOR/intermediate/filtered_objects.json"
OUTPUT_FILE="/home/coding/ARMOR/scratch/corruption-inventory-raw.json"
CONCURRENCY=8  # Process 8 objects in parallel

# Check if MEK is available
if [[ -z "${ARMOR_MEK:-}" ]]; then
    echo "Error: ARMOR_MEK environment variable not set" >&2
    exit 1
fi

# Check if armor-decrypt binary exists
ARMOR_DECRYPT="/home/coding/ARMOR/cmd/armor-decrypt/armor-decrypt"
if [[ ! -f "$ARMOR_DECRYPT" ]]; then
    echo "Building armor-decrypt..." >&2
    cd /home/coding/ARMOR
    go build -o "$ARMOR_DECRYPT" cmd/armor-decrypt/main.go
fi

# Initialize output file
echo '{"verified_objects": []}' > "$OUTPUT_FILE.tmp"

# Function to verify a single object
verify_object() {
    local bucket="$1"
    local key="$2"
    local size_bytes="$3"
    local created_at="$4"
    local affected_versions="$5"

    local output
    local exit_code

    # Attempt decryption with verbose output to stderr
    output=$("$ARMOR_DECRYPT" \
        -mek "$ARMOR_MEK" \
        -input "b2://${bucket}/${key}" \
        -output /dev/null \
        -v 2>&1)
    exit_code=$?

    local status="ERROR"
    local error_message=""

    if [[ $exit_code -eq 0 ]]; then
        status="OK"
        error_message=""
    elif [[ $exit_code -eq 1 ]]; then
        # Determine if it's corruption or another error
        if echo "$output" | grep -qiE "corrupt|invalid|hmac|checksum|decrypt"; then
            status="CORRUPTED"
        else
            status="ERROR"
        fi
        error_message=$(echo "$output" | head -1 | sed 's/"/\\"/g')
    elif [[ $exit_code -eq 2 ]]; then
        status="ERROR"
        error_message="Usage error: $(echo "$output" | head -1 | sed 's/"/\\"/g')"
    else
        status="ERROR"
        error_message="Unexpected exit code $exit_code: $(echo "$output" | head -1 | sed 's/"/\\"/g')"
    fi

    # Output JSON result
    jq -n \
        --arg bucket "$bucket" \
        --arg key "$key" \
        --arg size "$size_bytes" \
        --arg created "$created_at" \
        --arg versions "$affected_versions" \
        --arg status "$status" \
        --arg error "$error_message" \
        '{
            bucket: $bucket,
            key: $key,
            size_bytes: ($size | tonumber),
            created_at: $created,
            affected_armor_versions: ($versions | split(",") | map(gsub("^\\s|\\s$"; ""))),
            status: $status,
            error_message: (if $error == "" then null else $error end)
        }' >> "$OUTPUT_FILE.results"

    echo "✓ ${bucket}/${key}: ${status}" >&2
}

export ARMOR_MEK ARMOR_DECRYPT OUTPUT_FILE
export -f verify_object

# Process objects in parallel
echo "Starting verification of $(jq 'length' "$INPUT_FILE") objects..." >&2
echo "" > "$OUTPUT_FILE.results"

jq -c '.[]' "$INPUT_FILE" | while IFS= read -r obj; do
    # Extract fields
    bucket=$(echo "$obj" | jq -r '.bucket')
    key=$(echo "$obj" | jq -r '.key')
    size=$(echo "$obj" | jq -r '.size_bytes')
    created=$(echo "$obj" | jq -r '.created_at')
    versions=$(echo "$obj" | jq -r '.affected_armor_versions | join(",")')

    # Run verification in background with semaphore control
    (
        verify_object "$bucket" "$key" "$size" "$created" "$versions"
    ) &

    # Control concurrency
    if [[ $(jobs -r | wc -l) -ge $CONCURRENCY ]]; then
        wait -n
    fi
done

# Wait for all background jobs
wait

# Compile results
jq -s '{verified_objects: .}' "$OUTPUT_FILE.results" > "$OUTPUT_FILE"
rm -f "$OUTPUT_FILE.results" "$OUTPUT_FILE.tmp"

# Print summary
echo "" >&2
echo "Verification complete. Results saved to $OUTPUT_FILE" >&2
echo "" >&2
echo "Summary:" >&2
jq -r '
    "  Total objects: \(.verified_objects | length)",
    "  OK (recoverable): \([.verified_objects[] | select(.status == "OK")] | length)",
    "  CORRUPTED (unrecoverable): \([.verified_objects[] | select(.status == "CORRUPTED")] | length)",
    "  ERROR (verification failed): \([.verified_objects[] | select(.status == "ERROR")] | length)"
' "$OUTPUT_FILE" >&2
