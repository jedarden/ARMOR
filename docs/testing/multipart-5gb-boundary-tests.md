# 5 GiB Multipart Boundary Test Coverage

## Overview

Comprehensive regression coverage for the B2 large-object finalization incident (5 GiB CopyObject limit). This test suite ensures ARMOR can handle objects at and above the 5 GiB boundary without data loss or corruption.

## Background

### The Incident

B2's `CopyObject` operation (used by the old multipart finalization approach) is limited to 5 GiB source objects. For larger files:

- **Single-CopyObject approach**: Fails with `EntityTooLarge` or times out
- **ADR-016 manifest approach**: Works at any size (no CopyObject required)

### Test Strategy

The test suite has two components:

1. **Unit tests** (`internal/server/handlers/multipart_5gb_boundary_test.go`):
   - Use mocks and sparse fixtures
   - Run in normal test suite
   - No multi-gigabyte allocation
   - Test failures, timeouts, retries, and edge cases

2. **Integration test** (`tests/integration/s3_multipart_5gb_boundary_integration_test.go`):
   - Gated by `integration` build tag
   - Uploads actual >5 GiB object to real B2
   - Verifies byte-perfect roundtrip
   - Confirms ADR-016 implementation

## Running the Tests

### Unit Tests (Fast, No Credentials)

```bash
# Run all 5 GiB boundary unit tests
go test -v ./internal/server/handlers/... -run TestMultipartFinalization

# Run specific test
go test -v ./internal/server/handlers/... -run TestMultipartFinalization_5GiB_Boundary_CopyObjectRejected
```

These tests use mock backends and complete in seconds.

### Integration Tests (Requires B2 Credentials)

```bash
# Set environment variables
export ARMOR_INTEGRATION_TEST=1
export ARMOR_B2_ACCESS_KEY_ID="your-b2-key-id"
export ARMOR_B2_SECRET_ACCESS_KEY="your-b2-secret"
export ARMOR_B2_REGION="us-east-005"
export ARMOR_BUCKET="your-test-bucket"
export ARMOR_CF_DOMAIN="your-bucket-domain.b2.dev"
export ARMOR_MEK="32-byte-hex-master-encryption-key"
export ARMOR_AUTH_ACCESS_KEY="test-access-key"
export ARMOR_AUTH_SECRET_KEY="test-secret-key"
export ARMOR_ENDPOINT="http://localhost:9000"

# Run integration test with 6 GiB object
go test -v -tags=integration ./tests/integration/... -run TestMultipart5GB_B2Integration

# Run boundary edge cases (4.9 GiB, 5 GiB, 5.1 GiB)
go test -v -tags=integration ./tests/integration/... -run TestMultipart5GB_BoundaryEdgeCases
```

## Cost Estimates

### B2 Storage Costs

- **Rate**: $0.006/GB/month
- **Test object size**: 6 GiB
- **Duration**: ~1 hour per test run
- **Cost**: 6 GB × $0.006/GB/month × (1 hour / 730 hours/month) = **~$0.00004** per run

### B2 API Costs

- **Class B transactions**: CreateMultipartUpload, UploadPart, CompleteMultipartUpload
- **Estimated**: 1100 requests (1 create + 1000 parts + 1 complete + cleanup)
- **Cost**: ~1000 Class B × $0.004/1000 requests = **~$0.004** per run

### B2 Download Egress

- **Free tier**: 1 GB/day free
- **Test download**: 10 MB (ranged verification)
- **Cost**: **$0** (within free tier)

### Total Cost Per Test Run

**~$0.004** (less than one cent)

## What the Tests Verify

### Unit Tests (Mock Backend)

1. **Boundary Behavior**:
   - Objects < 5 GiB: CopyObject succeeds
   - Objects >= 5 GiB: CopyObject fails with EntityTooLarge
   - Confirms the old approach breaks at the boundary

2. **Timeout Scenarios**:
   - CopyObject timeout during finalization
   - Upload ID already consumed (B2 Complete succeeded)
   - Client retry behavior after timeout

3. **Process Restart**:
   - Restart between each step (create, upload parts, complete, metadata write)
   - State recovery from durable storage
   - No data loss across restart

4. **Concurrent Writers**:
   - Two writers to same key
   - B2 upload ID serialization
   - Last writer wins

5. **Metadata Verification**:
   - Exact plaintext size
   - Real plaintext SHA-256 (not placeholder)
   - ETag correctness
   - ARMOR metadata completeness

6. **Ranged GET**:
   - Ranges spanning part boundaries
   - Byte-perfect verification

7. **Cache Loss**:
   - Metadata cache eviction
   - Footer cache eviction
   - Object still readable without cache

### Integration Tests (Real B2)

1. **6 GiB Object Upload**:
   - 100 MB parts × 60 parts = 6 GiB
   - Upload time and throughput
   - Finalization succeeds

2. **Metadata Verification**:
   - All ARMOR metadata present
   - Manifest object exists (ADR-016)
   - Ciphertext reference correct
   - Completed-at timestamp

3. **Download Verification**:
   - Full GET: correct size and data
   - Ranged GET (first 10 MB): byte-perfect
   - Ranged GET across boundary: correct
   - SHA-256 verification

4. **HMAC Sidecar**:
   - Sidecar object exists
   - Correct format (v3 gzip JSON)

