// Package securecmp provides constant-time comparisons for secret values.
package securecmp

import "crypto/subtle"

// Equal reports whether a and b are equal using a constant-time comparison
// when they have the same length.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
