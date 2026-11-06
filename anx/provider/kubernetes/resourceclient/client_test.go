package resourceclient

import (
	"context"
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

var (
	testTarget = Target{
		Group:     "networking.example.io",
		Version:   "v1alpha1",
		Resource:  "loadbalancers",
		Namespace: "infrastructure",
	}
	testGVR = testTarget.GroupVersionResource()
)

func TestNewValidatesItsDependenciesAndTarget(t *testing.T) {
	tests := []struct {
		name    string
		client  dynamic.Interface
		target  Target
		wantErr string
	}{
		{
			name:    "nil dynamic client",
			target:  testTarget,
			wantErr: "dynamic Kubernetes client is nil",
		},
		{
			name:    "missing version",
			client:  newFakeClient(),
			target:  Target{Resource: "loadbalancers"},
			wantErr: "resource target requires version",
		},
		{
			name:    "missing resource",
			client:  newFakeClient(),
			target:  Target{Version: "v1alpha1"},
			wantErr: "resource target requires resource",
		},
		{
			name:   "core API resource may omit group",
			client: newFakeClient(),
			target: Target{Version: "v1", Resource: "configmaps"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.client, test.target)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("New() returned unexpected error: %v", err)
				}
				return
			}

			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("New() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestGetAndExists(t *testing.T) {
	dynamicClient := newFakeClient(newLoadBalancer("managed", "infrastructure", map[string]string{
		"networking.example.io/controller": "external",
	}))
	client, err := New(dynamicClient, testTarget)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}

	object, err := client.Get(context.Background(), "managed")
	if err != nil {
		t.Fatalf("Get() returned unexpected error: %v", err)
	}
	if got := object.GetAnnotations()["networking.example.io/controller"]; got != "external" {
		t.Fatalf("Get() annotation = %q, want %q", got, "external")
	}

	exists, err := client.Exists(context.Background(), "managed")
	if err != nil || !exists {
		t.Fatalf("Exists() = %v, %v, want true, nil", exists, err)
	}

	exists, err = client.Exists(context.Background(), "missing")
	if err != nil || exists {
		t.Fatalf("Exists() = %v, %v, want false, nil", exists, err)
	}
}

func TestExistsDoesNotHideAPIErrors(t *testing.T) {
	dynamicClient := newFakeClient()
	dynamicClient.PrependReactor("get", "loadbalancers", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Group: testGVR.Group, Resource: testGVR.Resource},
			"managed",
			errors.New("RBAC denied"),
		)
	})
	client, err := New(dynamicClient, testTarget)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}

	exists, err := client.Exists(context.Background(), "managed")
	if err == nil || exists {
		t.Fatalf("Exists() = %v, %v, want false and an error", exists, err)
	}
	if !apierrors.IsForbidden(err) {
		t.Fatalf("Exists() error = %v, want Kubernetes Forbidden error", err)
	}
}

func TestListAndAny(t *testing.T) {
	dynamicClient := newFakeClient(
		newLoadBalancer("internal", "infrastructure", map[string]string{
			"networking.example.io/controller": "ccm",
		}),
		newLoadBalancer("external", "infrastructure", map[string]string{
			"networking.example.io/controller": "external",
		}),
		newLoadBalancer("other-namespace", "other", map[string]string{
			"networking.example.io/controller": "external",
		}),
	)
	client, err := New(dynamicClient, testTarget)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}

	objects, err := client.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List() returned unexpected error: %v", err)
	}
	if got := len(objects.Items); got != 2 {
		t.Fatalf("List() returned %d objects, want 2", got)
	}

	exists, err := client.Any(
		context.Background(),
		metav1.ListOptions{},
		MatchAnnotations(map[string]string{"networking.example.io/controller": "external"}),
	)
	if err != nil || !exists {
		t.Fatalf("Any() = %v, %v, want true, nil", exists, err)
	}

	exists, err = client.Any(
		context.Background(),
		metav1.ListOptions{},
		MatchAnnotations(map[string]string{"networking.example.io/controller": "absent"}),
	)
	if err != nil || exists {
		t.Fatalf("Any() = %v, %v, want false, nil", exists, err)
	}
}

func TestMatchAnnotationsRequiresTheAnnotationToExist(t *testing.T) {
	matchEmptyValue := MatchAnnotations(map[string]string{"networking.example.io/controller": ""})

	if matchEmptyValue(newLoadBalancer("absent", "infrastructure", nil)) {
		t.Fatal("MatchAnnotations() matched an object without the required annotation")
	}
	if !matchEmptyValue(newLoadBalancer("present", "infrastructure", map[string]string{
		"networking.example.io/controller": "",
	})) {
		t.Fatal("MatchAnnotations() did not match a present annotation with the required empty value")
	}
}

func TestWatch(t *testing.T) {
	dynamicClient := newFakeClient()
	client, err := New(dynamicClient, testTarget)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}

	watcher, err := client.Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Watch() returned unexpected error: %v", err)
	}
	defer watcher.Stop()

	_, err = dynamicClient.Resource(testGVR).Namespace(testTarget.Namespace).Create(
		context.Background(),
		newLoadBalancer("managed", testTarget.Namespace, nil),
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("creating watched test object: %v", err)
	}

	event := <-watcher.ResultChan()
	if event.Type != watch.Added {
		t.Fatalf("Watch() event type = %q, want %q", event.Type, watch.Added)
	}
	object, ok := event.Object.(*unstructured.Unstructured)
	if !ok || object.GetName() != "managed" {
		t.Fatalf("Watch() event object = %#v, want unstructured object named managed", event.Object)
	}
}

func newFakeClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{testGVR: "LoadBalancerList"},
		objects...,
	)
}

func newLoadBalancer(name, namespace string, annotations map[string]string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "networking.example.io/v1alpha1",
		"kind":       "LoadBalancer",
		"metadata": map[string]interface{}{
			"name":        name,
			"namespace":   namespace,
			"annotations": stringMapToAnyMap(annotations),
		},
	}}
}

func stringMapToAnyMap(values map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
