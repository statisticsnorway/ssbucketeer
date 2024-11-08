package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/statisticsnorway/ssbucketeer/internal/audit"
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
	Auth         Auther
	GroupConfigs AccessGroupConfigs
	Audit        audit.Router
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
// is allowed to impersonate that group and whether the duration is within the allowed limit.
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

	groupType := m.GroupConfigs.GetConfig(group)
	if groupType == nil {
		return nil, fmt.Errorf("no group config matches %q", group)
	}

	team := strings.TrimPrefix(group, groupType.Name)

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

	durationStr, hasDuration := sa.Annotations[requestedServiceDurationAnnotation]
	reason, hasReason := sa.Annotations[accessReasonAnnotation]

	if !groupType.ReasonRequired {
		return nil, nil
	}
	if !hasDuration {
		return nil, fmt.Errorf("duration required for group %q of type %q", group, groupType.Name)
	}

	parsedDuration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Error(err, "failed to parse ServiceAccount duration", "duration", durationStr)
		return nil, fmt.Errorf("invalid duration: %s", durationStr)
	}

	if parsedDuration > groupType.MaxDuration {
		return nil, fmt.Errorf("requested duration %q exceeds max allowed %q for group %q", parsedDuration, groupType.MaxDuration, group)
	}

	if !hasReason {
		return nil, fmt.Errorf("reason required for group %q of type %q", group, groupType.Name)
	}

	chartName, chartVersion := getChartNameAndVersion(*sa)
	instanceName, instanceNamespace := getInstanceMeta(*sa)

	auditEntry := audit.Payload{
		UserPrincipalName: user,
		TeamName:          team,
		AccessGroup:       group,
		GroupType:         groupType.Name,
		Reason:            reason,
		StartTime:         sa.CreationTimestamp.Time,
		EndTime:           sa.CreationTimestamp.Add(parsedDuration),
		Duration:          parsedDuration,
		Service: audit.Service{
			Chart: audit.ChartMeta{
				Name:    chartName,
				Version: chartVersion,
			},
			Instance: audit.InstanceMeta{
				Name:      instanceName,
				Namespace: instanceNamespace,
			},
		},
	}

	if auditErr := m.Audit.RecordAll(auditEntry); auditErr != nil {
		log.Error(auditErr, "error delivering audit payload to one or more sinks", "payload", auditEntry)
	}

	return nil, nil
}

func getChartNameAndVersion(sa corev1.ServiceAccount) (name string, version string) {
	const helmMetaLabel = "helm.sh/chart"
	if meta, ok := sa.Labels[helmMetaLabel]; ok {
		split := strings.Split(meta, "-")
		return strings.Join(split[:len(split)-1], "-"), split[len(split)-1]
	}
	return "unknown", "unknown"
}

func getInstanceMeta(sa corev1.ServiceAccount) (name string, namespace string) {
	const (
		helmNameAnnotation      = "meta.helm.sh/release-name"
		helmNamespaceAnnotation = "meta.helm.sh/release-namespace"
	)

	name, ok := sa.Annotations[helmNameAnnotation]
	if !ok {
		name = "unknown"
	}

	namespace, ok = sa.Annotations[helmNamespaceAnnotation]
	if !ok {
		namespace = "unknown"
	}

	return name, namespace
}
