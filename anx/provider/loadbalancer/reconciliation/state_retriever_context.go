package reconciliation

import (
	"context"

	"go.anx.io/go-anxcloud/pkg/api"
)

type stateRetrieverContextKey string

var withStateRetrieverKey stateRetrieverContextKey = "withStateRetrieverKey"

func withStateRetriever(ctx context.Context, a api.API, svcUID string, lbCount int) context.Context {
	sr := newStateRetriever(ctx, a, svcUID, lbCount)
	return context.WithValue(ctx, withStateRetrieverKey, sr)
}

func getOrCreateStateRetriever(ctx context.Context, recon *reconciliation) (stateRetriever, func()) {
	sr := ctx.Value(withStateRetrieverKey)
	if sr == nil {
		sr = newStateRetriever(ctx, recon.api, recon.serviceUID, 1)
	}

	return sr.(stateRetriever), func() {
		// make Done call no-op when state retriever borrowed
		borrowed := ctx.Value(withBorrowedStateRetrieverKey)
		if borrowed == nil || borrowed.(bool) == false {
			sr.(stateRetriever).Done(recon.lb.Identifier)
		}
	}
}

var withBorrowedStateRetrieverKey stateRetrieverContextKey = "withBorrowedStateRetrieverKey"

func withBorrowedStateRetriever(ctx context.Context) context.Context {
	return context.WithValue(ctx, withBorrowedStateRetrieverKey, true)
}
