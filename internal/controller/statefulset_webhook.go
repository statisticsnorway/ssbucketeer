package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	"cloud.google.com/go/storage"
	"github.com/statisticsnorway/ssbucketeer/internal/template"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;get;list;watch;update;patch

//+kubebuilder:webhook:path=/mutate-apps-v1-statefulset,mutating=true,failurePolicy=fail,groups=apps,resources=statefulsets,verbs=create;update,versions=v1,name=mstatefulset.ssbucketeer.dapla.ssb.no,sideEffects=None,admissionReviewVersions=v1

var _ admission.Handler = (*StatefulsetMutator)(nil)

type StatefulsetMutator struct {
	Client  client.Client
	Decoder admission.Decoder

	Storage  *storage.Client
	Projects *rm.ProjectsClient
	Folders  *rm.FoldersClient

	TeamsFolderNumber string
	Stage             string
	IamProbeImage     string
	PrecreatorImage   *string
	ADCGroupEnvName   string

	GroupConfigs AccessGroupConfigs

	SharedBucketTemplate template.AnonymousTemplate[SharedBucketTemplateData]
}

type SharedBucketSpec struct {
	Team      string `json:"team"`
	ShortName string `json:"sharedBucket"`
}

func (m *StatefulsetMutator) SetupWithManager(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/mutate-apps-v1-statefulset", &admission.Webhook{Handler: m})
}

func (m *StatefulsetMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := klog.FromContext(ctx)
	sfs := &appsv1.StatefulSet{}
	err := m.Decoder.Decode(req, sfs)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !sfs.GetDeletionTimestamp().IsZero() {
		return admission.Allowed("statefulset is being deleted")
	}

	group, ok := sfs.Annotations[impersonateGroupAnnotation]
	if !ok {
		return admission.Allowed("skipping ssbucketeer mutation")
	}

	saAnnotations := map[string]string{
		impersonateGroupAnnotation: group,
		accessReasonAnnotation:     sfs.Annotations[accessReasonAnnotation],
	}

	groupConfig := m.GroupConfigs.GetConfig(group)
	if groupConfig == nil {
		return admission.Denied(fmt.Sprintf("no configuration found for group: %s", group))
	}

	if groupConfig.ReasonRequired {
		saAnnotations[requestedServiceDurationAnnotation] = sfs.Annotations[requestedServiceDurationAnnotation]
	}

	// Handle IAM bindings and k8s SA annotations
	if err := m.handleServiceAccount(ctx, req.Namespace, sfs.Spec.Template.Spec.ServiceAccountName, saAnnotations); err != nil {
		log.Error(err, "handle serviceaccount")
		return admission.Denied("error handling service account")
	}

	accessExpired := false
	if groupConfig.ReasonRequired {
		parsedDuration, err := time.ParseDuration(sfs.Annotations[requestedServiceDurationAnnotation])
		if err != nil {
			log.Error(err, "failed to parse requested duration", "durationAnnotation", sfs.Annotations[requestedServiceDurationAnnotation])
			return admission.Denied("invalid requested duration format")
		}

		accessExpired = time.Now().After(sfs.CreationTimestamp.Add(parsedDuration))
	}

	if accessExpired {
		maps.DeleteFunc(sfs.Spec.Template.Annotations, func(k, _ string) bool {
			return strings.HasPrefix(k, "dapla.ssb.no/")
		})
	} else {
		for k, v := range sfs.Annotations {
			if strings.HasPrefix(k, "dapla.ssb.no/") {
				sfs.Spec.Template.Annotations[k] = v
			}
		}
	}

	marshaledStatefulSet, err := json.Marshal(sfs)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledStatefulSet)
}

func (a *StatefulsetMutator) handleServiceAccount(ctx context.Context, namespace, name string, saAnnotations map[string]string) error {
	log := klog.FromContext(ctx)

	sa := &corev1.ServiceAccount{}
	if err := a.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sa); err != nil {
		log.Error(err, "could not get ServiceAccount")
		return err
	}

	if sa.Annotations == nil {
		sa.Annotations = maps.Clone(saAnnotations)
		return a.Client.Update(ctx, sa)
	}

	modified := false
	for key, val := range saAnnotations {
		if sa.Annotations[key] != val {
			modified = true
			sa.Annotations[key] = val
		}
	}

	if modified {
		return a.Client.Update(ctx, sa)
	}
	return nil
}
