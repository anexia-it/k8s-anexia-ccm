package loadbalancer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LBaaS operator")
}
