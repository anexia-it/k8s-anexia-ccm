package desiredstate

import (
	"reflect"
	"sort"
)

// DifferenceType describes how actual state differs from desired state.
type DifferenceType string

const (
	DifferenceMissing    DifferenceType = "missing"
	DifferenceUnexpected DifferenceType = "unexpected"
	DifferenceChanged    DifferenceType = "changed"
)

// Difference describes one resource-level difference. Desired is nil for an
// unexpected resource and Actual is nil for a missing resource.
type Difference struct {
	Type    DifferenceType `json:"type"`
	Key     string         `json:"key"`
	Desired any            `json:"desired,omitempty"`
	Actual  any            `json:"actual,omitempty"`
}

// Compare returns a deterministic resource-level comparison. Actual Engine
// resources can be converted to State with NormalizeActual.
func Compare(desired, actual State) []Difference {
	desiredByKey := resourcesByKey(desired.Resources())
	actualByKey := resourcesByKey(actual.Resources())
	keys := make([]string, 0, len(desiredByKey)+len(actualByKey))
	for key := range desiredByKey {
		keys = append(keys, key)
	}
	for key := range actualByKey {
		if _, ok := desiredByKey[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	differences := make([]Difference, 0)
	for _, key := range keys {
		desiredResource, desiredOK := desiredByKey[key]
		actualResource, actualOK := actualByKey[key]
		switch {
		case !actualOK:
			differences = append(differences, Difference{Type: DifferenceMissing, Key: key, Desired: desiredResource})
		case !desiredOK:
			differences = append(differences, Difference{Type: DifferenceUnexpected, Key: key, Actual: actualResource})
		case !reflect.DeepEqual(desiredResource, actualResource):
			differences = append(differences, Difference{Type: DifferenceChanged, Key: key, Desired: desiredResource, Actual: actualResource})
		}
	}
	return differences
}

func resourcesByKey(resources []PlannedResource) map[string]PlannedResource {
	ret := make(map[string]PlannedResource, len(resources))
	for _, resource := range resources {
		ret[resource.ResourceKey()] = resource
	}
	return ret
}
