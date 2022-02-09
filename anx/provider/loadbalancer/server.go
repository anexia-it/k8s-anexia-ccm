package loadbalancer

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"go.anx.io/go-anxcloud/pkg/lbaas/common"
	"go.anx.io/go-anxcloud/pkg/lbaas/server"
	"go.anx.io/go-anxcloud/pkg/pagination"
)

func ensureBackendServerInLoadBalancer(ctx context.Context, lb LoadBalancer,
	name string, endpoint NodeEndpoint) (server.Server, error) {
	if lb.State.BackendID == "" {
		return server.Server{}, errors.New("can't attach server to state without backend")
	}

	serverName := getServerName(name, endpoint)
	existingServer := findServersByBackendInLB(ctx, lb, name, serverName)
	if len(existingServer) > 1 {
		lb.Logger.Info("[WARN] search for server returned too many results", "name", serverName,
			"resource", "server")
	}

	if len(existingServer) == 0 {
		return createServerForLB(ctx, lb, name, endpoint)
	}

	fetchedServer := existingServer[0]
	if fetchedServer.Backend.Identifier != string(lb.State.BackendID) ||
		fetchedServer.Port != int(endpoint.Port) ||
		fetchedServer.IP != endpoint.IP {
		lb.Logger.Info("updating server", "name", name, "resource", "server")
		updatedServer, err := lb.Server().Update(ctx, fetchedServer.Identifier,
			getServerDefinition(name, endpoint, lb.State))

		return updatedServer, err
	}
	return *fetchedServer, nil
}

func getServerName(lbName string, endpoint NodeEndpoint) string {
	return fmt.Sprintf("%x.%s", md5.Sum([]byte(endpoint.IP)), lbName)
}

func createServerForLB(ctx context.Context, lb LoadBalancer, name string,
	endpoint NodeEndpoint) (server.Server, error) {
	definition := getServerDefinition(name, endpoint, lb.State)

	createdServer, err := lb.Server().Create(ctx, definition)
	return createdServer, err
}

func getServerDefinition(name string, endpoint NodeEndpoint, state *state) server.Definition {
	definition := server.Definition{
		Name:    getServerName(name, endpoint),
		State:   common.NewlyCreated,
		IP:      endpoint.IP,
		Port:    int(endpoint.Port),
		Backend: string(state.BackendID),
	}
	return definition
}

func findServersByBackendInLB(ctx context.Context, lb LoadBalancer, lbName, suffix string) []*server.Server {
	var fetchedServers []*server.Server
	if suffix == "" {
		suffix = lbName
	}
	servers, cancelFunc := pagination.AsChan(ctx, lb.Server(), SearchParameter(suffix))
	defer cancelFunc()

	for elem := range servers {
		serverInfo := elem.(server.ServerInfo)
		fetchedServer, err := lb.Server().GetByID(ctx, serverInfo.Identifier)
		if err != nil {
			return nil
		}
		if fetchedServer.Backend.Identifier == string(lb.State.BackendID) {
			fetchedServers = append(fetchedServers, &fetchedServer)
		}
	}
	return fetchedServers
}

func deleteServersFromBackendInLB(ctx context.Context, g LoadBalancer, name string) error {
	servers := findServersByBackendInLB(ctx, g, name, "")
	for _, server := range servers {
		err := g.Server().DeleteByID(ctx, server.Identifier)
		if err != nil {
			return err
		}
	}
	return nil
}
