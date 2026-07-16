// Package backend provides low-level multipart upload operations for S3-compatible storage.
//
// Multipart Upload Primitives
//
// This package provides low-level multipart upload operations that can be used by
// canary monitors and other testing tools to exercise the full multipart upload code path.
//
// The core multipart upload operations are:
//
//   CreateMultipartUpload(ctx, bucket, key, metadata) -> uploadID
//     Initiates a multipart upload and returns an upload ID that must be used for
//     subsequent UploadPart and CompleteMultipartUpload calls.
//
//   UploadPart(ctx, bucket, key, uploadID, partNumber, body, size) -> partETag
//     Uploads a single part to a multipart upload. Parts must be at least 5MB
//     (except the last part) and part numbers must be >= 1.
//
//   CompleteMultipartUpload(ctx, bucket, key, uploadID, parts) -> finalETag
//     Completes a multipart upload by combining all uploaded parts. Parts must
//     be sorted by part number and include all uploaded parts with their ETags.
//
//   AbortMultipartUpload(ctx, bucket, key, uploadID)
//     Cancels a multipart upload and releases all storage associated with it.
//     Should be called for cleanup when an upload fails.
//
// Helper Functions
//
// The MultipartUploadHelper provides higher-level helper functions for common
// multipart upload scenarios:
//
//   NewMultipartUploadHelper(backend, bucket) -> helper
//     Creates a helper for a specific bucket.
//
//   helper.CreateUpload(ctx, key, metadata) -> uploadInfo
//     Creates a new multipart upload.
//
//   helper.UploadPart(ctx, uploadInfo, partNumber, data) -> error
//     Uploads a single part (includes validation).
//
//   helper.CompleteUpload(ctx, uploadInfo) -> finalETag
//     Completes the multipart upload.
//
//   helper.AbortUpload(ctx, uploadInfo) -> error
//     Cancels the multipart upload.
//
//   helper.CanaryUpload(ctx, key, data) -> error
//     Performs a simple canary test of the multipart upload system.
//
//   VerifyMultipartUpload(ctx, backend, bucket, key, data, partSize) -> error
//     Full verification: uploads via multipart, downloads back, and verifies integrity.
//
// S3 Multipart Upload Thresholds and Limits
//
//   Minimum part size (except last part): 5MB (5,242,880 bytes)
//   Maximum part size: 5GB (5,368,709,120 bytes)
//   Maximum number of parts: 10,000
//   Maximum object size via multipart: 5TB (5,497,558,138,880 bytes)
//   Recommended part size: 10MB to 100MB for efficiency
//
// For ARMOR:
//   Part sizes should be multiples of the encryption block size (4096 bytes)
//   Typical ARMOR part sizes: 5MB, 10MB, 50MB (all multiples of 4096)
//
// Usage Examples
//
// Basic multipart upload:
//
//   helper := NewMultipartUploadHelper(backend, "my-bucket")
//
//   // Create upload
//   info, err := helper.CreateUpload(ctx, "test-key", map[string]string{
//       "Content-Type": "application/octet-stream",
//   })
//   if err != nil {
//       return err
//   }
//
//   // Upload parts
//   parts := [][]byte{part1Data, part2Data, part3Data}
//   for i, partData := range parts {
//       err = helper.UploadPart(ctx, info, i+1, partData)
//       if err != nil {
//           helper.AbortUpload(ctx, info) // cleanup on error
//           return err
//       }
//   }
//
//   // Complete upload
//   finalETag, err := helper.CompleteUpload(ctx, info)
//   if err != nil {
//       return err
//   }
//
// Canary monitor usage:
//
//   // Simple canary test
//   testKey := fmt.Sprintf("canary-test-%d", time.Now().Unix())
//   testData := bytes.Repeat([]byte{0xAB}, 6*1024*1024) // 6MB
//
//   err := helper.CanaryUpload(ctx, testKey, testData)
//   if err != nil {
//       log.Printf("Canary test failed: %v", err)
//       return err
//   }
//
//   // Full verification test
//   err = VerifyMultipartUpload(ctx, backend, "my-bucket", testKey, testData, 5*1024*1024)
//   if err != nil {
//       log.Printf("Verification test failed: %v", err)
//       return err
//   }
//
// Mock Backend for Testing
//
// The NewMockB2BackendForTesting() function creates a mock backend that simulates
// S3 multipart upload behavior without requiring real credentials. This is useful
// for canary monitors and other testing scenarios.
//
//   mockBackend := NewMockB2BackendForTesting()
//   helper := NewMultipartUploadHelper(mockBackend, "test-bucket")
//
//   // Perform test upload
//   info, _ := helper.CreateUpload(ctx, "test-key", nil)
//   helper.UploadPart(ctx, info, 1, testPart1Data)
//   helper.UploadPart(ctx, info, 2, testPart2Data)
//   finalETag, _ := helper.CompleteUpload(ctx, info)
//
// Error Handling
//
// All multipart operations return errors that should be checked and handled.
// Common errors include:
//
//   - NoSuchBucket: The specified bucket does not exist
//   - NoSuchUpload: The specified multipart upload does not exist
//   - InvalidPart: Part number or ETag is invalid
//   - EntityTooSmall: Part size is below minimum (5MB)
//   - AccessDenied: Insufficient permissions
//
// Always call AbortMultipartUpload for cleanup when an upload fails.
//
// Context Cancellation
//
// All multipart operations respect context cancellation. If the context is
// canceled, operations will return an error indicating cancellation.
//
package backend
