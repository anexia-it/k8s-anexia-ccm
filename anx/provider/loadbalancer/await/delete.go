package await

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
)

func Deleted(ctx context.Context, obj types.IdentifiedObject) error {
	client, err := getClient()
	if err != nil {
		return err
	}
	logger := logr.FromContextOrDiscard(ctx).V(2)
	until(30*time.Second, func() (bool, error) {
		logger.Info("waiting for resource to be deleted", "type", fmt.Sprintf("%T", obj))
		err = client.Get(ctx, obj)
		if errors.Is(err, api.ErrNotFound) {
			return true, nil
		}

		if err != nil {
			return false, err
		}

		return false, nil
	})

	return nil
}
