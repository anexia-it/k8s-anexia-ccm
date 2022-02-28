package await

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg/api"
	lbaas "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
	"go.anx.io/go-anxcloud/pkg/client"
)

var (
	// containing all the states that stand for a successfull change to the lbaas resource
	SuccessStates = []lbaas.State{lbaas.Updated, lbaas.Deployed}

	// containing all the states that stand for a failed change to the lbaas resource
	FailedStates = []lbaas.State{lbaas.DeploymentError}
)

func BackendState(ctx context.Context, identifier string, states ...lbaas.State) error {
	backend := lbaas.Backend{Identifier: identifier}
	anxClient, err := getClient()
	logger := logr.FromContextOrDiscard(ctx).V(2)

	until(1*time.Minute, func() (bool, error) {
		logger.Info("waiting for backend to reach desired state", "identifier", identifier)
		err = anxClient.Get(ctx, &backend)
		if err != nil {
			return false, err
		}

		if matchesAny(backend.State, states) {
			return true, nil
		}

		return false, nil
	})

	return nil
}

func FrontendState(ctx context.Context, identifier string, states ...lbaas.State) error {
	frontend := lbaas.Frontend{Identifier: identifier}
	anxClient, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	logger := logr.FromContextOrDiscard(ctx).V(2)

	until(1*time.Minute, func() (bool, error) {
		logger.Info("waiting for frontend to reach desired state", "identifier", identifier)
		err = anxClient.Get(ctx, &frontend)
		if err != nil {
			return false, err
		}

		if matchesAny(frontend.State, states) {
			return true, nil
		}

		return false, nil
	})

	return nil
}

func BindState(ctx context.Context, identifier string, states ...lbaas.State) error {
	bind := lbaas.Bind{Identifier: identifier}
	anxClient, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	logger := logr.FromContextOrDiscard(ctx).V(2)

	until(1*time.Minute, func() (bool, error) {
		logger.Info("waiting for bind to reach desired state", "identifier", identifier)
		err = anxClient.Get(ctx, &bind)
		if err != nil {
			return false, err
		}

		if matchesAny(bind.State, states) {
			return true, nil
		}

		return false, nil
	})

	return nil
}

func ServerState(ctx context.Context, identifier string, states ...lbaas.State) error {
	server := lbaas.Server{Identifier: identifier}
	anxClient, err := api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
	logger := logr.FromContextOrDiscard(ctx).V(2)

	until(1*time.Minute, func() (bool, error) {
		logger.Info("waiting for server to reach desired state", "identifier", identifier)
		err = anxClient.Get(ctx, &server)
		if err != nil {
			return false, err
		}

		if matchesAny(server.State, states) {
			return true, nil
		}

		return false, nil
	})

	return nil
}

func until(timeout time.Duration, conditionCode func() (bool, error)) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return errors.New("timeout when waiting for condition")
		default:
			isDone, err := conditionCode()
			if err != nil {
				return fmt.Errorf("error when waiting for condition: %w", err)
			}
			if isDone {
				return nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func matchesAny(state lbaas.State, matches []lbaas.State) bool {
	for _, matchState := range matches {
		if state.ID == matchState.ID {
			return true
		}
	}
	return false
}

func getClient() (api.API, error) {

	return api.NewAPI(api.WithClientOptions(client.TokenFromEnv(false)))
}
