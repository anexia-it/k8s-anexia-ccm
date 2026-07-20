package desiredstate

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestBuild(t *testing.T) {
	t.Parallel()

	service := testService()
	nodes := []*corev1.Node{
		testNode("node-b", corev1.NodeInternalIP, "10.0.0.12"),
		{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.11"},
				{Type: corev1.NodeExternalIP, Address: "192.0.2.11"},
			}},
		},
	}

	state, err := Build(Input{
		Service:                 service,
		Nodes:                   nodes,
		ClusterName:             "cluster-a",
		LoadBalancerIdentifiers: []string{"lb-b", "lb-a", "lb-a"},
		ExternalAddresses:       []string{"2001:db8::10", "192.0.2.10"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := state.Service, "default/web"; got != want {
		t.Fatalf("State.Service = %q, want %q", got, want)
	}
	if got, want := state.LoadBalancerIdentifiers, []string{"lb-a", "lb-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("State.LoadBalancerIdentifiers = %v, want %v", got, want)
	}
	if got, want := state.ExternalAddresses, []string{"192.0.2.10", "2001:db8::10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("State.ExternalAddresses = %v, want %v", got, want)
	}

	assertLength(t, "Backends", len(state.Backends), 4)
	assertLength(t, "Servers", len(state.Servers), 8)
	assertLength(t, "Frontends", len(state.Frontends), 4)
	assertLength(t, "Binds", len(state.Binds), 8)
	assertLength(t, "Resources", len(state.Resources()), 24)

	backend := state.Backends[0]
	if got, want := backend.Key, "backend/lb-a/http.web.default.cluster-a"; got != want {
		t.Errorf("Backend.Key = %q, want %q", got, want)
	}
	if got, want := backend.Tags, []string{"anxccm-svc-uid=service-uid"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Backend.Tags = %v, want %v", got, want)
	}

	server := state.Servers[0]
	if got, want := server.Name, "node-a.http.web.default.cluster-a"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
	if got, want := server.IP, "192.0.2.11"; got != want {
		t.Errorf("Server.IP = %q, want external address %q", got, want)
	}
	if got, want := server.Dependencies(), []string{backend.Key}; !reflect.DeepEqual(got, want) {
		t.Errorf("Server.Dependencies() = %v, want %v", got, want)
	}

	frontend := state.Frontends[0]
	if got, want := frontend.Dependencies(), []string{backend.Key}; !reflect.DeepEqual(got, want) {
		t.Errorf("Frontend.Dependencies() = %v, want %v", got, want)
	}

	bind := state.Binds[0]
	if got, want := bind.Name, "v4.http.web.default.cluster-a"; got != want {
		t.Errorf("Bind.Name = %q, want %q", got, want)
	}
	if got, want := bind.Address, "192.0.2.10"; got != want {
		t.Errorf("Bind.Address = %q, want %q", got, want)
	}
	if got, want := bind.Dependencies(), []string{frontend.Key}; !reflect.DeepEqual(got, want) {
		t.Errorf("Bind.Dependencies() = %v, want %v", got, want)
	}

	stages := state.ExecutionStages()
	if got, want := len(stages), 3; got != want {
		t.Fatalf("len(ExecutionStages()) = %d, want %d", got, want)
	}
	assertLength(t, "stage 0", len(stages[0]), 4)
	assertLength(t, "stage 1", len(stages[1]), 12)
	assertLength(t, "stage 2", len(stages[2]), 8)

	secondState, err := Build(Input{
		Service:                 service,
		Nodes:                   nodes,
		ClusterName:             "cluster-a",
		LoadBalancerIdentifiers: []string{"lb-b", "lb-a"},
		ExternalAddresses:       []string{"2001:db8::10", "192.0.2.10"},
	})
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	firstJSON, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal(first state) error = %v", err)
	}
	secondJSON, err := json.Marshal(secondState)
	if err != nil {
		t.Fatalf("json.Marshal(second state) error = %v", err)
	}
	if !reflect.DeepEqual(firstJSON, secondJSON) {
		t.Errorf("Build() is not deterministic\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

func TestBuildValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Input)
		want   error
	}{
		{
			name: "service required",
			mutate: func(input *Input) {
				input.Service = nil
			},
			want: ErrServiceRequired,
		},
		{
			name: "load balancer service required",
			mutate: func(input *Input) {
				input.Service.Spec.Type = corev1.ServiceTypeClusterIP
			},
			want: ErrNotLoadBalancerService,
		},
		{
			name: "service UID required",
			mutate: func(input *Input) {
				input.Service.UID = ""
			},
			want: ErrServiceUIDRequired,
		},
		{
			name: "load balancer required",
			mutate: func(input *Input) {
				input.LoadBalancerIdentifiers = nil
			},
			want: ErrNoLoadBalancers,
		},
		{
			name: "address required",
			mutate: func(input *Input) {
				input.ExternalAddresses = nil
			},
			want: ErrNoExternalAddresses,
		},
		{
			name: "one address per family",
			mutate: func(input *Input) {
				input.ExternalAddresses = []string{"192.0.2.10", "192.0.2.11"}
			},
			want: ErrAddressFamilyNotUnique,
		},
		{
			name: "TCP only",
			mutate: func(input *Input) {
				input.Service.Spec.Ports[0].Protocol = corev1.ProtocolUDP
			},
			want: ErrUnsupportedProtocol,
		},
		{
			name: "NodePort required",
			mutate: func(input *Input) {
				input.Service.Spec.Ports[0].NodePort = 0
			},
			want: ErrNodePortRequired,
		},
		{
			name: "usable node address required",
			mutate: func(input *Input) {
				input.Nodes[0].Status.Addresses = nil
			},
			want: ErrNoUsableNodeAddress,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := validInput()
			test.mutate(&input)
			_, err := Build(input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Build() error = %v, want errors.Is(_, %v)", err, test.want)
			}
		})
	}
}

func testService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
			UID:       types.UID("service-uid"),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "https", Protocol: corev1.ProtocolTCP, Port: 443, NodePort: 30443},
				{Name: "http", Protocol: corev1.ProtocolTCP, Port: 80, NodePort: 30080},
			},
		},
	}
}

func testNode(name string, addressType corev1.NodeAddressType, address string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{
			Type:    addressType,
			Address: address,
		}}},
	}
}

func validInput() Input {
	return Input{
		Service:                 testService(),
		Nodes:                   []*corev1.Node{testNode("node-a", corev1.NodeInternalIP, "10.0.0.11")},
		ClusterName:             "cluster-a",
		LoadBalancerIdentifiers: []string{"lb-a"},
		ExternalAddresses:       []string{"192.0.2.10"},
	}
}

func assertLength(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("len(%s) = %d, want %d", name, got, want)
	}
}
