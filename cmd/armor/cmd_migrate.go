// cmd_migrate.go implements the 'armor migrate' subcommand for format migration.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/server"
)

const (
	migrationPath           = "/admin/format/migrate"
	migrationRequestTimeout = 30 * time.Second
	migrationPollInterval   = 2 * time.Second
)

func init() {
	migrateFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	migrateFlags.StringVar(&adminURLFlag, "admin-url", "", "Admin API endpoint (required, e.g., http://127.0.0.1:9001)")
	migrateFlags.BoolVar(&dryRunFlag, "dry-run", false, "Dry run: verify objects can be migrated without making changes")
	migrateFlags.BoolVar(&inventoryOnlyFlag, "inventory-only", false, "Inventory only: classify object metadata without reading bodies or writing migrated objects")
	migrateFlags.StringVar(&targetFlag, "target", "3", "Target format version to migrate to (e.g., v3 or 3); must match the server's configured format write version")
	migrateFlags.StringVar(&includeFlag, "include", "1,2", "Comma-separated source versions to migrate (default: 1,2 for a V3 migration)")
	migrateFlags.IntVar(&concurrencyFlag, "concurrency", 0, "Number of concurrent workers (default: server-side default)")
	migrateFlags.BoolVar(&watchFlag, "watch", false, "Watch mode: poll progress until completion")
	migrateFlags.StringVar(&resumeOperationIDFlag, "resume", "", "Resume watching an existing migration operation by ID (does not start a new operation)")
	migrateFlags.StringVar(&operationIDFlag, "operation-id", "", "Alias for -resume")
	migrateFlags.BoolVar(&jsonOutputFlag, "json", false, "Print the completion report as machine-readable JSON on stdout (the human-readable summary still goes to stderr)")

	registerCommand(Command{
		Name:        "migrate",
		Description: "Migrate ARMOR objects to the current encryption format (client of /admin/format/migrate)",
		Flags:       migrateFlags,
		Func:        migrate,
	})
}

var (
	adminURLFlag          string
	dryRunFlag            bool
	inventoryOnlyFlag     bool
	targetFlag            string
	includeFlag           string
	concurrencyFlag       int
	watchFlag             bool
	resumeOperationIDFlag string
	operationIDFlag       string
	jsonOutputFlag        bool
)

// MigrationState is the durable state document returned by the admin API.
type MigrationState struct {
	ID                  string   `json:"id"`
	OperationID         string   `json:"operation_id,omitempty"`
	StartTime           string   `json:"start_time"`
	LastUpdated         string   `json:"last_updated"`
	Status              string   `json:"status"`
	TotalObjects        int      `json:"total_objects"`
	ProcessedObjects    int      `json:"processed_objects"`
	SkippedObjects      int      `json:"skipped_objects"`
	FailedObjects       int      `json:"failed_objects"`
	UnknownObjects      int      `json:"-"`
	UnknownFormats      int      `json:"-"`
	UnknownVersions     int      `json:"-"`
	LastKey             string   `json:"last_key"`
	IncludeVersions     []string `json:"include_versions"`
	CurrentWriteVersion uint8    `json:"current_write_version"`
	DryRun              bool     `json:"dry_run"`
	InventoryOnly       bool     `json:"inventory_only,omitempty"`
	Concurrency         int      `json:"concurrency"`
	Failures            []struct {
		Key    string `json:"key"`
		Reason string `json:"reason"`
		Time   string `json:"time"`
	} `json:"failures,omitempty"`
	ErrorMessage   string                       `json:"error_message,omitempty"`
	Classification *server.ObjectClassification `json:"classification,omitempty"`
}

// MigrationResult represents the final result of a migration operation.
type MigrationResult struct {
	TotalObjects     int    `json:"total_objects"`
	ProcessedObjects int    `json:"processed_objects"`
	SkippedObjects   int    `json:"skipped_objects"`
	FailedObjects    int    `json:"failed_objects"`
	UnknownObjects   int    `json:"unknown_objects,omitempty"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message,omitempty"`
	DryRun           bool   `json:"dry_run"`
	InventoryOnly    bool   `json:"inventory_only,omitempty"`
	OperationID      string `json:"operation_id,omitempty"`
}

