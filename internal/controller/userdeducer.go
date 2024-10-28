package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UsernameDeducer interface {
	FromNamespace(ctx context.Context, namespace string) (username string, err error)
}

type UsernameDeducers []UsernameDeducer

func (ds UsernameDeducers) FromNamespace(ctx context.Context, namespace string) (username string, err error) {
	if len(ds) == 0 {
		return "", errors.New("no UsernameDeducers configured")
	}
	var allErr error
	for _, d := range ds {
		if user, err := d.FromNamespace(ctx, namespace); err != nil {
			allErr = errors.Join(allErr, err)
		} else {
			return user, nil
		}
	}
	return "", allErr
}

// PrefixUsernameDeducer uses a simple "namespace-prefix" approach to deduce the username
type PrefixUsernameDeducer struct {
	Prefix string
}

func (d PrefixUsernameDeducer) FromNamespace(ctx context.Context, namespace string) (string, error) {
	if username := strings.TrimPrefix(namespace, d.Prefix); username != namespace {
		return username, nil
	}
	return "", fmt.Errorf("namespace %q does not have a %q prefix", namespace, d.Prefix)
}

// NamespaceAnnotationUsernameDeducer looks for an annotation on the namespace to deduce the username
type NamespaceAnnotationUsernameDeducer struct {
	Annotation string
	Client     client.Client
}

func (d *NamespaceAnnotationUsernameDeducer) FromNamespace(ctx context.Context, namespace string) (string, error) {
	var ns corev1.Namespace
	if err := d.Client.Get(ctx, types.NamespacedName{Name: namespace}, &ns); err != nil {
		return "", err
	}

	if username, ok := ns.Annotations[d.Annotation]; ok {
		return username, nil
	}

	return "", fmt.Errorf("namespace %q does not have a %q annotation", namespace, d.Annotation)
}
