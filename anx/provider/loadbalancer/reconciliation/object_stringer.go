package reconciliation

import (
	"errors"
	"fmt"

	"go.anx.io/go-anxcloud/pkg/api/types"
)

func stringifyObject(o types.Object) (string, error) {
	identifier, err := types.GetObjectIdentifier(o, true)
	if err != nil && errors.Is(err, types.ErrUnidentifiedObject) {
		identifier = "<no identifier>"
	} else if err != nil {
		return "", err
	}

	return fmt.Sprintf("%T:%v", o, identifier), nil
}

func stringifyObjects(os []types.Object) ([]string, error) {
	ret := make([]string, 0, len(os))
	for _, o := range os {
		s, err := stringifyObject(o)
		if err != nil {
			return nil, err
		}

		ret = append(ret, s)
	}
	return ret, nil
}

func mustStringifyObject(o types.Object) string {
	ret, err := stringifyObject(o)
	if err != nil {
		panic(fmt.Errorf("stringifyObject failed in mustStringifyObject: %w", err))
	}
	return ret
}

func mustStringifyObjects(os []types.Object) []string {
	ret, err := stringifyObjects(os)
	if err != nil {
		panic(fmt.Errorf("stringifyObjects failed in mustStringifyObjects: %w", err))
	}
	return ret
}
