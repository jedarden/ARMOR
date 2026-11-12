package backend

import (
	"context"
)

// ConditionalCopyBackend publishes a staged object over an existing object
// only when the destination still has the ETag observed before migration.
// Implementations must perform the comparison as part of the publish
// operation; a separate Head followed by Copy is not sufficient.
type ConditionalCopyBackend interface {
	CopyIfMatch(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey, expectedETag string, meta map[string]string, replaceMetadata bool) error
}
