// cmd_migrate.go implements the 'armor migrate' subcommand for format migration
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jedarden/armor/internal/server"
)

func init() {
	// Migration-specific flags, on migrate's own flag set
	migrateFlags := flag.NewFlagSet("migrate", flag.ExitOnError)
	migrateFlags.StringVar(&adminURLFlag, "admin-url", "", "Admin API endpoint (required, e.g., http://127.0.0.1:9001)")
	migrateFlags.BoolVar(&dryRunFlag, "dry-run", false, "Dry run: verify objects can be migrated without making changes")
	migrateFlags.StringVar(&targetFlag, "target", "", "Target format version to migrate to (e.g., v3 or 3); must match the server's configured format write version")
	migrateFlags.StringVar(&includeFlag, "include", "", "Comma-separated source versions to migrate (e.g., v1,v2; defaults to v2 for V3 target, v1 for V2 target)")
	migrateFlags.IntVar(&concurrencyFlag, "concurrency", 0, "Number of concurrent workers (default: server-side default)")
	migrateFlags.BoolVar(&watchFlag, "watch", false, "Watch mode: poll progress until completion")
	migrateFlags.BoolVar(&jsonOutputFlag, "json", false, "Print the completion report as machine-readable JSON on stdout (the human-readable summary still goes to stderr)")

	registerCommand(Command{
		Name:        "migrate",
		Description: "Migrate ARMOR objects to the current encryption format (client of /admin/format/migrate)",
		Flags:       migrateFlags,
		Func:        migrate,
	})
}

// Migration flags
var (
	adminURLFlag    string
	dryRunFlag      bool
	targetFlag      string
	includeFlag     string
	concurrencyFlag int
	watchFlag       bool
	jsonOutputFlag  bool
)

// exit is declared once, in cmd_client_config.go (same package); reused
// here rather than redeclared, since a second package-level var of the same
// name doesn't compile.

// MigrationState represents the state of a format migration operation
type MigrationState struct {
	ID                  string   `json:"id"`
	StartTime           string   `json:"start_time"`
	LastUpdated         string   `json:"last_updated"`
	Status              string   `json:"status"`
	TotalObjects        int      `json:"total_objects"`
	ProcessedObjects    int      `json:"processed_objects"`
	SkippedObjects      int      `json:"skipped_objects"`
	FailedObjects       int      `json:"failed_objects"`
	LastKey             string   `json:"last_key"`
	IncludeVersions     []string `json:"include_versions"`
	CurrentWriteVersion uint8    `json:"current_write_version"`
	DryRun              bool     `json:"dry_run"`
	Concurrency         int      `json:"concurrency"`
	Failures            []struct {
		Key    string `json:"key"`
		Reason string `json:"reason"`
		Time   string `json:"time"`
	} `json:"failures,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	// Classification mirrors server.ObjectClassification, the per-dimension
	// count report (source/layout, size bucket, key fingerprint, outcome)
	// the server accumulates over the walk.
	Classification *server.ObjectClassification `json:"classification,omitempty"`
}

// MigrationResult represents the final result of a migration operation
type MigrationResult struct {
	TotalObjects     int    `json:"total_objects"`
	ProcessedObjects int    `json:"processed_objects"`
	SkippedObjects   int    `json:"skipped_objects"`
	FailedObjects    int    `json:"failed_objects"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message,omitempty"`
	DryRun           bool   `json:"dry_run"`
}

