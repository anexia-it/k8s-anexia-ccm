// Package resourceclient provides a small, read-only client for Kubernetes
// resources that do not have a compile-time Go type.
//
// Objects are returned as unstructured.Unstructured values. This makes the
// package suitable for optional CRDs: callers only configure the resource's
// group, version and plural resource name and do not need generated clients or
// API packages for that CRD.
package resourceclient
