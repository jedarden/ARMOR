package validate

import (
	"fmt"
	"testing"
)

func TestCheckSuggestions(t *testing.T) {
	// Test status 409
	err409 := FormatStatusCodeError(201, 409, "POST /api/resource")
	fmt.Printf("=== Status 409 ===\n")
	fmt.Printf("Suggestions: %v\n", err409.Suggestions)
	
	// Test status 502  
	err502 := FormatStatusCodeError(200, 502, "GET /api/data")
	fmt.Printf("=== Status 502 ===\n")
	fmt.Printf("Suggestions: %v\n", err502.Suggestions)
	
	// Test status 503
	err503 := FormatStatusCodeError(200, 503, "GET /api/data")
	fmt.Printf("=== Status 503 ===\n")
	fmt.Printf("Suggestions: %v\n", err503.Suggestions)
	
	// Test status 504
	err504 := FormatStatusCodeError(200, 504, "GET /api/data")
	fmt.Printf("=== Status 504 ===\n")
	fmt.Printf("Suggestions: %v\n", err504.Suggestions)
}
