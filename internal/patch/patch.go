package patch

import (
	"encoding/json"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateTwoWayMergePatch creates a patch that can be passed to StrategicMergePatch between
// the original and desired objects. It will return a [client.Patch] or an error if either of
// the two documents is invalid.
func CreateTwoWayMergePatch[T client.Object](original, desired T, fns ...mergepatch.PreconditionFunc) (client.Patch, error) {
	var object T

	data, err := strategicpatch.CreateTwoWayMergePatch(lo.Must(json.Marshal(original)), lo.Must(json.Marshal(desired)), object, fns...)
	if err != nil {
		return nil, err
	}

	return client.RawPatch(types.StrategicMergePatchType, data), nil
}
