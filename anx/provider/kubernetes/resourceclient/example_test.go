package resourceclient_test

import (
	"context"
	"fmt"

	"github.com/anexia-it/k8s-anexia-ccm/anx/provider/kubernetes/resourceclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func ExampleClient_Any() {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return
	}

	loadBalancers, err := resourceclient.NewForConfig(restConfig, resourceclient.Target{
		Group:    "networking.example.io",
		Version:  "v1alpha1",
		Resource: "loadbalancers", // the plural name from the CRD
		// Leave Namespace empty to query all namespaces or a cluster-scoped CRD.
	})
	if err != nil {
		return
	}

	exists, err := loadBalancers.Any(
		context.Background(),
		metav1.ListOptions{},
		resourceclient.MatchAnnotations(map[string]string{
			"networking.example.io/controller": "external",
		}),
	)
	if err != nil {
		return
	}

	fmt.Printf("an external controller owns load balancers: %t\n", exists)
}
