// Package server provides enhanced test server infrastructure for error testing.
//
// # Enhanced Error Server Setup
//
// This file extends the base error_server_setup.go with advanced error injection
// capabilities including severity levels, frequency control, and probability-based
// error scenarios.
//
// Usage:
//   server := NewEnhancedErrorServer([]EnhancedErrorScenario{
//       {Type: ErrorTypeRateLimit, Severity: ErrorSeverityHigh, Frequency: 0.5},
//   })
//   defer server.Close()
package server

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

// =============================================================================
// ERROR TYPE DEFINITIONS
// =============================================================================

// ErrorType represents the category of error to inject.
type ErrorType string

const (
	// ErrorTypeTimeout simulates timeout scenarios (uses Delay field)
	ErrorTypeTimeout ErrorType = "timeout"

	// ErrorTypeRateLimit simulates rate limiting (429 TooManyRequests)
	ErrorTypeRateLimit ErrorType = "rate_limit"

	// ErrorTypeConnectionRefused simulates connection failures
	ErrorTypeConnectionRefused ErrorType = "connection_refused"

	// ErrorTypeServiceUnavailable simulates 503 errors
	ErrorTypeServiceUnavailable ErrorType = "service_unavailable"

	// ErrorTypeInternalError simulates 500 errors
	ErrorTypeInternalError ErrorType = "internal_error"

	// ErrorTypeBadRequest simulates 400 errors
	ErrorTypeBadRequest ErrorType = "bad_request"

	// ErrorTypeUnauthorized simulates 401 errors
	ErrorTypeUnauthorized ErrorType = "unauthorized"

	// ErrorTypeForbidden simulates 403 errors
	ErrorTypeForbidden ErrorType = "forbidden"

	// ErrorTypeNotFound simulates 404 errors
	ErrorTypeNotFound ErrorType = "not_found"

	// ErrorTypeSlowResponse simulates slow (but successful) responses
	ErrorTypeSlowResponse ErrorType = "slow_response"
)

// ErrorSeverity represents the impact level of an error.
type ErrorSeverity string