func migrate(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", fs.Args())
		fmt.Fprintln(os.Stderr, "Usage: armor migrate [flags]")
		exit(2)
	}
	if adminURLFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: -admin-url is required")
		fmt.Fprintln(os.Stderr, "Usage: armor migrate -admin-url <url> [other flags]")
		exit(2)
	}
	adminToken := os.Getenv("ARMOR_ADMIN_TOKEN")
	if adminToken == "" {
		fmt.Fprintln(os.Stderr, "Error: ARMOR_ADMIN_TOKEN environment variable is required")
		exit(1)
	}

	client := migrationHTTPClient()
	endpoint := strings.TrimRight(adminURLFlag, "/") + migrationPath
	operationID := strings.TrimSpace(resumeOperationIDFlag)
	if alias := strings.TrimSpace(operationIDFlag); alias != "" {
		if operationID != "" && operationID != alias {
			fmt.Fprintln(os.Stderr, "Error: -resume and -operation-id name different operations")
			exit(2)
		}
		operationID = alias
	}

	var initial map[string]interface{}
	started := operationID == ""
	if started {
		fmt.Fprintln(os.Stderr, "Starting migration...")
		var err error
		initial, err = postMigration(client, endpoint, migrationStartQuery(), adminToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to start migration: %v\n", err)
			exit(1)
		}
		fmt.Fprintln(os.Stderr, "Migration started successfully.")
		operationID = migrationOperationID(initial)
		if operationID != "" {
			fmt.Fprintf(os.Stderr, "Migration operation started: %s\n", operationID)
		} else {
			fmt.Fprintln(os.Stderr, "Migration operation started (legacy server returned no operation ID)")
		}

		if isMigrationTerminal(migrationStatus(initial)) {
			if err := reportMigration(initial, "migration start"); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				exit(1)
			}
			return
		}
	} else {
		fmt.Fprintf(os.Stderr, "Resuming migration operation: %s\n", operationID)
	}

	// A resume has no start response to report and must attach to durable state.
	if started && !watchFlag {
		if jsonOutputFlag {
			if err := writeJSON(os.Stdout, initial); err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to encode operation report: %v\n", err)
				exit(1)
			}
		}
		fmt.Fprintln(os.Stderr, "Migration accepted; use -watch or -resume with this operation ID to follow it.")
		return
	}

	if err := watchMigration(client, endpoint, adminToken, operationID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: migration watch failed: %v\n", err)
		exit(1)
	}
}

func migrationHTTPClient() *http.Client {
	// The POST creates durable server-side state. The watch loop may run for
	// hours, but every individual start or status request remains bounded.
	return &http.Client{Timeout: migrationRequestTimeout}
}

func migrationStartQuery() url.Values {
	values := url.Values{}
	if dryRunFlag || inventoryOnlyFlag {
		values.Set("dry_run", "true")
	}
	if inventoryOnlyFlag {
		values.Set("inventory_only", "true")
	}
	if targetFlag != "" {
		values.Set("target", targetFlag)
	}
	if includeFlag != "" {
		values.Set("include", includeFlag)
	}
	if concurrencyFlag > 0 {
		values.Set("concurrency", strconv.Itoa(concurrencyFlag))
	}
	return values
}