func migrate(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor migrate [flags]\n")
		exit(2)
	}

	// Validate required flags
	if adminURLFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: -admin-url is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor migrate -admin-url <url> [other flags]\n")
		exit(2)
	}

	// Get admin token from environment
	adminToken := os.Getenv("ARMOR_ADMIN_TOKEN")
	if adminToken == "" {
		fmt.Fprintf(os.Stderr, "Error: ARMOR_ADMIN_TOKEN environment variable is required\n")
		exit(1)
	}

	// Build query parameters
	values := url.Values{}
	if dryRunFlag {
		values.Add("dry_run", "true")
	}
	if targetFlag != "" {
		values.Add("target", targetFlag)
	}
	if includeFlag != "" {
		values.Add("include", includeFlag)
	}
	if concurrencyFlag > 0 {
		values.Add("concurrency", fmt.Sprintf("%d", concurrencyFlag))
	}

	// Build the full URL
	migrateURL := adminURLFlag + "/admin/format/migrate"
	if len(values) > 0 {
		migrateURL += "?" + values.Encode()
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	// Start the migration
	fmt.Fprintf(os.Stderr, "Starting migration...\n")
	resp, err := doRequest(client, "POST", migrateURL, adminToken, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to start migration: %v\n", err)
		exit(1)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Error: migration request failed with status %d: %s\n", resp.StatusCode, string(body))
		exit(1)
	}

	// Parse response to get initial state
	var initialState map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&initialState); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse response: %v\n", err)
		exit(1)
	}

	// Check if the response contains a status field indicating an async operation
	// If the migration started asynchronously, we need to poll for progress
	if watchFlag {
		// Watch mode: poll progress until completion
		if err := watchMigration(client, adminURLFlag+"/admin/format/migrate", adminToken); err != nil {
			fmt.Fprintf(os.Stderr, "Error: migration watch failed: %v\n", err)
			exit(1)
		}
		return
	}

	// Non-watch mode: just report the initial response and exit
	fmt.Fprintf(os.Stderr, "Migration started successfully.\n")
	if status, ok := initialState["status"].(string); ok {
		fmt.Fprintf(os.Stderr, "Status: %s\n", status)
	}
	// The POST runs the migration synchronously, so the response already
	// carries the final count report.
	if jsonOutputFlag {
		if err := writeJSON(os.Stdout, initialState); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to encode completion report: %v\n", err)
			exit(1)
		}
	}
	printClassification(initialState)
}