const (
	// ErrorSeverityLow represents minor errors that may not block operations
	ErrorSeverityLow ErrorSeverity = "low"

	// ErrorSeverityMedium represents errors that partially impact operations
	ErrorSeverityMedium ErrorSeverity = "medium"

	// ErrorSeverityHigh represents critical errors that block operations
	ErrorSeverityHigh ErrorSeverity = "high"

	// ErrorSeverityCritical represents severe errors requiring immediate attention
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// =============================================================================
// ENHANCED ERROR SCENARIO
// =============================================================================

// EnhancedErrorScenario extends ErrorServerScenario with advanced configuration.
type EnhancedErrorScenario struct {
	// Base scenario configuration
	ErrorServerScenario

	// Type is the error category (timeout, rate_limit, etc.)
	Type ErrorType

	// Severity indicates the impact level (low, medium, high, critical)
	Severity ErrorSeverity

	// Frequency controls how often this error occurs (0.0 to 1.0)
	// 0.0 = never occurs, 1.0 = always occurs
	Frequency float64

	// Probability controls the chance of occurrence per request (0.0 to 1.0)
	// If both Frequency and Probability are set, the effective probability is:
	// min(Frequency, Probability) for sequential evaluation
	// Frequency * Probability for independent evaluation
	Probability float64

	// RetryAfter specifies the suggested retry duration for rate limits
	RetryAfter time.Duration

	// ErrorCodeOverride overrides the default error code for the type
	ErrorCodeOverride string

	// MessageOverride overrides the default error message
	MessageOverride string

	// Intercept determines if the request should be intercepted before processing
	// If true, the request fails immediately without processing
	Intercept bool

	// Condition is an optional function that determines if this scenario should apply
	// Returns true if the scenario should be triggered
	Condition func(*http.Request) bool
}

// =============================================================================
// ENHANCED ERROR SERVER
// =============================================================================

// EnhancedErrorServer extends ConfigurableErrorServer with advanced error injection.
type EnhancedErrorServer struct {
	*ConfigurableErrorServer

	// Scenarios holds the enhanced error scenarios
	EnhancedScenarios []EnhancedErrorScenario

	// RequestCounter tracks total requests for frequency calculation
	RequestCounter uint64

	// ErrorCounter tracks errors injected for frequency calculation
	ErrorCounter uint64

	// RandomSource provides deterministic randomness for testing
	RandomSource *rand.Rand
}

// NewEnhancedErrorServer creates a new server with enhanced error injection.
//
// This server supports advanced error scenarios with severity levels,
// frequency control, and probability-based injection.
//
// Parameters:
//   - scenarios: List of enhanced error scenarios
//
// Returns:
//   - *EnhancedErrorServer: Configured enhanced test server
//
// Example:
//
//	scenarios := []EnhancedErrorScenario{
//	    {
//	        Type: ErrorTypeRateLimit,
//	        Severity: ErrorSeverityMedium,
//	        Frequency: 0.3, // 30% of requests
//	        RetryAfter: 5 * time.Second,
//	    },
//	}
//	server := NewEnhancedErrorServer(scenarios)
//	defer server.Close()
func NewEnhancedErrorServer(scenarios []EnhancedErrorScenario) *EnhancedErrorServer {
	// Convert enhanced scenarios to base scenarios
	baseScenarios := make([]ErrorServerScenario, len(scenarios))
	for i, scenario := range scenarios {
		baseScenarios[i] = scenario.ErrorServerScenario
		// Apply enhanced defaults
		if baseScenarios[i].StatusCode == 0 {
			baseScenarios[i].StatusCode = scenario.getDefaultStatusCode()
		}
		if baseScenarios[i].ErrorCode == "" && scenario.ErrorCodeOverride == "" {
			baseScenarios[i].ErrorCode = scenario.getDefaultErrorCode()
		}
		if baseScenarios[i].Message == "" && scenario.MessageOverride == "" {
			baseScenarios[i].Message = scenario.getDefaultMessage()
		}
	}

	baseServer := NewConfigurableErrorServer(baseScenarios)

	return &EnhancedErrorServer{
		ConfigurableErrorServer: baseServer,
		EnhancedScenarios:        scenarios,
		RandomSource:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewEnhancedErrorServerWithSeed creates a server with deterministic randomness.
//
// This variant uses a fixed seed for reproducible test behavior.
//
// Parameters:
//   - scenarios: List of enhanced error scenarios
//   - seed: Random seed for deterministic behavior
//
// Returns:
//   - *EnhancedErrorServer: Configured enhanced test server with deterministic randomness
func NewEnhancedErrorServerWithSeed(scenarios []EnhancedErrorScenario, seed int64) *EnhancedErrorServer {
	server := NewEnhancedErrorServer(scenarios)
	server.RandomSource = rand.New(rand.NewSource(seed))
	return server
}

// ShouldInjectError determines if an error should be injected based on configuration.
func (s *EnhancedErrorServer) ShouldInjectError(scenario EnhancedErrorScenario, r *http.Request) bool {
	atomic.AddUint64(&s.RequestCounter, 1)

	// Check condition first
	if scenario.Condition != nil && !scenario.Condition(r) {
		return false
	}

	// Check intercept flag
	if scenario.Intercept {
		return true
	}

	// Calculate effective probability
	effectiveProbability := scenario.Frequency

	// If both are set, combine them
	if scenario.Probability > 0 && scenario.Frequency > 0 {
		// Independent evaluation: both must pass
		if s.RandomSource.Float64() > scenario.Probability {
			return false
		}
		effectiveProbability = scenario.Frequency
	} else if scenario.Probability > 0 {
		effectiveProbability = scenario.Probability
	}

	// Roll the dice
	shouldInject := s.RandomSource.Float64() < effectiveProbability

	if shouldInject {
		atomic.AddUint64(&s.ErrorCounter, 1)
	}

	return shouldInject
}

// GetErrorRate returns the actual error rate (errors / total requests).
func (s *EnhancedErrorServer) GetErrorRate() float64 {
	total := atomic.LoadUint64(&s.RequestCounter)
	errors := atomic.LoadUint64(&s.ErrorCounter)

	if total == 0 {
		return 0.0
	}

	return float64(errors) / float64(total)
}

// GetStats returns error injection statistics.
func (s *EnhancedErrorServer) GetStats() (total uint64, errors uint64, rate float64) {
	total = atomic.LoadUint64(&s.RequestCounter)
	errors = atomic.LoadUint64(&s.ErrorCounter)
	rate = s.GetErrorRate()
	return
}

// ResetCounters resets the request and error counters.
func (s *EnhancedErrorServer) ResetCounters() {
	atomic.StoreUint64(&s.RequestCounter, 0)
	atomic.StoreUint64(&s.ErrorCounter, 0)
}

// =============================================================================
// ENHANCED ERROR SCENARIO METHODS
// =============================================================================

// getDefaultStatusCode returns the default HTTP status code for the error type.
func (s *EnhancedErrorScenario) getDefaultStatusCode() int {
	switch s.Type {
	case ErrorTypeRateLimit:
		return 429
	case ErrorTypeServiceUnavailable:
		return 503
	case ErrorTypeInternalError:
		return 500
	case ErrorTypeBadRequest:
		return 400
	case ErrorTypeUnauthorized:
		return 401
	case ErrorTypeForbidden:
		return 403
	case ErrorTypeNotFound:
		return 404
	case ErrorTypeTimeout:
		return 408 // Request Timeout
	case ErrorTypeConnectionRefused:
		return 503 // Service Unavailable (simulated)
	case ErrorTypeSlowResponse:
		return 200 // Success, but slow
	default:
		return 500
	}
}

// getDefaultErrorCode returns the default S3 error code for the error type.
func (s *EnhancedErrorScenario) getDefaultErrorCode() string {
	if s.ErrorCodeOverride != "" {
		return s.ErrorCodeOverride
	}

	switch s.Type {
	case ErrorTypeRateLimit:
		return "SlowDown" // S3 rate limit error
	case ErrorTypeServiceUnavailable:
		return "ServiceUnavailable"
	case ErrorTypeInternalError:
		return ErrorCodeInternalError
	case ErrorTypeBadRequest:
		return ErrorCodeInvalidRequest
	case ErrorTypeUnauthorized:
		return "Unauthorized"
	case ErrorTypeForbidden:
		return ErrorCodeAccessDenied
	case ErrorTypeNotFound:
		return ErrorCodeNoSuchKey
	case ErrorTypeTimeout:
		return "RequestTimeout"
	case ErrorTypeConnectionRefused:
		return "ServiceUnavailable"
	case ErrorTypeSlowResponse:
		return "" // No error for slow success
	default:
		return ErrorCodeInternalError
	}
}

// getDefaultMessage returns the default error message for the error type.
func (s *EnhancedErrorScenario) getDefaultMessage() string {
	if s.MessageOverride != "" {
		return s.MessageOverride
	}

	switch s.Type {
	case ErrorTypeRateLimit:
		return "Rate limit exceeded. Please retry later."
	case ErrorTypeServiceUnavailable:
		return "Service is temporarily unavailable. Please retry later."
	case ErrorTypeInternalError:
		return "An internal error occurred. Please retry."
	case ErrorTypeBadRequest:
		return "The request is invalid or malformed."
	case ErrorTypeUnauthorized:
		return "Authentication is required to access this resource."
	case ErrorTypeForbidden:
		return "Access to this resource is denied."
	case ErrorTypeNotFound:
		return "The specified resource does not exist."
	case ErrorTypeTimeout:
		return "The request timed out. Please retry."
	case ErrorTypeConnectionRefused:
		return "Connection to the service failed. Please check your network and retry."
	case ErrorTypeSlowResponse:
		return "Request processed successfully"
	default:
		return "An error occurred. Please retry."
	}
}

// =============================================================================
// CONVENIENCE FUNCTIONS FOR ENHANCED SCENARIOS
// =============================================================================

// NewRateLimitScenario creates a rate limit error scenario.
//
// This convenience function creates a scenario for testing rate limit handling.
//
// Parameters:
//   - frequency: How often to inject this error (0.0 to 1.0)
//   - retryAfter: Suggested retry duration
//
// Returns:
//   - EnhancedErrorScenario: Configured rate limit scenario
func NewRateLimitScenario(frequency float64, retryAfter time.Duration) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 429,
			ErrorCode:  "SlowDown",
			Message:    "Rate limit exceeded. Please retry later.",
			Headers: map[string]string{
				"Content-Type":  "application/xml",
				"Retry-After":   fmt.Sprintf("%.0f", retryAfter.Seconds()),
				"X-RateLimit-Limit": "100",
				"X-RateLimit-Remaining": "0",
				"X-RateLimit-Reset": fmt.Sprintf("%d", time.Now().Add(retryAfter).Unix()),
			},
		},
		Type:       ErrorTypeRateLimit,
		Severity:   ErrorSeverityMedium,
		Frequency:  frequency,
		RetryAfter: retryAfter,
	}
}

