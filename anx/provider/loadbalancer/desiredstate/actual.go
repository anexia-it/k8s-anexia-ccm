package desiredstate

import (
	"fmt"
	"math"
	"slices"
	"sort"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

// ActualInput contains LBaaS resources already retrieved from the Engine. The
// normalizer never retrieves or waits for resources itself and ignores their
// automation/status fields.
type ActualInput struct {
	Service     string
	ServiceUID  string
	ClusterName string

	Backends  []*lbaasv1.Backend
	Servers   []*lbaasv1.Server
	Frontends []*lbaasv1.Frontend
	Binds     []*lbaasv1.Bind

	// DefaultTags is useful when all objects were retrieved through the same
	// service UID tag. TagsByIdentifier overrides it for individual objects.
	DefaultTags      []string
	TagsByIdentifier map[string][]string
}

// ActualIssue describes malformed or ambiguous actual state. NormalizeActual
// still returns all resources it can normalize so debugging can continue.
type ActualIssue struct {
	Kind       Kind   `json:"kind"`
	Identifier string `json:"identifier,omitempty"`
	Message    string `json:"message"`
}

type actualReference struct {
	key          string
	loadBalancer string
}

// NormalizeActual converts already-fetched Engine objects into the same shape
// as Build. Engine identifiers are used only to resolve relationships; they do
// not become part of the comparable resource specifications.
func NormalizeActual(input ActualInput) (State, []ActualIssue) {
	state := State{
		Service:     input.Service,
		ServiceUID:  input.ServiceUID,
		ClusterName: input.ClusterName,
		Backends:    make([]Backend, 0, len(input.Backends)),
		Servers:     make([]Server, 0, len(input.Servers)),
		Frontends:   make([]Frontend, 0, len(input.Frontends)),
		Binds:       make([]Bind, 0, len(input.Binds)),
	}
	issues := make([]ActualIssue, 0)
	loadBalancers := make(map[string]struct{})
	addresses := make(map[string]struct{})
	backendReferences := make(map[string]actualReference, len(input.Backends))
	frontendReferences := make(map[string]actualReference, len(input.Frontends))

	for _, actual := range input.Backends {
		if actual == nil {
			issues = append(issues, ActualIssue{Kind: KindBackend, Message: "backend is nil"})
			continue
		}
		loadBalancer := actual.LoadBalancer.Identifier
		if loadBalancer == "" {
			loadBalancer = unresolvedSegment(KindBackend, actual.Identifier)
			issues = append(issues, ActualIssue{Kind: KindBackend, Identifier: actual.Identifier, Message: "load balancer identifier is missing"})
		} else {
			loadBalancers[loadBalancer] = struct{}{}
		}
		key := resourceKey(KindBackend, loadBalancer, actual.Name)
		state.Backends = append(state.Backends, Backend{
			Key:                    key,
			Name:                   actual.Name,
			LoadBalancerIdentifier: loadBalancer,
			Mode:                   string(actual.Mode),
			HealthCheck:            actual.HealthCheck,
			Tags:                   actualTags(input, actual.Identifier),
		})
		if actual.Identifier == "" {
			issues = append(issues, ActualIssue{Kind: KindBackend, Message: fmt.Sprintf("backend %q has no identifier", actual.Name)})
		} else {
			backendReferences[actual.Identifier] = actualReference{key: key, loadBalancer: loadBalancer}
		}
	}

	for _, actual := range input.Frontends {
		if actual == nil {
			issues = append(issues, ActualIssue{Kind: KindFrontend, Message: "frontend is nil"})
			continue
		}
		loadBalancer := ""
		if actual.LoadBalancer != nil {
			loadBalancer = actual.LoadBalancer.Identifier
		}
		if loadBalancer == "" {
			loadBalancer = unresolvedSegment(KindFrontend, actual.Identifier)
			issues = append(issues, ActualIssue{Kind: KindFrontend, Identifier: actual.Identifier, Message: "load balancer identifier is missing"})
		} else {
			loadBalancers[loadBalancer] = struct{}{}
		}

		backendIdentifier := ""
		if actual.DefaultBackend != nil {
			backendIdentifier = actual.DefaultBackend.Identifier
		}
		backendReference, ok := backendReferences[backendIdentifier]
		if !ok {
			backendReference.key = unresolvedReference(KindBackend, backendIdentifier)
			issues = append(issues, ActualIssue{Kind: KindFrontend, Identifier: actual.Identifier, Message: fmt.Sprintf("default backend %q was not found", backendIdentifier)})
		}

		key := resourceKey(KindFrontend, loadBalancer, actual.Name)
		state.Frontends = append(state.Frontends, Frontend{
			Key:                    key,
			Name:                   actual.Name,
			LoadBalancerIdentifier: loadBalancer,
			DefaultBackendKey:      backendReference.key,
			Mode:                   string(actual.Mode),
			Tags:                   actualTags(input, actual.Identifier),
		})
		if actual.Identifier == "" {
			issues = append(issues, ActualIssue{Kind: KindFrontend, Message: fmt.Sprintf("frontend %q has no identifier", actual.Name)})
		} else {
			frontendReferences[actual.Identifier] = actualReference{key: key, loadBalancer: loadBalancer}
		}
	}

	for _, actual := range input.Servers {
		if actual == nil {
			issues = append(issues, ActualIssue{Kind: KindServer, Message: "server is nil"})
			continue
		}
		backendIdentifier := actual.Backend.Identifier
		backendReference, ok := backendReferences[backendIdentifier]
		if !ok {
			backendReference = actualReference{
				key:          unresolvedReference(KindBackend, backendIdentifier),
				loadBalancer: unresolvedSegment(KindBackend, backendIdentifier),
			}
			issues = append(issues, ActualIssue{Kind: KindServer, Identifier: actual.Identifier, Message: fmt.Sprintf("backend %q was not found", backendIdentifier)})
		}
		port, portIssue := actualPort(KindServer, actual.Identifier, actual.Port)
		if portIssue != nil {
			issues = append(issues, *portIssue)
		}
		state.Servers = append(state.Servers, Server{
			Key:        resourceKey(KindServer, backendReference.loadBalancer, actual.Name),
			Name:       actual.Name,
			BackendKey: backendReference.key,
			IP:         actual.IP,
			Port:       port,
			Check:      actual.Check,
			Tags:       actualTags(input, actual.Identifier),
		})
	}

	for _, actual := range input.Binds {
		if actual == nil {
			issues = append(issues, ActualIssue{Kind: KindBind, Message: "bind is nil"})
			continue
		}
		frontendIdentifier := actual.Frontend.Identifier
		frontendReference, ok := frontendReferences[frontendIdentifier]
		if !ok {
			frontendReference = actualReference{
				key:          unresolvedReference(KindFrontend, frontendIdentifier),
				loadBalancer: unresolvedSegment(KindFrontend, frontendIdentifier),
			}
			issues = append(issues, ActualIssue{Kind: KindBind, Identifier: actual.Identifier, Message: fmt.Sprintf("frontend %q was not found", frontendIdentifier)})
		}
		port, portIssue := actualPort(KindBind, actual.Identifier, actual.Port)
		if portIssue != nil {
			issues = append(issues, *portIssue)
		}
		state.Binds = append(state.Binds, Bind{
			Key:         resourceKey(KindBind, frontendReference.loadBalancer, actual.Name),
			Name:        actual.Name,
			FrontendKey: frontendReference.key,
			Address:     actual.Address,
			Port:        port,
			Tags:        actualTags(input, actual.Identifier),
		})
		if actual.Address != "" {
			addresses[actual.Address] = struct{}{}
		}
	}

	state.LoadBalancerIdentifiers = sortedSet(loadBalancers)
	state.ExternalAddresses = sortedSet(addresses)
	sort.Slice(state.Backends, func(i, j int) bool { return state.Backends[i].Key < state.Backends[j].Key })
	sort.Slice(state.Servers, func(i, j int) bool { return state.Servers[i].Key < state.Servers[j].Key })
	sort.Slice(state.Frontends, func(i, j int) bool { return state.Frontends[i].Key < state.Frontends[j].Key })
	sort.Slice(state.Binds, func(i, j int) bool { return state.Binds[i].Key < state.Binds[j].Key })
	issues = append(issues, duplicateKeyIssues(state.Resources())...)
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Kind != issues[j].Kind {
			return issues[i].Kind < issues[j].Kind
		}
		if issues[i].Identifier != issues[j].Identifier {
			return issues[i].Identifier < issues[j].Identifier
		}
		return issues[i].Message < issues[j].Message
	})
	return state, issues
}

