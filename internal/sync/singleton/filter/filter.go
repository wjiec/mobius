package filter

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Filter is a type alias for a function that filters and selects
// an object from a list based on the controller.
type Filter[C, R client.Object] func(controller C, objects []R) R

// First returns the first object from the list. If the
// list is empty, it returns the zero value.
func First[C, R client.Object](_ C, objects []R) R {
	if len(objects) != 0 {
		return objects[0]
	}

	var zero R
	return zero
}

// SameName returns the object from the list that has the same name as the controller.
func SameName[C, R client.Object](controller C, objects []R) R {
	return OverrideName[C, R](C.GetName)(controller, objects)
}

// OverrideName creates a filter that selects an object with a name matching the
// extractor's result for the controller.
func OverrideName[C, R client.Object](extractor func(C) string) Filter[C, R] {
	return func(controller C, objects []R) R {
		for _, object := range objects {
			if extractor(controller) == object.GetName() {
				return object
			}
		}

		var zero R
		return zero
	}
}

// Or creates a filter that applies multiple filters in
// sequence and returns the first non-zero result.
func Or[C, R client.Object](filters ...Filter[C, R]) Filter[C, R] {
	return func(controller C, objects []R) R {
		for _, filter := range filters {
			filtered := filter(controller, objects)
			if !reflect.ValueOf(filtered).IsZero() {
				return filtered
			}
		}

		var zero R
		return zero
	}
}
