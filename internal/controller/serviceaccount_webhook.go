package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate--v1-serviceaccount,mutating=false,failurePolicy=fail,groups="",resources=serviceaccounts,verbs=create;update,versions=v1,name=vserviceaccount.ssbucketeer.dapla.ssb.no,sideEffects=None,admissionReviewVersions=v1

var _ webhook.CustomValidator = (*ServiceAccountValidator)(nil)

type IllegalImpersionationError struct {
	User           string
	RequestedGroup string
}

func (e IllegalImpersionationError) Error() string {
	return fmt.Sprintf("user %q tried to impersonate group %q, which they are not a member of", e.User, e.RequestedGroup)
}

type IllegalNamespaceError struct {
	Namespace      string
	RequestedGroup string
}

func (e IllegalNamespaceError) Error() string {
	return fmt.Sprintf("attempted to impersonate group %q from non-user namespace %q", e.RequestedGroup, e.Namespace)
}

type ServiceAccountValidator struct {
	Auth Auther
}

func (m *ServiceAccountValidator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithValidator(m).
		Complete()
}

func (m *ServiceAccountValidator) ValidateCreate(ctx context.Context, req runtime.Object) (admission.Warnings, error) {
	return m.validate(ctx, req)
}

func (m *ServiceAccountValidator) ValidateUpdate(ctx context.Context, old, new runtime.Object) (admission.Warnings, error) {
	return m.validate(ctx, new)
}

func (m *ServiceAccountValidator) ValidateDelete(ctx context.Context, req runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validate checks if a group impersonation is requested, and, if so, whether the user (namespace)
// is allowed to impersonate that group.
func (m *ServiceAccountValidator) validate(ctx context.Context, req runtime.Object) (admission.Warnings, error) {
	log := klog.FromContext(ctx)

	sa, ok := req.(*corev1.ServiceAccount)
	if !ok {
		return nil, fmt.Errorf("expected a ServiceAccount, but got a %T", req)
	}

	group, ok := sa.Annotations[impersonateGroupAnnotation]
	if !ok {
		return nil, nil
	}

	user := strings.TrimPrefix(sa.Namespace, userNamespacePrefix)
	if user == sa.Namespace {
		err := IllegalNamespaceError{Namespace: sa.Namespace, RequestedGroup: group}
		log.Error(err, "attempt to impersonate group from non-user namespace")
		return nil, err
	}

	isMember, err := m.Auth.UserIsMemberOf(user, group)
	if err != nil {
		log.Error(err, "failed to look up group membership")
		return nil, err
	}

	if !isMember {
		err := IllegalImpersionationError{User: user, RequestedGroup: group}
		log.Error(err, "user tried to impersonate a group they are not a member of")
		return nil, err
	}

	return nil, nil
}