func postMigration(client *http.Client, endpoint string, values url.Values, token string) (map[string]interface{}, error) {
	if encoded := values.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	resp, err := doRequest(client, http.MethodPost, endpoint, token, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("migration request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var state map[string]interface{}
	if err := json.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return state, nil
}

func migrationStatusURL(endpoint, operationID string) string {
	if operationID == "" {
		return endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint + "?operation_id=" + url.QueryEscape(operationID)
	}
	q := u.Query()
	q.Set("operation_id", operationID)
	u.RawQuery = q.Encode()
	return u.String()
}

func migrationOperationID(m map[string]interface{}) string {
	for _, key := range []string{"operation_id", "operationId", "id"} {
		if id, ok := m[key].(string); ok && strings.TrimSpace(id) != "" {
			return id
		}
	}
	return ""
}

func migrationStatus(m map[string]interface{}) string {
	status, _ := m["status"].(string)
	return strings.ToLower(strings.TrimSpace(status))
}

func isMigrationTerminal(status string) bool {
	switch status {
	case "completed", "completed_with_failures", "failed", "interrupted", "aborted", "cancelled", "canceled", "unknown":
		return true
	default:
		return false
	}
}

func writeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// watchMigration attaches to a durable operation and reconnects after a
// transient request failure. A request timeout never cancels the operation.
func watchMigration(client *http.Client, migrateURL, adminToken, operationID string) error {
	ticker := time.NewTicker(migrationPollInterval)
	defer ticker.Stop()

	lastProcessed, lastFailed, lastUnknown := -1, -1, -1
	lastClassification := ""
	firstPoll := true
	for {
		if !firstPoll {
			<-ticker.C
		}
		firstPoll = false

		resp, err := doRequest(client, http.MethodGet, migrationStatusURL(migrateURL, operationID), adminToken, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: migration status request failed; reconnecting: %v\n", err)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read migration status; reconnecting: %v\n", readErr)
			continue
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			fmt.Fprintf(os.Stderr, "Warning: migration status returned %d; reconnecting: %s\n", resp.StatusCode, strings.TrimSpace(string(body)))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var result interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse status response: %w", err)
		}
		m, ok := result.(map[string]interface{})
		if !ok {
			return fmt.Errorf("status response is not a JSON object")
		}
		if migrationStatus(m) == "no_migration" {
			return fmt.Errorf("migration operation %q is not available", operationID)
		}
		state, err := parseMigrationState(m)
		if err != nil {
			return fmt.Errorf("failed to parse migration state: %w", err)
		}
		if operationID != "" && state.OperationID != "" && state.OperationID != operationID {
			return fmt.Errorf("status response belongs to operation %q, expected %q", state.OperationID, operationID)
		}
		unknown := migrationUnknownObjects(m)
		classificationJSON := ""
		if state.Classification != nil {
			data, _ := json.Marshal(state.Classification)
			classificationJSON = string(data)
		}
		if state.ProcessedObjects != lastProcessed || state.FailedObjects != lastFailed || unknown != lastUnknown || classificationJSON != lastClassification {
			progress := io.Writer(os.Stdout)
			if jsonOutputFlag {
				progress = os.Stderr
			}
			fmt.Fprintf(progress, "Progress: %d/%d processed", state.ProcessedObjects, state.TotalObjects)
			if state.SkippedObjects > 0 {
				fmt.Fprintf(progress, ", %d skipped", state.SkippedObjects)
			}
			if state.FailedObjects > 0 {
				fmt.Fprintf(progress, ", %d failed", state.FailedObjects)
			}
			if unknown > 0 {
				fmt.Fprintf(progress, ", %d unknown", unknown)
			}
			fmt.Fprintf(progress, " (%s)\n", state.Status)
			if state.Classification != nil {
				fmt.Fprint(progress, state.Classification.Summary())
			}
			lastProcessed, lastFailed, lastUnknown = state.ProcessedObjects, state.FailedObjects, unknown
			lastClassification = classificationJSON
		}

		if isMigrationTerminal(state.Status) {
			return reportMigration(m, "migration watch")
		}
		if state.Status == "" {
			return fmt.Errorf("status response has no migration status")
		}
	}
}

func reportMigration(m map[string]interface{}, source string) error {
	state, err := parseMigrationState(m)
	if err != nil {
		return fmt.Errorf("%s response is invalid: %w", source, err)
	}
	status := migrationStatus(m)
	if status == "completed" && state.FailedObjects == 0 && migrationUnknownObjects(m) == 0 && len(state.Failures) == 0 {
		fmt.Fprintln(os.Stderr, "Migration completed successfully.")
	} else {
		fmt.Fprintf(os.Stderr, "Migration %s.\n", status)
	}
	fmt.Fprintf(os.Stderr, "Total: %d, Processed: %d, Skipped: %d, Failed: %d, Unknown: %d\n", state.TotalObjects, state.ProcessedObjects, state.SkippedObjects, state.FailedObjects, migrationUnknownObjects(m))
	printClassification(m)
	for i, failure := range state.Failures {
		if i == 20 {
			fmt.Fprintf(os.Stderr, "... %d additional failure records (use -json for the full report)\n", len(state.Failures)-i)
			break
		}
		fmt.Fprintf(os.Stderr, "Failed object %q: %s\n", failure.Key, failure.Reason)
	}
	if jsonOutputFlag {
		if err := writeJSON(os.Stdout, m); err != nil {
			return fmt.Errorf("failed to encode completion report: %w", err)
		}
	}
	if status != "completed" {
		return fmt.Errorf("migration %s: %s", status, state.ErrorMessage)
	}
	unknown := migrationUnknownObjects(m)
	if state.FailedObjects > 0 || unknown > 0 || len(state.Failures) > 0 {
		fmt.Fprintf(os.Stderr, "Migration had %d failures and %d unknown objects.\n", state.FailedObjects, unknown)
		return fmt.Errorf("migration completed with failed or unknown objects")
	}
	return nil
}

func migrationUnknownObjects(m map[string]interface{}) int {
	count := 0
	for _, key := range []string{"unknown_objects", "unknown_formats", "unknown_versions", "unknown_count", "unrecognized_objects", "unrecognized_formats"} {
		if n := jsonNumberOrCollectionCount(m[key]); n > count {
			count = n
		}
	}
	// Older servers expose unknown source formats through the inventory
	// classification rather than a top-level counter. Treat malformed and
	// contradictory entries as unknown objects even if a partial top-level
	// count is also present.
	if cls, ok := m["classification"].(map[string]interface{}); ok {
		classified := jsonNumberOrCollectionCount(cls["malformed"]) + jsonNumberOrCollectionCount(cls["contradictory"])
		if classified > count {
			count = classified
		}
	}
	return count
}

func jsonNumberOrCollectionCount(v interface{}) int {
	switch value := v.(type) {
	case float64:
		if value > 0 {
			return int(value)
		}
	case json.Number:
		if n, err := strconv.Atoi(string(value)); err == nil && n > 0 {
			return n
		}
	case []interface{}:
		return len(value)
	case map[string]interface{}:
		if count := jsonNumberOrCollectionCount(value["count"]); count > 0 {
			return count
		}
		return len(value)
	}
	return 0
}

func classificationFromMap(m map[string]interface{}) (*server.ObjectClassification, error) {
	cls, ok := m["classification"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	data, err := json.Marshal(cls)
	if err != nil {
		return nil, fmt.Errorf("failed to re-encode classification: %w", err)
	}
	var c server.ObjectClassification
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse classification: %w", err)
	}
	return &c, nil
}

func printClassification(m map[string]interface{}) {
	cls, err := classificationFromMap(m)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		return
	}
	if cls != nil {
		fmt.Fprintf(os.Stderr, "\n%s", cls.Summary())
	}
}

func parseMigrationState(m map[string]interface{}) (*MigrationState, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal migration state: %w", err)
	}
	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode migration state: %w", err)
	}
	state.OperationID = migrationOperationID(m)
	state.UnknownObjects = migrationUnknownObjects(m)
	state.UnknownFormats = jsonNumberOrCollectionCount(m["unknown_formats"])
	state.UnknownVersions = jsonNumberOrCollectionCount(m["unknown_versions"])
	if state.Classification == nil {
		state.Classification, err = classificationFromMap(m)
		if err != nil {
			return nil, err
		}
	}
	return &state, nil
}

// doRequest makes an HTTP request with the admin token.
func doRequest(client *http.Client, method, requestURL, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}
