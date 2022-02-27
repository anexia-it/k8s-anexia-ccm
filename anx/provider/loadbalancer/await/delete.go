package await

import (
	"context"
	"errors"
	"time"

	"go.anx.io/go-anxcloud/pkg/api"
	"go.anx.io/go-anxcloud/pkg/api/types"
)

func Deleted(ctx context.Context, obj types.IdentifiedObject) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	until(30*time.Second, func() (bool, error) {
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
