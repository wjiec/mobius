package fieldindex

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IndexOwnerReferenceUID = "metadata.controller.uid"
)

func ownerReferenceUIDIndex(object client.Object) []string {
	owners := make([]string, 1) // a slices of size 1 is sufficient
	for _, owner := range object.GetOwnerReferences() {
		owners = append(owners, string(owner.UID))
	}

	return owners
}

func RegisterFieldIndexes(ctx context.Context, indexer client.FieldIndexer) (err error) {
	err = errors.Join(err, indexer.IndexField(ctx, &corev1.Service{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))
	err = errors.Join(err, indexer.IndexField(ctx, &corev1.Endpoints{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))
	err = errors.Join(err, indexer.IndexField(ctx, &networkingv1.Ingress{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))

	return err
}
