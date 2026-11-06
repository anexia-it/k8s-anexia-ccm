package configuration

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewProviderConfigPrecedence(t *testing.T) {
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if strings.HasPrefix(name, "ANEXIA_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	savedOptions := managerOptions
	managerOptions = nil
	t.Cleanup(func() { managerOptions = savedOptions })

	tests := []struct {
		name        string
		yaml        string
		environment map[string]string
		wantPrefix  string
		wantSteps   int
	}{
		{name: "defaults without yaml", wantPrefix: "anxkube-ccm-lb", wantSteps: 30},
		{name: "defaults for omitted yaml fields", yaml: "clusterName: test\n", wantPrefix: "anxkube-ccm-lb", wantSteps: 30},
		{name: "yaml overrides defaults", yaml: "autoDiscoveryTagPrefix: mine\nloadBalancerBackoffSteps: 5\n", wantPrefix: "mine", wantSteps: 5},
		{name: "explicit yaml zero values override defaults", yaml: "autoDiscoveryTagPrefix: \"\"\nloadBalancerBackoffSteps: 0\n", wantPrefix: "", wantSteps: 0},
		{
			name: "environment overrides yaml",
			yaml: "autoDiscoveryTagPrefix: mine\nloadBalancerBackoffSteps: 5\n",
			environment: map[string]string{
				"ANEXIA_AUTO_DISCOVERY_TAG_PREFIX": "environment",
				"ANEXIA_LOADBALANCERBACKOFFSTEPS":  "7",
			},
			wantPrefix: "environment", wantSteps: 7,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for name, value := range test.environment {
				t.Setenv(name, value)
			}
			var reader io.Reader
			if test.yaml != "" {
				reader = strings.NewReader(test.yaml)
			}
			got, err := NewProviderConfig(reader)
			if err != nil {
				t.Fatalf("NewProviderConfig() error = %v", err)
			}
			if got.AutoDiscoveryTagPrefix != test.wantPrefix {
				t.Errorf("AutoDiscoveryTagPrefix = %q, want %q", got.AutoDiscoveryTagPrefix, test.wantPrefix)
			}
			if got.LoadBalancerBackoffSteps != test.wantSteps {
				t.Errorf("LoadBalancerBackoffSteps = %d, want %d", got.LoadBalancerBackoffSteps, test.wantSteps)
			}
		})
	}
}
