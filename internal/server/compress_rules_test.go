package server

import (
	"testing"
)

func TestParseCompressRules(t *testing.T) {
	tests := []struct {
		name        string
		rulesStr    string
		wantRules   int // number of expected rules
		wantErr     bool
		errContains string
	}{
		{
			name:      "empty string",
			rulesStr:  "",
			wantRules: 0,
			wantErr:   false,
		},
		{
			name:      "single suffix rule",
			rulesStr:  ".jsonl=zstd",
			wantRules: 1,
			wantErr:   false,
		},
		{
			name:      "multiple suffix rules",
			rulesStr:  ".jsonl=zstd,.wal=zstd,.log=none",
			wantRules: 3,
			wantErr:   false,
		},
		{
			name:      "content-type rule",
			rulesStr:  "application/json=zstd",
			wantRules: 1,
			wantErr:   false,
		},
		{
			name:      "mixed rules",
			rulesStr:  ".jsonl=zstd,application/json=zstd,.wal=none",
			wantRules: 3,
			wantErr:   false,
		},
		{
			name:      "wildcard rule",
			rulesStr:  "*=none",
			wantRules: 1,
			wantErr:   false,
		},
		{
			name:      "wildcard with suffix",
			rulesStr:  ".jsonl=zstd,*=none",
			wantRules: 2,
			wantErr:   false,
		},
		{
			name:      "whitespace handling",
			rulesStr:  " .jsonl = zstd , application/json = zstd ",
			wantRules: 2,
			wantErr:   false,
		},
		{
			name:        "missing action",
			rulesStr:    ".jsonl",
			wantRules:   0,
			wantErr:     true,
			errContains: "expected pattern=action",
		},
		{
			name:        "invalid action",
			rulesStr:    ".jsonl=gzip",
			wantRules:   0,
			wantErr:     true,
			errContains: "invalid action",
		},
		{
			name:        "empty pattern",
			rulesStr:    "=zstd",
			wantRules:   0,
			wantErr:     true,
			errContains: "empty pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseCompressRules(tt.rulesStr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseCompressRules() expected error, got nil")
				}
				if tt.errContains != "" && !strContains(err.Error(), tt.errContains) {
					t.Fatalf("ParseCompressRules() error = %v, expected error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCompressRules() unexpected error: %v", err)
			}
			if len(rules.Rules) != tt.wantRules {
				t.Fatalf("ParseCompressRules() got %d rules, want %d", len(rules.Rules), tt.wantRules)
			}
		})
	}
}

func TestCompressRules_ShouldCompress(t *testing.T) {
	tests := []struct {
		name         string
		rulesStr     string
		key          string
		contentType  string
		wantCompress bool
		wantMatched  bool
	}{
		{
			name:         "no rules",
			rulesStr:     "",
			key:          "test.jsonl",
			contentType:  "",
			wantCompress: false,
			wantMatched:  false,
		},
		{
			name:         "suffix match",
			rulesStr:     ".jsonl=zstd",
			key:          "data/test.jsonl",
			contentType:  "",
			wantCompress: true,
			wantMatched:  true,
		},
		{
			name:         "suffix no match",
			rulesStr:     ".jsonl=zstd",
			key:          "data/test.log",
			contentType:  "",
			wantCompress: false,
			wantMatched:  false,
		},
		{
			name:         "content-type match",
			rulesStr:     "application/json=zstd",
			key:          "any-key",
			contentType:  "application/json",
			wantCompress: true,
			wantMatched:  true,
		},
		{
			name:         "content-type no match",
			rulesStr:     "application/json=zstd",
			key:          "any-key",
			contentType:  "text/plain",
			wantCompress: false,
			wantMatched:  false,
		},
		{
			name:         "wildcard matches all",
			rulesStr:     "*=zstd",
			key:          "any-key",
			contentType:  "",
			wantCompress: true,
			wantMatched:  true,
		},
		{
			name:         "wildcard none",
			rulesStr:     "*=none",
			key:          "any-key",
			contentType:  "",
			wantCompress: false,
			wantMatched:  true,
		},
		{
			name:         "first match wins - suffix before wildcard",
			rulesStr:     ".jsonl=zstd,*=none",
			key:          "test.jsonl",
			contentType:  "",
			wantCompress: true,
			wantMatched:  true,
		},
		{
			name:         "first match wins - wildcard catches unmatched",
			rulesStr:     ".jsonl=zstd,*=none",
			key:          "test.log",
			contentType:  "",
			wantCompress: false,
			wantMatched:  true,
		},
		{
			name:         "content-type with charset",
			rulesStr:     "application/json=zstd",
			key:          "test",
			contentType:  "application/json; charset=utf-8",
			wantCompress: false, // No match: full content-type with charset doesn't match exactly
			wantMatched:  false,
		},
		{
			name:         "multiple rules - first suffix matches",
			rulesStr:     ".jsonl=zstd,.wal=none",
			key:          "test.jsonl",
			contentType:  "",
			wantCompress: true,
			wantMatched:  true,
		},
		{
			name:         "multiple rules - second suffix matches",
			rulesStr:     ".jsonl=zstd,.wal=none",
			key:          "test.wal",
			contentType:  "",
			wantCompress: false,
			wantMatched:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseCompressRules(tt.rulesStr)
			if err != nil {
				t.Fatalf("ParseCompressRules() failed: %v", err)
			}

			// ShouldCompress expects the full content-type string (with charset if present)
			// EvaluateCompression passes it directly without extraction.
			gotCompress, gotMatched := rules.ShouldCompress(tt.key, tt.contentType)
			if gotCompress != tt.wantCompress {
				t.Errorf("ShouldCompress() compress = %v, want %v", gotCompress, tt.wantCompress)
			}
			if gotMatched != tt.wantMatched {
				t.Errorf("ShouldCompress() matched = %v, want %v", gotMatched, tt.wantMatched)
			}
		})
	}
}

