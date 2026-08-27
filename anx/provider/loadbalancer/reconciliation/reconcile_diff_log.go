package reconciliation

import (
	"fmt"
	"reflect"

	"go.anx.io/go-anxcloud/pkg/api/types"
	"go.anx.io/go-anxcloud/pkg/utils/object/compare"
)

// logReconcileDiff logs, in a human readable way, why resources of the given resourceType are about to be
// created or destroyed. It matches target and existing resources by their Name attribute and then uses
// compare.Compare to find out which of the compareAttributes actually differ, so operators can see the
// concrete reason a resource is being recreated instead of just its identifier.
//
// target and existing must be slices of the same pointer-to-struct type implementing types.Object, e.g.
// []*lbaasv1.Backend. This is only used for logging, it does not influence create/destroy decisions.
func (r *reconciliation) logReconcileDiff(resourceType string, target, existing interface{}, compareAttributes ...string) {
	targetVal := reflect.ValueOf(target)
	existingVal := reflect.ValueOf(existing)

	for i := 0; i < targetVal.Len(); i++ {
		t := targetVal.Index(i).Interface()

		idx, err := compare.Search(t, existing, "Name")
		if err != nil {
			r.logger.V(1).Info("could not compute human readable diff for resource", "resource-type", resourceType, "error", err.Error())
			continue
		}

		if idx == -1 {
			r.logger.Info("planning to create resource: no existing resource with this name found",
				"resource-type", resourceType,
				"resource", mustStringifyObject(t.(types.Object)),
			)
			continue
		}

		e := existingVal.Index(idx).Interface()

		diffs, err := compare.Compare(t, e, compareAttributes...)
		if err != nil {
			r.logger.V(1).Info("could not compute human readable diff for resource", "resource-type", resourceType, "error", err.Error())
			continue
		}

		if len(diffs) > 0 {
			changes := make([]string, 0, len(diffs))
			for _, d := range diffs {
				changes = append(changes, fmt.Sprintf("%s: %v -> %v", d.Key, d.B, d.A))
			}

			r.logger.Info("planning to recreate resource: existing resource differs from desired state",
				"resource-type", resourceType,
				"resource", mustStringifyObject(e.(types.Object)),
				"differences", changes,
			)
		}
	}

	for i := 0; i < existingVal.Len(); i++ {
		e := existingVal.Index(i).Interface()

		idx, err := compare.Search(e, target, "Name")
		if err != nil {
			r.logger.V(1).Info("could not compute human readable diff for resource", "resource-type", resourceType, "error", err.Error())
			continue
		}

		if idx == -1 {
			r.logger.Info("planning to destroy resource: no longer part of desired state",
				"resource-type", resourceType,
				"resource", mustStringifyObject(e.(types.Object)),
			)
		}
	}
}
