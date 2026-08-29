// Package config provides compression-rule parsing and matching for ARMOR.
package config

import (
	"fmt"
	"strings"
)

// CompressionAction represents the compression action for a matched rule.
type CompressionAction string

const (
	// CompressionActionZstd enables zstd compression.
	CompressionActionZstd CompressionAction = "zstd"
	// CompressionActionNone disables compression.
	CompressionActionNone CompressionAction = "none"
)

// CompressRule represents a single compression rule.
type CompressRule struct {
	// Pattern is the suffix or content-type to match.
	// Examples: ".jsonl", ".wal", "application/json", "*"
	Pattern string

	// Action is the compression action (zstd or none).
	Action CompressionAction

	// IsContentType is true if Pattern is a content-type, false if it's a suffix.
	IsContentType bool
}

// CompressRules holds ordered compression rules for first-match-wins evaluation.
type CompressRules struct {
	// Rules is the ordered list of rules. First match wins.
	Rules []CompressRule
}

// ParseCompressRules parses a comma-separated compress rules string.
// Format: "<suffix>|<content-type>=zstd|none,..."
// Examples: ".jsonl=zstd,.wal=zstd,application/json=zstd,*=none"
// ARMOR_COMPRESS=true is treated as the alias "*=zstd".
//
// Returns an error if any rule is invalid.
func ParseCompressRules(rulesStr string) (*CompressRules, error) {
	if rulesStr == "" {
		return &CompressRules{Rules: []CompressRule{}}, nil
	}

	rules := &CompressRules{}

	parts := strings.Split(rulesStr, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Parse pattern=action
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid rule %q at position %d (expected pattern=action)", part, i)
		}

		pattern := strings.TrimSpace(kv[0])
		actionStr := strings.TrimSpace(kv[1])

		if pattern == "" {
			return nil, fmt.Errorf("empty pattern in rule %q at position %d", part, i)
		}

		// Validate and parse action
		var action CompressionAction
		switch actionStr {
		case "zstd":
			action = CompressionActionZstd
		case "none":
			action = CompressionActionNone
		default:
			return nil, fmt.Errorf("invalid action %q in rule %q (expected 'zstd' or 'none')", actionStr, part)
		}

		// Determine if pattern is a content-type (contains '/') or suffix
		isContentType := strings.Contains(pattern, "/")

		rules.Rules = append(rules.Rules, CompressRule{
			Pattern:       pattern,
			Action:        action,
			IsContentType: isContentType,
		})
	}

	return rules, nil
}

// ShouldCompress evaluates rules against a key and content-type to determine
// whether to compress. Returns (shouldCompress, matched) where shouldCompress
// is true if compression should be applied, and matched is true if any rule matched.
// If no rule matches, returns (false, false).
//
// First match wins. Rules are evaluated in order.
// Suffix rules check if the key ends with the pattern (e.g., ".jsonl").
// Content-type rules check for exact match.
// Wildcard "*" matches everything (should be last rule).
func (r *CompressRules) ShouldCompress(key, contentType string) (bool, bool) {
	for _, rule := range r.Rules {
		if rule.IsContentType {
			// Content-type exact match
			if contentType == rule.Pattern {
				return rule.Action == CompressionActionZstd, true
			}
		} else {
			// Suffix match or wildcard
			if rule.Pattern == "*" {
				return rule.Action == CompressionActionZstd, true
			}
			// Check suffix match
			if strings.HasSuffix(key, rule.Pattern) {
				return rule.Action == CompressionActionZstd, true
			}
		}
	}

	// No rule matched
	return false, false
}

// HasRules returns true if any rules are configured.
func (r *CompressRules) HasRules() bool {
	if r == nil {
		return false
	}
	return len(r.Rules) > 0
}

// String returns a string representation of the rules (for debugging).
func (r *CompressRules) String() string {
	if r == nil {
		return "<no rules>"
	}
	if len(r.Rules) == 0 {
		return "<no rules>"
	}

	var parts []string
	for _, rule := range r.Rules {
		part := fmt.Sprintf("%s=%s", rule.Pattern, rule.Action)
		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

// ParseCompressRulesWithAlias parses rules and handles the ARMOR_COMPRESS=true alias.
// If compressAlias is true, it's treated as "*=zstd" (everything compressed).
// If rulesStr is non-empty, it takes precedence over the alias.
func ParseCompressRulesWithAlias(rulesStr string, compressAlias bool) (*CompressRules, error) {
	// If explicit rules are provided, use them
	if rulesStr != "" {
		return ParseCompressRules(rulesStr)
	}

	// Handle ARMOR_COMPRESS=true alias
	if compressAlias {
		return ParseCompressRules("*=zstd")
	}

	// No compression rules
	return &CompressRules{Rules: []CompressRule{}}, nil
}

// ParseOverrideHeader parses the x-amz-meta-armor-compress header override.
// Valid values: "true" (compress), "false" (don't compress).
// Returns (shouldCompress, hasOverride, error).
// If the header is not set, returns (false, false, nil).
// If the header has an invalid value, returns an error.
func ParseOverrideHeader(headerValue string) (bool, bool, error) {
	if headerValue == "" {
		return false, false, nil
	}

	switch strings.ToLower(headerValue) {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("invalid x-amz-meta-armor-compress value %q (expected 'true' or 'false')", headerValue)
	}
}

// EvaluateCompression determines whether to compress based on rules and override header.
// Parameters:
//   - key: object key
//   - contentType: content-type header value
//   - rules: compression rules (may be empty)
//   - overrideHeader: x-amz-meta-armor-compress header value (may be empty)
//
// Returns (shouldCompress, error).
// Override header takes precedence over rules.
// If no override and no rules, returns false (no compression).
func EvaluateCompression(key, contentType string, rules *CompressRules, overrideHeader string) (bool, error) {
	// Check override header first (highest priority)
	shouldCompress, hasOverride, err := ParseOverrideHeader(overrideHeader)
	if err != nil {
		return false, err
	}
	if hasOverride {
		return shouldCompress, nil
	}

	// Apply rules if configured
	if rules != nil && rules.HasRules() {
		shouldCompress, matched := rules.ShouldCompress(key, contentType)
		if matched {
			return shouldCompress, nil
		}
	}

	// Default: no compression
	return false, nil
}

// ExtractContentType extracts the content-type from a full content-type string
// (e.g., "application/json; charset=utf-8" -> "application/json").
// Returns empty string if contentTypeStr is empty.
func ExtractContentType(contentTypeStr string) string {
	if contentTypeStr == "" {
		return ""
	}

	// Split on semicolon and take the first part (mime type)
	parts := strings.SplitN(contentTypeStr, ";", 2)
	return strings.TrimSpace(parts[0])
}

// NormalizeKey normalizes a key for suffix matching.
// For most cases, this is a no-op. It can be extended to handle
// special cases (e.g., case normalization).
func NormalizeKey(key string) string {
	return key
}
