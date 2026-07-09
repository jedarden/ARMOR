# Dashboard Test Coverage (bf-4va60)

## Summary
Added comprehensive tests for the two previously untested dashboard handlers:
- `KeyRotateStatusHandler` - Returns key rotation status/progress
- `KeyRotateHandler` - Initiates key rotation

## Tests Added

### KeyRotateStatusHandler (3 tests)
1. `TestKeyRotateStatusHandlerNoRotation` - Verifies "none" status when no rotation state file exists
2. `TestKeyRotateStatusHandlerWithAuth` - Verifies authentication wrapper works with Bearer token
3. `TestKeyRotateStatusHandlerMethodNotAllowed` - Verifies non-GET requests are rejected

### KeyRotateHandler (5 tests)
1. `TestKeyRotateHandlerSuccess` - Verifies successful rotation initiation with mock admin API
2. `TestKeyRotateHandlerWithAuth` - Verifies authentication wrapper works with Basic Auth
3. `TestKeyRotateHandlerMethodNotAllowed` - Verifies non-POST requests are rejected
4. `TestKeyRotateHandlerAdminAPIFailure` - Verifies error handling when admin API returns error response
5. `TestKeyRotateHandlerDefaultURL` - Verifies default admin URL (localhost:9001) is used when none provided

## Fix Applied
Modified `mockBackend.GetDirect()` to return an error instead of nil values, simulating "file not found" behavior. This prevents nil pointer dereference when the key rotation status handler attempts to read from a non-existent rotation state file.

## Coverage Status
All dashboard handlers now have test coverage:
- ✅ Main dashboard page (Handler)
- ✅ Object detail endpoint (ObjectDetailHandler)
- ✅ Metrics endpoint (MetricsHandler)
- ✅ Key rotation status endpoint (KeyRotateStatusHandler) - **NOW TESTED**
- ✅ Key rotation endpoint (KeyRotateHandler) - **NOW TESTED**
- ✅ Encryption stats endpoint (EncryptionStatsHandler)
- ✅ JSON list API endpoint (ListAPIHandler)

All authentication wrappers are also tested.