func TestParseCompressRulesWithAlias(t *testing.T) {
	tests := []struct {
		name          string
		rulesStr      string
		compressAlias bool
		wantRules     int
		wantErr       bool
	}{
		{
			name:          "explicit rules override alias",
			rulesStr:      ".jsonl=zstd",
			compressAlias: true,
			wantRules:     1,
			wantErr:       false,
		},
		{
			name:          "alias true produces wildcard",
			rulesStr:      "",
			compressAlias: true,
			wantRules:     1,
			wantErr:       false,
		},
		{
			name:          "alias false produces no rules",
			rulesStr:      "",
			compressAlias: false,
			wantRules:     0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseCompressRulesWithAlias(tt.rulesStr, tt.compressAlias)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseCompressRulesWithAlias() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCompressRulesWithAlias() unexpected error: %v", err)
			}
			if len(rules.Rules) != tt.wantRules {
				t.Errorf("ParseCompressRulesWithAlias() got %d rules, want %d", len(rules.Rules), tt.wantRules)
			}
		})
	}
}

func TestParseOverrideHeader(t *testing.T) {
	tests := []struct {
		name           string
		headerValue    string
		wantCompress   bool
		wantOverride   bool
		wantErr        bool
		errContains    string
	}{
		{
			name:         "empty header",
			headerValue:  "",
			wantCompress: false,
			wantOverride: false,
			wantErr:      false,
		},
		{
			name:         "true",
			headerValue:  "true",
			wantCompress: true,
			wantOverride: true,
			wantErr:      false,
		},
		{
			name:         "false",
			headerValue:  "false",
			wantCompress: false,
			wantOverride: true,
			wantErr:      false,
		},
		{
			name:         "TRUE uppercase",
			headerValue:  "TRUE",
			wantCompress: true,
			wantOverride: true,
			wantErr:      false,
		},
		{
			name:         "mixed case",
			headerValue:  "True",
			wantCompress: true,
			wantOverride: true,
			wantErr:      false,
		},
		{
			name:        "invalid value",
			headerValue: "yes",
			wantCompress: false,
			wantOverride: false,
			wantErr:      true,
			errContains:  "invalid x-amz-meta-armor-compress value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCompress, gotOverride, err := ParseOverrideHeader(tt.headerValue)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseOverrideHeader() expected error, got nil")
				}
				if tt.errContains != "" && !strContains(err.Error(), tt.errContains) {
					t.Fatalf("ParseOverrideHeader() error = %v, expected error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOverrideHeader() unexpected error: %v", err)
			}
			if gotCompress != tt.wantCompress {
				t.Errorf("ParseOverrideHeader() compress = %v, want %v", gotCompress, tt.wantCompress)
			}
			if gotOverride != tt.wantOverride {
				t.Errorf("ParseOverrideHeader() override = %v, want %v", gotOverride, tt.wantOverride)
			}
		})
	}
}

