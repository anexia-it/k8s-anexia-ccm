package desiredstate

import corev1 "k8s.io/api/core/v1"

// Kind identifies an LBaaS resource type in a desired state.
type Kind string

const (
	KindBackend  Kind = "backend"
	KindServer   Kind = "server"
	KindFrontend Kind = "frontend"
	KindBind     Kind = "bind"
)

const (
	ModeTCP                = "tcp"
	TCPHealthCheck         = `"adv_check": "tcp-check"`
	ServerCheckEnabled     = "enabled"
	ServiceUIDTagKeyPrefix = "anxccm-svc-uid="
)

// Input contains Kubernetes state and the infrastructure decisions which
// cannot be derived from it without consulting Anexia APIs.
type Input struct {
	Service *corev1.Service
	Nodes   []*corev1.Node

	ClusterName             string
	LoadBalancerIdentifiers []string
	ExternalAddresses       []string
}

// State is the complete desired LBaaS resource graph. All slices and resource
// dependencies have deterministic ordering.
type State struct {
	Service                 string     `json:"service"`
	ServiceUID              string     `json:"serviceUID"`
	ClusterName             string     `json:"clusterName,omitempty"`
	LoadBalancerIdentifiers []string   `json:"loadBalancerIdentifiers"`
	ExternalAddresses       []string   `json:"externalAddresses"`
	Backends                []Backend  `json:"backends"`
	Servers                 []Server   `json:"servers"`
	Frontends               []Frontend `json:"frontends"`
	Binds                   []Bind     `json:"binds"`
}

// PlannedResource is implemented by every resource in State.
type PlannedResource interface {
	ResourceKey() string
	ResourceKind() Kind
	Dependencies() []string
}

// Backend is the desired Engine backend for one Service port.
type Backend struct {
	Key                    string   `json:"key"`
	Name                   string   `json:"name"`
	LoadBalancerIdentifier string   `json:"loadBalancerIdentifier"`
	Mode                   string   `json:"mode"`
	HealthCheck            string   `json:"healthCheck"`
	Tags                   []string `json:"tags"`
}

func (r Backend) ResourceKey() string    { return r.Key }
func (r Backend) ResourceKind() Kind     { return KindBackend }
func (r Backend) Dependencies() []string { return nil }

// Server is the desired Engine backend server for one Node and Service port.
type Server struct {
	Key        string   `json:"key"`
	Name       string   `json:"name"`
	BackendKey string   `json:"backendKey"`
	IP         string   `json:"ip"`
	Port       uint16   `json:"port"`
	Check      string   `json:"check"`
	Tags       []string `json:"tags"`
}

func (r Server) ResourceKey() string    { return r.Key }
func (r Server) ResourceKind() Kind     { return KindServer }
func (r Server) Dependencies() []string { return []string{r.BackendKey} }

// Frontend is the desired Engine frontend for one Service port.
type Frontend struct {
	Key                    string   `json:"key"`
	Name                   string   `json:"name"`
	LoadBalancerIdentifier string   `json:"loadBalancerIdentifier"`
	DefaultBackendKey      string   `json:"defaultBackendKey"`
	Mode                   string   `json:"mode"`
	Tags                   []string `json:"tags"`
}

func (r Frontend) ResourceKey() string    { return r.Key }
func (r Frontend) ResourceKind() Kind     { return KindFrontend }
func (r Frontend) Dependencies() []string { return []string{r.DefaultBackendKey} }

// Bind is the desired Engine frontend bind for one external address and
// Service port.
type Bind struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	FrontendKey string   `json:"frontendKey"`
	Address     string   `json:"address"`
	Port        uint16   `json:"port"`
	Tags        []string `json:"tags"`
}

func (r Bind) ResourceKey() string    { return r.Key }
func (r Bind) ResourceKind() Kind     { return KindBind }
func (r Bind) Dependencies() []string { return []string{r.FrontendKey} }

// Resources returns one deterministic flat list. Its order is suitable for
// display and comparison, but an executor should use ExecutionStages.
func (s State) Resources() []PlannedResource {
	resources := make([]PlannedResource, 0, len(s.Backends)+len(s.Servers)+len(s.Frontends)+len(s.Binds))
	for _, resource := range s.Backends {
		resources = append(resources, resource)
	}
	for _, resource := range s.Servers {
		resources = append(resources, resource)
	}
	for _, resource := range s.Frontends {
		resources = append(resources, resource)
	}
	for _, resource := range s.Binds {
		resources = append(resources, resource)
	}
	return resources
}

// ExecutionStages groups resources by dependency. Resources within a stage
// may be created concurrently. A later stage may refer to identifiers resolved
// while creating an earlier stage.
func (s State) ExecutionStages() [][]PlannedResource {
	backends := make([]PlannedResource, 0, len(s.Backends))
	backendDependants := make([]PlannedResource, 0, len(s.Servers)+len(s.Frontends))
	binds := make([]PlannedResource, 0, len(s.Binds))

	for _, resource := range s.Backends {
		backends = append(backends, resource)
	}
	for _, resource := range s.Servers {
		backendDependants = append(backendDependants, resource)
	}
	for _, resource := range s.Frontends {
		backendDependants = append(backendDependants, resource)
	}
	for _, resource := range s.Binds {
		binds = append(binds, resource)
	}

	return [][]PlannedResource{backends, backendDependants, binds}
}