// writeJSON renders one machine-readable report to w, indented so two runs
// remain diffable by eye. This is the JSON output mode of the completion
// report: the full response document, classification member included.
func writeJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// watchMigration polls the migration progress endpoint until completion
func watchMigration(client *http.Client, migrateURL, adminToken string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastProcessed := -1
	lastFailed := -1

	for range ticker.C {
		resp, err := doRequest(client, "GET", migrateURL, adminToken, nil)
		if err != nil {
			return fmt.Errorf("failed to get migration status: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Parse response
		var result interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse status response: %w", err)
		}

		// Check if it's a "no migration" response
		if m, ok := result.(map[string]interface{}); ok {
			if status, ok := m["status"].(string); ok && status == "no_migration" {
				fmt.Fprintf(os.Stderr, "No migration in progress.\n")
				return nil
			}

			// Parse the migration state
			state, err := parseMigrationState(m)
			if err != nil {
				return fmt.Errorf("failed to parse migration state: %w", err)
			}

			// Print progress line only if something changed. Under -json the
			// progress line moves to stderr so stdout carries nothing but
			// the final JSON report.
			if state.ProcessedObjects != lastProcessed || state.FailedObjects != lastFailed {
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
				fmt.Fprintf(progress, " (%s)\n", state.Status)
				lastProcessed = state.ProcessedObjects
				lastFailed = state.FailedObjects
			}

			// Check if migration is complete
			if state.Status == "completed" {
				fmt.Fprintf(os.Stderr, "Migration completed successfully.\n")
				fmt.Fprintf(os.Stderr, "Total: %d, Processed: %d, Skipped: %d, Failed: %d\n",
					state.TotalObjects, state.ProcessedObjects, state.SkippedObjects, state.FailedObjects)

				// Per-dimension count report: source/layout, size bucket,
				// key fingerprint, outcome. The same counts travel
				// machine-readable on the state JSON's classification field.
				if state.Classification != nil {
					fmt.Fprintf(os.Stderr, "\n%s", state.Classification.Summary())
				}

				// JSON output mode: the server's final state document,
				// classification member included, verbatim — the same
				// document the synchronous (non-watch) mode reports, not a
				// re-serialization through the CLI's MigrationState subset.
				if jsonOutputFlag {
					if err := writeJSON(os.Stdout, m); err != nil {
						return fmt.Errorf("failed to encode completion report: %w", err)
					}
				}

				// Exit non-zero if there were failures
				if state.FailedObjects > 0 {
					fmt.Fprintf(os.Stderr, "Migration had %d failures.\n", state.FailedObjects)
					exit(1)
				}
				return nil
			}

			if state.Status == "failed" || state.Status == "interrupted" {
				return fmt.Errorf("migration %s: %s", state.Status, state.ErrorMessage)
			}
		}
	}

	// Unreachable: ticker.C is never closed, so the loop above only exits via
	// an explicit return. Present for the compiler's control-flow analysis.
	return nil
}

// classificationFromMap decodes the "classification" member of a decoded
// JSON response (migration state or result) into the server's
// ObjectClassification. It returns nil when the member is absent.
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

// printClassification renders the human-readable per-dimension count report
// from a decoded JSON response, when that response carries one.
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

// parseMigrationState parses a map into a MigrationState
func parseMigrationState(m map[string]interface{}) (*MigrationState, error) {
	state := &MigrationState{}

	if id, ok := m["id"].(string); ok {
		state.ID = id
	}
	if startTime, ok := m["start_time"].(string); ok {
		state.StartTime = startTime
	}
	if lastUpdated, ok := m["last_updated"].(string); ok {
		state.LastUpdated = lastUpdated
	}
	if status, ok := m["status"].(string); ok {
		state.Status = status
	}
	if total, ok := m["total_objects"].(float64); ok {
		state.TotalObjects = int(total)
	}
	if processed, ok := m["processed_objects"].(float64); ok {
		state.ProcessedObjects = int(processed)
	}
	if skipped, ok := m["skipped_objects"].(float64); ok {
		state.SkippedObjects = int(skipped)
	}
	if failed, ok := m["failed_objects"].(float64); ok {
		state.FailedObjects = int(failed)
	}
	if lastKey, ok := m["last_key"].(string); ok {
		state.LastKey = lastKey
	}
	if includeVersions, ok := m["include_versions"].([]interface{}); ok {
		for _, v := range includeVersions {
			if vs, ok := v.(string); ok {
				state.IncludeVersions = append(state.IncludeVersions, vs)
			}
		}
	}
	if currentWriteVersion, ok := m["current_write_version"].(float64); ok {
		state.CurrentWriteVersion = uint8(currentWriteVersion)
	}
	if dryRun, ok := m["dry_run"].(bool); ok {
		state.DryRun = dryRun
	}
	if concurrency, ok := m["concurrency"].(float64); ok {
		state.Concurrency = int(concurrency)
	}
	if errorMsg, ok := m["error_message"].(string); ok {
		state.ErrorMessage = errorMsg
	}
	if cls, err := classificationFromMap(m); err != nil {
		return nil, err
	} else if cls != nil {
		state.Classification = cls
	}

	// Parse failures if present
	if failures, ok := m["failures"].([]interface{}); ok {
		for _, f := range failures {
			if fm, ok := f.(map[string]interface{}); ok {
				var failure struct {
					Key    string `json:"key"`
					Reason string `json:"reason"`
					Time   string `json:"time"`
				}
				if key, ok := fm["key"].(string); ok {
					failure.Key = key
				}
				if reason, ok := fm["reason"].(string); ok {
					failure.Reason = reason
				}
				if time, ok := fm["time"].(string); ok {
					failure.Time = time
				}
				state.Failures = append(state.Failures, failure)
			}
		}
	}

	return state, nil
}

// doRequest makes an HTTP request with the admin token
func doRequest(client *http.Client, method, url, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return client.Do(req)
}
