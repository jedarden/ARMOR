// Package server re-exports compression-rule helpers for existing server callers.
package server

import "github.com/jedarden/armor/internal/config"

type CompressionAction = config.CompressionAction

const (
	CompressionActionZstd = config.CompressionActionZstd
	CompressionActionNone = config.CompressionActionNone
)

type CompressRule = config.CompressRule
type CompressRules = config.CompressRules

var ParseCompressRules = config.ParseCompressRules
var ParseCompressRulesWithAlias = config.ParseCompressRulesWithAlias
var ParseOverrideHeader = config.ParseOverrideHeader
var ExtractContentType = config.ExtractContentType
var NormalizeKey = config.NormalizeKey

// EvaluateCompression keeps the server-level API while allowing Config to own
// the rule data without importing server (which would create an import cycle).
func EvaluateCompression(key, contentType string, rules *CompressRules, overrideHeader string, compress bool) (bool, error) {
	return config.EvaluateCompression(key, contentType, rules, overrideHeader, compress)
}
