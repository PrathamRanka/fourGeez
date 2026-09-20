package publicationops

import "context"

// NoopCacheInvalidator is used when authoritative DynamoDB reads are enabled
// without a distributed publication cache.
type NoopCacheInvalidator struct{}

func (NoopCacheInvalidator) InvalidateSellerPublication(context.Context, string, uint64) error {
	return nil
}
