package reconciliation

import (
	"context"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
)

type stateRetrieverContextKey string

var withStateRetrieverKey stateRetrieverContextKey = "withStateRetrieverKey"

// WithStateRetriever adds a shared state retriever to the context
func WithStateRetriever(ctx context.Context, a api.API, svcUID string, lbIdentifiers []string) context.Context {
	sr := newStateRetriever(ctx, a, svcUID, lbIdentifiers)
	return context.WithValue(ctx, withStateRetrieverKey, sr)
}

// stateRetrieverFromContextOrNew extracts the stateRetriever from context if attached or creates a new
// it returns the stateRetriever and a unsubscribe function which MUST be called excactly once per registered load balancer
func stateRetrieverFromContextOrNew(ctx context.Context, recon *reconciliation) (stateRetriever, func()) {
	sr := ctx.Value(withStateRetrieverKey)
	if sr == nil {
		sr = newStateRetriever(ctx, recon.api, recon.serviceUID, []string{recon.lb.Identifier})
	}

	return sr.(stateRetriever), func() {
		if err := sr.(stateRetriever).Done(recon.lb.Identifier); err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "Failed to unsubscribe from state retriever.")
		}
	}
}
