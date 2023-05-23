package mock

import (
	"context"
)

// GQLClient is the mock http client
type GQLClient struct {
	DoQuery func(ctx context.Context, q interface{}, variables map[string]interface{}) error
}

func (c *GQLClient) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	return c.DoQuery(ctx, q, variables)
}
