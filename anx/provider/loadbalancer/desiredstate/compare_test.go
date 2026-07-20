package desiredstate

import (
	"reflect"
	"testing"
)

func TestCompare(t *testing.T) {
	t.Parallel()

	desired, err := Build(validInput())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	actual := desired
	actual.Backends = append([]Backend(nil), desired.Backends...)
	actual.Servers = nil
	actual.Frontends = append([]Frontend(nil), desired.Frontends...)
	actual.Binds = append([]Bind(nil), desired.Binds...)
	actual.Backends[0].HealthCheck = "different"
	actual.Binds = append(actual.Binds, Bind{
		Key:         "bind/lb-a/unexpected",
		Name:        "unexpected",
		FrontendKey: actual.Frontends[0].Key,
		Address:     "192.0.2.99",
		Port:        9999,
	})

	differences := Compare(desired, actual)
	if got, want := len(differences), 4; got != want {
		t.Fatalf("len(Compare()) = %d, want %d: %#v", got, want, differences)
	}

	wantTypes := []DifferenceType{
		DifferenceChanged,
		DifferenceUnexpected,
		DifferenceMissing,
		DifferenceMissing,
	}
	gotTypes := make([]DifferenceType, 0, len(differences))
	for _, difference := range differences {
		gotTypes = append(gotTypes, difference.Type)
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Errorf("difference types = %v, want %v", gotTypes, wantTypes)
	}

	if got := Compare(desired, desired); len(got) != 0 {
		t.Errorf("Compare(state, state) = %#v, want no differences", got)
	}
}
