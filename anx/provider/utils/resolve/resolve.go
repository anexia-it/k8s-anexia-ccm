package resolve

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-logr/logr"
	"go.anx.io/go-anxcloud/pkg"
	"strings"
)

// NodeResolver takes a context and a node name and responds with the corresponding server ID on Anexia side.
type NodeResolver interface {
	Resolve(context.Context, string) (string, error)
}

// The CustomerPrefixResolver takes the CustomerPrefix from anexia and prepends it to every name that is passed into
// its `Resolve` function with the following pattern `%s-%s`.
type CustomerPrefixResolver struct {
	pkg.API
	CustomerPrefix string
}

func (c CustomerPrefixResolver) Resolve(ctx context.Context, name string) (string, error) {
	vms, err := c.VSphere().Search().ByName(ctx, fmt.Sprintf("%s-%s", c.CustomerPrefix, name))
	if err != nil {
		return "", err
	}

	if len(vms) != 1 {
		return "", fmt.Errorf("expected 1 VM with the name '%s', but found %d", name, len(vms))
	}

	return vms[0].Identifier, nil
}

// AutomaticResolver tries to find out what the customer prefix is by looking at the names of already provisioned VMs
// If `UseCache` is `true` the customerPrefix is only fetched once and then cached forever.
type AutomaticResolver struct {
	pkg.API
	UseCache       bool
	customerPrefix string
}

func (receiver *AutomaticResolver) Resolve(ctx context.Context, name string) (string, error) {
	if receiver.customerPrefix == "" || !receiver.UseCache {
		vms, err := receiver.VSphere().VMList().Get(ctx, 1, 1)
		if err != nil {
			return "", fmt.Errorf("could not list vms to obtain the customer prefix: %w", err)
		}

		if len(vms) == 0 {
			return "", errors.New("could not obtain customer prefix since there is no VM present")
		}

		nameParts := strings.Split(vms[0].Name, "-")
		if len(nameParts) < 2 {
			return "", errors.New("unexpected vm name format. expected [customerPrefix]-[vmName]." +
				" could not obtain customerPrefix")
		}
		receiver.customerPrefix = nameParts[0]
	}

	logr.FromContextOrDiscard(ctx).Info("Setting up NodeResolver", "customerPrefix", receiver.customerPrefix)
	return CustomerPrefixResolver{
		API:            receiver,
		CustomerPrefix: receiver.customerPrefix,
	}.Resolve(ctx, name)
}