5. **Boundary Edge Cases**:
   - 4.9 GiB: CopyObject succeeds (old approach works)
   - 5.0 GiB: Requires manifest approach
   - 5.1 GiB: Requires manifest approach

## CI Configuration

### iad-ci Argo Workflow Template

The integration test should run in `iad-ci` with opt-in credentials:

```yaml
# WorkflowTemplate: armor-5gb-boundary-test
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: armor-5gb-boundary-test
  namespace: argo-workflows
spec:
  entrypoint: test
  templates:
  - name: test
    inputs:
      parameters:
      - name: armor-endpoint
      - name: b2-bucket
      - name: b2-key-id
        value: "{{ secrets.B2_TEST_KEY_ID }}"
      - name: b2-key-secret
        value: "{{ secrets.B2_TEST_KEY_SECRET }}"
      - name: armor-mek
        value: "{{ secrets.ARMOR_MEK }}"
    container:
      image: ronaldraygun/armor:latest  # Use built ARMOR image
      command: [sh]
      args:
      - -c
      - |
        # Set up environment
        export ARMOR_INTEGRATION_TEST=1
        export ARMOR_ENDPOINT="{{inputs.parameters.armor-endpoint}}"
        export ARMOR_B2_ACCESS_KEY_ID="{{inputs.parameters.b2-key-id}}"
        export ARMOR_B2_SECRET_ACCESS_KEY="{{inputs.parameters.b2-key-secret}}"
        export ARMOR_B2_REGION="us-east-005"
        export ARMOR_BUCKET="{{inputs.parameters.b2-bucket}}"
        export ARMOR_CF_DOMAIN="{{inputs.parameters.b2-bucket}}.b2.dev"
        export ARMOR_MEK="{{inputs.parameters.armor-mek}}"
        export ARMOR_AUTH_ACCESS_KEY="test-access-key"
        export ARMOR_AUTH_SECRET_KEY="test-secret-key"

        # Start ARMOR server in background
        /armor serve &
        sleep 10

        # Run 5 GiB boundary integration test
        go test -v -tags=integration -timeout=3h \
          ./tests/integration/... -run TestMultipart5GB_B2Integration
    retryStrategy:
      limit: 2
```

### Opt-In Execution

To run the test manually from `iad-ci`:

```bash
kubectl --server=http://traefik-iad-ci:8001 create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: armor-5gb-boundary-test-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: armor-5gb-boundary-test
  arguments:
    parameters:
    - name: armor-endpoint
      value: "http://armor-service.armor.svc.cluster.local:9000"
    - name: b2-bucket
      value: "armor-integration-tests"
EOF
```

## Failure Diagnosis

### Unit Test Failures

| Test Name | Failure Symptom | Likely Cause |
|-----------|----------------|--------------|
| `TestMultipartFinalization_5GiB_Boundary_CopyObjectRejected/at_5gb_fails` | No EntityTooLarge error | CopyObject limit not enforced |
| `TestMultipartFinalization_CopyObjectTimeout` | No timeout error | Timeout not configured |
| `TestMultipartFinalization_ProcessRestartBetweenSteps` | State lost after restart | State persistence broken |
| `TestMultipartFinalization_ConcurrentSameKeyWriters` | Same upload ID for both writers | Upload ID collision |
| `TestMultipartFinalization_MetadataVerification` | Missing SHA-256 or wrong size | Metadata calculation bug |

### Integration Test Failures

| Symptom | Likely Cause | Resolution |
|---------|--------------|------------|
| `EntityTooLarge` at >=5 GiB | ADR-016 not implemented | Implement manifest-based finalization |
| Timeout during Complete | Network issue or slow B2 | Increase timeout, check connectivity |
| SHA-256 mismatch | Data corruption | Check encryption/decryption |
| Missing manifest | Manifest write failed | Check ADR-016 implementation |
| Ranged GET fails | Range calculation bug | Check boundary math |

## Related Documentation

- **ADR-016**: B2-safe multipart metadata finalization protocol
- **ADR-005**: Out-of-order multipart uploads
- **ADR-003**: Multipart object layout and read path
- `internal/backend/multipart.go`: Multipart state management
- `internal/server/handlers/handlers.go`: CompleteMultipartUpload handler

## Maintenance

### Updating Test Sizes

If B2 changes the CopyObject limit:

1. Update `B2CopyObjectSizeCeiling` in test files
2. Update test size constants (`sizeJustBelow`, `sizeAtBoundary`, `sizeJustAbove`)
3. Update documentation with new limit

### Adding New Edge Cases

To add a new test case:

1. Add function to `multipart_5gb_boundary_test.go`
2. Name with `TestMultipartFinalization_` prefix
3. Use `entityTooLargeBackend` mock for CopyObject behavior
4. Verify failure against old approach, success with ADR-016

## Monitoring

### CI Metrics to Track

- Test duration (should be <2 hours for integration test)
- B2 API cost per run
- Failure rate by test name
- Timeouts vs. hard failures

### Alerts

- Integration test timeout (>3 hours)
- Cost spike (>10× normal)
- Persistent failures across 3+ runs
- SHA-256 mismatch (data corruption)

## Future Work

1. **Add 10 GiB test**: Even larger object stress test
2. **Concurrent large uploads**: Multiple >5 GiB objects simultaneously
3. **Network interruption testing**: Simulate failures during large upload
4. **Performance benchmarks**: Upload throughput vs. object size
