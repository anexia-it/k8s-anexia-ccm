package desiredstate

import (
	"testing"

	lbaasv1 "go.anx.io/go-anxcloud/pkg/apis/lbaas/v1"
)

func TestNormalizeActual(t *testing.T) {
	t.Parallel()

	input := validInput()
	input.Service.Spec.Ports = input.Service.Spec.Ports[:1]
	desired, err := Build(input)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	backend := &lbaasv1.Backend{
		Identifier:   "backend-id",
		Name:         desired.Backends[0].Name,
		Mode:         lbaasv1.TCP,
		HealthCheck:  TCPHealthCheck,
		LoadBalancer: lbaasv1.LoadBalancer{Identifier: "lb-a"},
	}
	frontend := &lbaasv1.Frontend{
		Identifier:     "frontend-id",
		Name:           desired.Frontends[0].Name,
		Mode:           lbaasv1.TCP,
		LoadBalancer:   &lbaasv1.LoadBalancer{Identifier: "lb-a"},
		DefaultBackend: &lbaasv1.Backend{Identifier: backend.Identifier},
	}
	server := &lbaasv1.Server{
		Identifier: "server-id",
		Name:       desired.Servers[0].Name,
		IP:         desired.Servers[0].IP,
		Port:       int(desired.Servers[0].Port),
		Check:      ServerCheckEnabled,
		Backend:    lbaasv1.Backend{Identifier: backend.Identifier},
	}
	bind := &lbaasv1.Bind{
		Identifier: "bind-id",
		Name:       desired.Binds[0].Name,
		Address:    desired.Binds[0].Address,
		Port:       int(desired.Binds[0].Port),
		Frontend:   lbaasv1.Frontend{Identifier: frontend.Identifier},
	}

	actual, issues := NormalizeActual(ActualInput{
		Service:     desired.Service,
		ServiceUID:  desired.ServiceUID,
		ClusterName: desired.ClusterName,
		Backends:    []*lbaasv1.Backend{backend},
		Servers:     []*lbaasv1.Server{server},
		Frontends:   []*lbaasv1.Frontend{frontend},
		Binds:       []*lbaasv1.Bind{bind},
		DefaultTags: []string{ServiceUIDTagKeyPrefix + desired.ServiceUID},
	})
	if len(issues) != 0 {
		t.Fatalf("NormalizeActual() issues = %#v, want none", issues)
	}
	if differences := Compare(desired, actual); len(differences) != 0 {
		t.Errorf("Compare() = %#v, want no differences", differences)
	}

	bind.Port++
	actual, issues = NormalizeActual(ActualInput{
		Backends:    []*lbaasv1.Backend{backend},
		Servers:     []*lbaasv1.Server{server},
		Frontends:   []*lbaasv1.Frontend{frontend},
		Binds:       []*lbaasv1.Bind{bind},
		DefaultTags: []string{ServiceUIDTagKeyPrefix + desired.ServiceUID},
	})
	if len(issues) != 0 {
		t.Fatalf("NormalizeActual() issues after change = %#v, want none", issues)
	}
	if differences := Compare(desired, actual); len(differences) != 1 || differences[0].Type != DifferenceChanged {
		t.Errorf("Compare() after bind change = %#v, want one changed difference", differences)
	}
}

func TestNormalizeActualReportsBrokenRelationships(t *testing.T) {
	t.Parallel()

	actual, issues := NormalizeActual(ActualInput{
		Servers: []*lbaasv1.Server{{
			Identifier: "server-id",
			Name:       "orphaned-server",
			Backend:    lbaasv1.Backend{Identifier: "missing-backend"},
		}},
	})
	if len(actual.Servers) != 1 {
		t.Fatalf("len(actual.Servers) = %d, want 1", len(actual.Servers))
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d, want 1: %#v", len(issues), issues)
	}
}
