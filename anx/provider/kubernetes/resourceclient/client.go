package resourceclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Target identifies the Kubernetes resource queried by a Client.
//
// Resource is the plural REST resource name from the CRD, for example
// "loadbalancers". Namespace may be left empty for a cluster-scoped resource
// or to list/watch a namespaced resource across all namespaces. Get and Exists
// on a namespaced resource require a concrete namespace.
type Target struct {
	Group     string `json:"group" yaml:"group"`
	Version   string `json:"version" yaml:"version"`
	Resource  string `json:"resource" yaml:"resource"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

// GroupVersionResource returns the client-go resource identifier for t.
func (t Target) GroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    t.Group,
		Version:  t.Version,
		Resource: t.Resource,
	}
}

func (t Target) validate() error {
	var missing []string
	if strings.TrimSpace(t.Version) == "" {
		missing = append(missing, "version")
	}
	if strings.TrimSpace(t.Resource) == "" {
		missing = append(missing, "resource")
	}
	if len(missing) != 0 {
		return fmt.Errorf("resource target requires %s", strings.Join(missing, " and "))
	}

	return nil
}

// Matcher decides whether an unstructured object is relevant to the caller.
// It can inspect annotations or any other field without introducing a Go type
// for the resource.
type Matcher func(*unstructured.Unstructured) bool

// MatchAnnotations returns a Matcher that requires every supplied annotation
// to exist with the supplied value. An empty map matches every object.
func MatchAnnotations(required map[string]string) Matcher {
	return func(object *unstructured.Unstructured) bool {
		annotations := object.GetAnnotations()
		for key, value := range required {
			actual, exists := annotations[key]
			if !exists || actual != value {
				return false
			}
		}

		return true
	}
}

// Client is a read-only client bound to one Kubernetes resource target.
type Client struct {
	target   Target
	resource dynamic.ResourceInterface
}

// NewForConfig creates a read-only resource client from a Kubernetes REST
// config. Use New with an existing dynamic client when sharing transports or
// when testing.
func NewForConfig(config *rest.Config, target Target) (*Client, error) {
	if config == nil {
		return nil, errors.New("kubernetes REST config is nil")
	}
	if err := target.validate(); err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create dynamic Kubernetes client: %w", err)
	}

	return New(dynamicClient, target)
}

// New creates a read-only resource client using an existing dynamic client.
func New(dynamicClient dynamic.Interface, target Target) (*Client, error) {
	if dynamicClient == nil {
		return nil, errors.New("dynamic Kubernetes client is nil")
	}
	if err := target.validate(); err != nil {
		return nil, err
	}

	resource := dynamicClient.Resource(target.GroupVersionResource())
	if target.Namespace != "" {
		return &Client{target: target, resource: resource.Namespace(target.Namespace)}, nil
	}

	return &Client{target: target, resource: resource}, nil
}

// Target returns the resource target the client is bound to.
func (c *Client) Target() Target {
	return c.target
}

// Get retrieves one object by name.
func (c *Client) Get(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	object, err := c.resource.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s %q: %w", c.target.GroupVersionResource(), name, err)
	}

	return object, nil
}

// Exists reports whether an object with name exists. Errors other than a
// Kubernetes NotFound response are returned to the caller; in particular,
// missing RBAC permissions and transport failures are not hidden.
func (c *Client) Exists(ctx context.Context, name string) (bool, error) {
	_, err := c.Get(ctx, name)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// List retrieves objects matching Kubernetes list options. Label and field
// selectors should be expressed through opts; annotation filtering can be done
// with Any and a Matcher.
func (c *Client) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	objects, err := c.resource.List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", c.target.GroupVersionResource(), err)
	}

	return objects, nil
}

// Any reports whether at least one listed object satisfies match. A nil
// Matcher accepts every object and limits the API request to one result. When
// the API paginates a filtered request, Any follows continuation tokens until
// it finds a match or exhausts the result set.
func (c *Client) Any(ctx context.Context, opts metav1.ListOptions, match Matcher) (bool, error) {
	if match == nil && opts.Limit == 0 {
		opts.Limit = 1
	}

	for {
		objects, err := c.List(ctx, opts)
		if err != nil {
			return false, err
		}

		for i := range objects.Items {
			if match == nil || match(&objects.Items[i]) {
				return true, nil
			}
		}

		if objects.GetContinue() == "" {
			return false, nil
		}
		opts.Continue = objects.GetContinue()
	}
}

// Watch watches objects matching Kubernetes list options. The caller owns the
// returned watch and must call Stop when it is no longer needed.
func (c *Client) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	watcher, err := c.resource.Watch(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("watch %s: %w", c.target.GroupVersionResource(), err)
	}

	return watcher, nil
}