// NewTimeoutScenario creates a timeout error scenario.
//
// This convenience function creates a scenario for testing timeout handling.
//
// Parameters:
//   - frequency: How often to inject this error (0.0 to 1.0)
//   - delay: Delay before responding (simulates slow server)
//
// Returns:
//   - EnhancedErrorScenario: Configured timeout scenario
func NewTimeoutScenario(frequency float64, delay time.Duration) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 408,
			ErrorCode:  "RequestTimeout",
			Message:    "The request timed out. Please retry.",
			Delay:      delay,
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      ErrorTypeTimeout,
		Severity:  ErrorSeverityHigh,
		Frequency: frequency,
	}
}

// NewConnectionRefusedScenario creates a connection refused scenario.
//
// This convenience function creates a scenario that simulates connection failures.
// Note: Since we can't actually refuse connections in httptest, we return 503.
//
// Parameters:
//   - frequency: How often to inject this error (0.0 to 1.0)
//
// Returns:
//   - EnhancedErrorScenario: Configured connection refused scenario
func NewConnectionRefusedScenario(frequency float64) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 503,
			ErrorCode:  "ServiceUnavailable",
			Message:    "Connection to the service failed. Please check your network and retry.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      ErrorTypeConnectionRefused,
		Severity:  ErrorSeverityCritical,
		Frequency: frequency,
	}
}

