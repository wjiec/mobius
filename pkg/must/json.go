package must

import "encoding/json"

// JsonMarshal marshals the given value v to a JSON format data.
//
// If marshalling fails, it panics with the encountered error.
func JsonMarshal[T any](v T) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