func actualTags(input ActualInput, identifier string) []string {
	tags := input.DefaultTags
	if resourceTags, ok := input.TagsByIdentifier[identifier]; ok {
		tags = resourceTags
	}
	ret := slices.Clone(tags)
	sort.Strings(ret)
	return ret
}

func actualPort(kind Kind, identifier string, value int) (uint16, *ActualIssue) {
	if value < 0 || value > math.MaxUint16 {
		return 0, &ActualIssue{Kind: kind, Identifier: identifier, Message: fmt.Sprintf("port %d is outside the uint16 range", value)}
	}
	return uint16(value), nil
}

func unresolvedSegment(kind Kind, identifier string) string {
	if identifier == "" {
		identifier = "missing"
	}
	return fmt.Sprintf("unresolved-%s-%s", kind, identifier)
}

func unresolvedReference(kind Kind, identifier string) string {
	return resourceKey(kind, unresolvedSegment(kind, identifier), "unresolved")
}

func sortedSet(values map[string]struct{}) []string {
	ret := make([]string, 0, len(values))
	for value := range values {
		ret = append(ret, value)
	}
	sort.Strings(ret)
	return ret
}

func duplicateKeyIssues(resources []PlannedResource) []ActualIssue {
	seen := make(map[string]struct{}, len(resources))
	issues := make([]ActualIssue, 0)
	for _, resource := range resources {
		if _, ok := seen[resource.ResourceKey()]; ok {
			issues = append(issues, ActualIssue{Kind: resource.ResourceKind(), Message: fmt.Sprintf("duplicate resource key %q", resource.ResourceKey())})
		}
		seen[resource.ResourceKey()] = struct{}{}
	}
	return issues
}
