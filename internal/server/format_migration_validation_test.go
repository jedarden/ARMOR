// Package server provides tests for format migration parameter validation.
//
// These tests exercise the production parsing functions in
// format_migration_params.go — the same code the /admin/format/migrate handler
// runs — not a copy of it.
package server

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseMigrationTarget covers target validation against the configured
// write version: a mismatching target must fail closed.
func TestParseMigrationTarget(t *testing.T) {
	tests := []struct {
		name            string
		writeVersion    uint8
		targetParam     string
		expectedVersion uint8
		wantErr         bool
		errContains     string
	}{
		{name: "V3 target: explicit numeric match", writeVersion: 3, targetParam: "3", expectedVersion: 3},
		{name: "V3 target: explicit v-prefixed match", writeVersion: 3, targetParam: "v3", expectedVersion: 3},
		{name: "V3 target: v-prefixed uppercase match", writeVersion: 3, targetParam: "V3", expectedVersion: 3},
		{name: "V3 target: empty means configured write version", writeVersion: 3, targetParam: "", expectedVersion: 3},
		{
			name:         "V3 target: mismatched target fails closed",
			writeVersion: 3,
			targetParam:  "2",
			wantErr:      true,
			errContains:  "does not match configured write version v3",
		},
		{
			name:         "V2 target: mismatched target fails closed",
			writeVersion: 2,
			targetParam:  "3",
			wantErr:      true,
			errContains:  "does not match configured write version v2",
		},
		{
			name:         "V3 target: non-numeric target rejected",
			writeVersion: 3,
			targetParam:  "invalid",
			wantErr:      true,
			errContains:  "invalid target version",
		},
		{
			name:         "V3 target: out-of-range target rejected instead of wrapping",
			writeVersion: 3,
			targetParam:  "259",
			wantErr:      true,
			errContains:  "invalid target version",
		},
		{
			name:         "V3 target: zero target rejected",
			writeVersion: 3,
			targetParam:  "0",
			wantErr:      true,
			errContains:  "invalid target version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseMigrationTarget(tt.targetParam, tt.writeVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got target v%d", version)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got: %s", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != tt.expectedVersion {
				t.Errorf("expected target v%d, got v%d", tt.expectedVersion, version)
			}
		})
	}
}

// TestParseMigrationIncludeVersions covers include normalization: the v1/v2
// spellings, the comma-separated list, the version-dependent default, and
// rejection of source versions not older than the target.
func TestParseMigrationIncludeVersions(t *testing.T) {
	tests := []struct {
		name             string
		writeVersion     uint8
		includeParam     string
		expectedVersions []string
		wantErr          bool
		errContains      string
	}{
		{name: "V3 target: empty param defaults to v2", writeVersion: 3, includeParam: "", expectedVersions: []string{"2"}},
		{name: "V2 target: empty param defaults to v1", writeVersion: 2, includeParam: "", expectedVersions: []string{"1"}},
		{name: "V3 target: v2 normalized", writeVersion: 3, includeParam: "v2", expectedVersions: []string{"2"}},
		{name: "V3 target: bare 2 normalized", writeVersion: 3, includeParam: "2", expectedVersions: []string{"2"}},
		{name: "V3 target: whitespace tolerated", writeVersion: 3, includeParam: " v1 , v2 ", expectedVersions: []string{"1", "2"}},
		{name: "V3 target: v1,v2 both accepted and ordered", writeVersion: 3, includeParam: "v1,v2", expectedVersions: []string{"1", "2"}},
		{name: "V3 target: duplicates collapsed", writeVersion: 3, includeParam: "v2,v1,v2", expectedVersions: []string{"2", "1"}},
		{
			name:         "V3 target: v3 rejected as source",
			writeVersion: 3,
			includeParam: "v3",
			wantErr:      true,
			errContains:  "not compatible with target version v3",
		},
		{
			name:         "V3 target: bare 3 rejected as source",
			writeVersion: 3,
			includeParam: "3",
			wantErr:      true,
			errContains:  "not compatible with target version v3",
		},
		{
			name:         "V2 target: v2 rejected as source",
			writeVersion: 2,
			includeParam: "v2",
			wantErr:      true,
			errContains:  "not compatible with target version v2",
		},
		{
			name:         "V2 target: v3 rejected as source",
			writeVersion: 2,
			includeParam: "v3",
			wantErr:      true,
			errContains:  "not compatible with target version v2",
		},
		{
			name:         "V3 target: v1,v3 rejected as a set",
			writeVersion: 3,
			includeParam: "v1,v3",
			wantErr:      true,
			errContains:  "not compatible with target version v3",
		},
		{
			name:         "V3 target: non-numeric entry rejected",
			writeVersion: 3,
			includeParam: "v1,invalid",
			wantErr:      true,
			errContains:  "invalid include version",
		},
		{
			name:         "V3 target: zero rejected",
			writeVersion: 3,
			includeParam: "v0",
			wantErr:      true,
			errContains:  "invalid include version",
		},
		{
			name:         "V3 target: bare comma yields invalid entry",
			writeVersion: 3,
			includeParam: "v1,",
			wantErr:      true,
			errContains:  "invalid include version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, err := parseMigrationIncludeVersions(tt.includeParam, tt.writeVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got versions %v", versions)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got: %s", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(versions, tt.expectedVersions) {
				t.Errorf("expected versions %v, got %v", tt.expectedVersions, versions)
			}
		})
	}
}

// TestParseVersionParam covers the shared version-spelling parser, including
// the bounds that keep the value from wrapping in a uint8.
func TestParseVersionParam(t *testing.T) {
	valid := map[string]uint8{
		"1":   1,
		"v1":  1,
		"V2":  2,
		" 3 ": 3,
		"255": 255,
	}
	for input, want := range valid {
		got, err := parseVersionParam(input)
		if err != nil {
			t.Errorf("parseVersionParam(%q) unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("parseVersionParam(%q) = %d, want %d", input, got, want)
		}
	}

	for _, input := range []string{"", "v", "0", "-1", "256", "300", "one", "v1.5", "1x"} {
		if _, err := parseVersionParam(input); err == nil {
			t.Errorf("parseVersionParam(%q) expected error, got none", input)
		}
	}
}
