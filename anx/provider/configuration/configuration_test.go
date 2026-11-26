package configuration

import (
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cloud-provider/options"
)

func TestConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Provider configuration")
}

type providerConfigWant struct {
	prefix       string
	backoffSteps int
	clusterName  string
}

type newProviderConfigTestCase struct {
	yamlConfig  string
	environment map[string]string
	clusterName string
	want        providerConfigWant
}

var _ = Describe("NewProviderConfig", func() {
	var savedManagerOptions *options.CloudControllerManagerOptions

	BeforeEach(func() {
		for _, variable := range os.Environ() {
			name, _, _ := strings.Cut(variable, "=")
			if strings.HasPrefix(name, "ANEXIA_") {
				DeferCleanup(os.Setenv, name, os.Getenv(name))
				_ = os.Unsetenv(name)
			}
		}

		savedManagerOptions = managerOptions
		managerOptions = nil
		DeferCleanup(func() { managerOptions = savedManagerOptions })
	})

	DescribeTable("precedence between defaults, YAML, environment and CLI flags",
		func(testCase newProviderConfigTestCase) {
			if testCase.clusterName != "" {
				mo, err := options.NewCloudControllerManagerOptions()
				Expect(err).ToNot(HaveOccurred())
				mo.KubeCloudShared.ClusterName = testCase.clusterName
				managerOptions = mo
			}

			for name, value := range testCase.environment {
				_ = os.Setenv(name, value)
				DeferCleanup(os.Unsetenv, name)
			}

			var reader io.Reader
			if testCase.yamlConfig != "" {
				reader = strings.NewReader(testCase.yamlConfig)
			}

			got, err := NewProviderConfig(reader)
			Expect(err).ToNot(HaveOccurred())
			Expect(got.AutoDiscoveryTagPrefix).To(Equal(testCase.want.prefix))
			Expect(got.LoadBalancerBackoffSteps).To(Equal(testCase.want.backoffSteps))
			Expect(got.ClusterName).To(Equal(testCase.want.clusterName))
		},

		Entry("uses defaults when no YAML or environment is given", newProviderConfigTestCase{
			want: providerConfigWant{prefix: "anxkube-ccm-lb", backoffSteps: 30},
		}),

		Entry("keeps defaults for YAML fields that are omitted", newProviderConfigTestCase{
			yamlConfig: `
clusterName: test
`,
			want: providerConfigWant{prefix: "anxkube-ccm-lb", backoffSteps: 30, clusterName: "test"},
		}),

		Entry("lets YAML override the defaults", newProviderConfigTestCase{
			yamlConfig: `
autoDiscoveryTagPrefix: mine
loadBalancerBackoffSteps: 5
`,
			want: providerConfigWant{prefix: "mine", backoffSteps: 5},
		}),

		Entry("lets explicit YAML zero values override the defaults", newProviderConfigTestCase{
			yamlConfig: `
autoDiscoveryTagPrefix: ""
loadBalancerBackoffSteps: 0
`,
			want: providerConfigWant{backoffSteps: 0},
		}),

		Entry("lets the environment override YAML", newProviderConfigTestCase{
			yamlConfig: `
autoDiscoveryTagPrefix: mine
loadBalancerBackoffSteps: 5
`,
			environment: map[string]string{
				"ANEXIA_AUTO_DISCOVERY_TAG_PREFIX": "environment",
				"ANEXIA_LOADBALANCERBACKOFFSTEPS":  "7",
			},
			want: providerConfigWant{prefix: "environment", backoffSteps: 7},
		}),

		Entry("ignores unknown YAML keys without error", newProviderConfigTestCase{
			yamlConfig: `
unknownKey: value
loadBalancerBackoffSteps: 9
`,
			want: providerConfigWant{prefix: "anxkube-ccm-lb", backoffSteps: 9},
		}),

		Entry("lets the --cluster-name flag override YAML clusterName", newProviderConfigTestCase{
			yamlConfig: `
clusterName: yaml-cluster
`,
			clusterName: "flag-cluster",
			want:        providerConfigWant{prefix: "anxkube-ccm-lb", backoffSteps: 30, clusterName: "flag-cluster"},
		}),
	)
})
