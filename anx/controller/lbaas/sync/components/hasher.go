package components

import (
	"fmt"
	"reflect"
)

type Hasher interface {
	Hash() Hash
}

func ToHasher(elem interface{}) []Hasher {
	var result []Hasher
	value := reflect.ValueOf(elem)
	kind := value.Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			hasher := value.Index(i).Interface().(Hasher)
			result = append(result, hasher)
		}
	default:
		panic(fmt.Sprintf("only arrays and strings are supported for 'ToHasher': got %s", value.Kind().String()))
	}

	return result
}