func TestEvaluateCompression(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		contentType   string
		rulesStr      string
		overrideValue string
		compress      bool
		wantCompress  bool
		wantErr       bool
		errContains   string
	}{
		{
			name:          "no rules, no override, compress=false",
			key:           "test.jsonl",
			contentType:   "",
			rulesStr:      "",
			overrideValue: "",
			compress:      false,
			wantCompress:  false,
			wantErr:       false,
		},
		{
			name:          "no rules, no override, compress=true",
			key:           "test.jsonl",
			contentType:   "",
			rulesStr:      "",
			overrideValue: "",
			compress:      true,
			wantCompress:  true,
			wantErr:       false,
		},
		{
			name:          "override true ignores rules and compress flag",
			key:           "test.log",
			contentType:   "",
			rulesStr:      "*.log=none",
			overrideValue: "true",
			compress:      false,
			wantCompress:  true,
			wantErr:       false,
		},
		{
			name:          "override false ignores rules and compress flag",
			key:           "test.jsonl",
			contentType:   "",
			rulesStr:      ".jsonl=zstd",
			overrideValue: "false",
			compress:      true,
			wantCompress:  false,
			wantErr:       false,
		},
		{
			name:          "rules apply when no override",
			key:           "test.jsonl",
			contentType:   "",
			rulesStr:      ".jsonl=zstd",
			overrideValue: "",
			compress:      false,
			wantCompress:  true,
			wantErr:       false,
		},
		{
			name:          "content-type rule match",
			key:           "any-key",
			contentType:   "application/json",
			rulesStr:      "application/json=zstd",
			overrideValue: "",
			compress:      false,
			wantCompress:  true,
			wantErr:       false,
		},
		{
			name:          "wildcard catches unmatched",
			key:           "test.unknown",
			contentType:   "",
			rulesStr:      ".jsonl=zstd,*=none",
			overrideValue: "",
			compress:      false,
			wantCompress:  false,
			wantErr:       false,
		},
		{
			name:          "invalid override header",
			key:           "test",
			contentType:   "",
			rulesStr:      "",
			overrideValue: "invalid",
			compress:      false,
			wantCompress:  false,
			wantErr:       true,
			errContains:   "invalid x-amz-meta-armor-compress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rules *CompressRules
			var err error

			if tt.rulesStr != "" {
				rules, err = ParseCompressRules(tt.rulesStr)
				if err != nil {
					t.Fatalf("ParseCompressRules() failed: %v", err)
				}
			}

			gotCompress, err := EvaluateCompression(tt.key, tt.contentType, rules, tt.overrideValue, tt.compress)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("EvaluateCompression() expected error, got nil")
				}
				if tt.errContains != "" && !strContains(err.Error(), tt.errContains) {
					t.Fatalf("EvaluateCompression() error = %v, expected error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("EvaluateCompression() unexpected error: %v", err)
			}
			if gotCompress != tt.wantCompress {
				t.Errorf("EvaluateCompression() = %v, want %v", gotCompress, tt.wantCompress)
			}
		})
	}
}

func TestExtractContentType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"application/json", "application/json"},
		{"application/json; charset=utf-8", "application/json"},
		{"text/plain; charset=iso-8859-1", "text/plain"},
		{"multipart/form-data; boundary=----WebKitFormBoundary", "multipart/form-data"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExtractContentType(tt.input)
			if got != tt.expected {
				t.Errorf("ExtractContentType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCompressRules_String(t *testing.T) {
	tests := []struct {
		name     string
		rulesStr string
		want     string
	}{
		{
			name:     "no rules",
			rulesStr: "",
			want:     "<no rules>",
		},
		{
			name:     "single rule",
			rulesStr: ".jsonl=zstd",
			want:     ".jsonl=zstd",
		},
		{
			name:     "multiple rules",
			rulesStr: ".jsonl=zstd,.wal=none",
			want:     ".jsonl=zstd,.wal=none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseCompressRules(tt.rulesStr)
			if err != nil {
				t.Fatalf("ParseCompressRules() failed: %v", err)
			}
			got := rules.String()
			if got != tt.want {
				t.Errorf("CompressRules.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// strContains checks if a string contains a substring. Named to avoid
// colliding with dashboard_integration_test.go's containsString, which
// checks slice membership ([]string, string), not substrings. contains
// (the actual substring scan) is declared once, in
// error_server_enhanced_test.go -- reused here instead of a duplicate.
func strContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && contains(s, substr)))
}