// NewSlowResponseScenario creates a slow response scenario.
//
// This convenience function creates a scenario for testing slow (but successful) responses.
//
// Parameters:
//   - frequency: How often to inject this scenario (0.0 to 1.0)
//   - delay: Delay before responding
//
// Returns:
//   - EnhancedErrorScenario: Configured slow response scenario
func NewSlowResponseScenario(frequency float64, delay time.Duration) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 200,
			Message:    "Request processed successfully",
			Delay:      delay,
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      ErrorTypeSlowResponse,
		Severity:  ErrorSeverityLow,
		Frequency: frequency,
	}
}

// NewServiceUnavailableScenario creates a service unavailable error scenario.
//
// This convenience function creates a scenario for testing 503 error handling.
//
// Parameters:
//   - frequency: How often to inject this error (0.0 to 1.0)
//   - retryAfter: Suggested retry duration
//
// Returns:
//   - EnhancedErrorScenario: Configured service unavailable scenario
func NewServiceUnavailableScenario(frequency float64, retryAfter time.Duration) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 503,
			ErrorCode:  "ServiceUnavailable",
			Message:    "Service is temporarily unavailable. Please retry later.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
				"Retry-After":  fmt.Sprintf("%.0f", retryAfter.Seconds()),
			},
		},
		Type:       ErrorTypeServiceUnavailable,
		Severity:   ErrorSeverityHigh,
		Frequency:  frequency,
		RetryAfter: retryAfter,
	}
}

// =============================================================================
// SEVERITY-BASED SCENARIOS
// =============================================================================

// NewLowSeverityScenario creates a low severity error scenario.
//
// Low severity errors typically don't block operations and may be transient.
func NewLowSeverityScenario(errorType ErrorType, frequency float64) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 500,
			Message:    "A minor error occurred. Retrying may succeed.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      errorType,
		Severity:  ErrorSeverityLow,
		Frequency: frequency,
	}
}

// NewMediumSeverityScenario creates a medium severity error scenario.
//
// Medium severity errors partially impact operations and may require retry logic.
func NewMediumSeverityScenario(errorType ErrorType, frequency float64) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 500,
			Message:    "An error occurred that may impact operations. Please retry.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      errorType,
		Severity:  ErrorSeverityMedium,
		Frequency: frequency,
	}
}

// NewHighSeverityScenario creates a high severity error scenario.
//
// High severity errors block operations and require immediate attention.
func NewHighSeverityScenario(errorType ErrorType, frequency float64) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 500,
			Message:    "A critical error occurred. Operations are blocked.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
			},
		},
		Type:      errorType,
		Severity:  ErrorSeverityHigh,
		Frequency: frequency,
	}
}

// NewCriticalSeverityScenario creates a critical severity error scenario.
//
// Critical severity errors indicate system failure requiring immediate intervention.
func NewCriticalSeverityScenario(errorType ErrorType, frequency float64) EnhancedErrorScenario {
	return EnhancedErrorScenario{
		ErrorServerScenario: ErrorServerScenario{
			StatusCode: 503,
			Message:    "System failure. Immediate intervention required.",
			Headers: map[string]string{
				"Content-Type": "application/xml",
				"Retry-After":  "60", // Suggest longer retry
			},
		},
		Type:      errorType,
		Severity:  ErrorSeverityCritical,
		Frequency: frequency,
	}
}

// =============================================================================
// PRECONFIGURED ENHANCED SCENARIOS
// =============================================================================

// PreconfiguredEnhancedScenarios provides ready-to-use enhanced error scenarios.
var PreconfiguredEnhancedScenarios = struct {
	// OccasionalRateLimit injects rate limits 10% of the time
	OccasionalRateLimit EnhancedErrorScenario

	// FrequentTimeouts injects timeouts 30% of the time with 5s delay
	FrequentTimeouts EnhancedErrorScenario

	// RareConnectionRefused injects connection errors 5% of the time
	RareConnectionRefused EnhancedErrorScenario

	// SlowResponses injects 2s delays 20% of the time
	SlowResponses EnhancedErrorScenario

	// FrequentServiceUnavailable injects 503 errors 25% of the time
	FrequentServiceUnavailable EnhancedErrorScenario
}{
	OccasionalRateLimit: NewRateLimitScenario(0.10, 5*time.Second),
	FrequentTimeouts:    NewTimeoutScenario(0.30, 5*time.Second),
	RareConnectionRefused: NewConnectionRefusedScenario(0.05),
	SlowResponses:       NewSlowResponseScenario(0.20, 2*time.Second),
	FrequentServiceUnavailable: NewServiceUnavailableScenario(0.25, 10*time.Second),
}
