package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	PrecreatorImage   string
	ADCGroupEnvName   string

	GroupConfigs AccessGroupConfigs
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

	ensurePodAnnotations(&sfs.Spec.Template)

	// Find the service container, so we can add volumeMounts
	serviceContainer := getServiceContainer(&sfs.Spec.Template, sfs.Annotations[serviceContainerAnnotation])
	if serviceContainer == nil {
		err := fmt.Errorf("could not find container with name %q", sfs.Annotations[serviceContainerAnnotation])
		log.Error(err, "could not find service container")
		return admission.Errored(http.StatusBadRequest, err)
	}

	bucketMounts := getExtraBucketMounts(sfs.Annotations)

	saAnnotations := map[string]string{
		impersonateGroupAnnotation:         group,
		requestedServiceDurationAnnotation: sfs.Annotations[requestedServiceDurationAnnotation],
		accessReasonAnnotation:             sfs.Annotations[accessReasonAnnotation],
	}

	groupConfig := m.GroupConfigs.GetConfig(group)
	if groupConfig == nil {
		return admission.Denied(fmt.Sprintf("no configuration found for group: %s", group))
	}

	// Handle IAM bindings and k8s SA annotations
	if err := m.handleServiceAccount(ctx, req.Namespace, sfs.Spec.Template.Spec.ServiceAccountName, saAnnotations); err != nil {
		log.Error(err, "handle serviceaccount")
		return admission.Denied("error handling service account")
	}

	if m.ADCGroupEnvName != "" {
		if !slices.ContainsFunc(serviceContainer.Env, func(e corev1.EnvVar) bool {
			return e.Name == m.ADCGroupEnvName
		}) {
			serviceContainer.Env = append(serviceContainer.Env, corev1.EnvVar{Name: m.ADCGroupEnvName, Value: group})
		}
	}

	// TODO: Use Dapla Team API for this?
	if sfs.Annotations[mountStandardBucketsAnnotation] == "true" {
		team := groupConfig.ToTeam(group)
		if err := m.addStandardBuckets(ctx, team, *groupConfig, bucketMounts); err != nil {
			log.Error(err, "failed to add standard buckets")
		}
	}

	addBucketsToPodSpec(&sfs.Spec.Template.Spec, serviceContainer, bucketMounts, m.PrecreatorImage)

	if m.IamProbeImage != "" && sfs.Annotations[iamProbeStatus] != iamProbeDone {
		if err := m.launchIamProbe(ctx, sfs); err != nil {
			log.Error(err, "could not start iam probe")
		}
	}

	marshaledStatefulSet, err := json.Marshal(sfs)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledStatefulSet)
}

func getExtraBucketMounts(annotations map[string]string) map[string]string {
	var bucketNames []string
	if annotationBuckets, ok := annotations[mountBucketsAnnotation]; ok {
		bucketNames = strings.Split(annotationBuckets, ",")
	}

	bucketMounts := make(map[string]string, len(bucketNames))
	for _, bucket := range bucketNames {
		bucketMounts[bucket] = bucket
	}
	return bucketMounts
}

func getServiceContainer(pod *corev1.PodTemplateSpec, name string) *corev1.Container {
	idx := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == name
	})
	if idx == -1 {
		return nil
	}
	return &pod.Spec.Containers[idx]
}

func (m *StatefulsetMutator) launchIamProbe(ctx context.Context, sfs *appsv1.StatefulSet) error {
	sfs.Annotations[iamProbeStatus] = fmt.Sprintf("%s%d", iamProbeRunningPrefix, *sfs.Spec.Replicas)
	sfs.Spec.Replicas = ptr[int32](0)

	probeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-iam-probe", sfs.Name),
			Namespace: sfs.Namespace,
			Annotations: map[string]string{
				probeJobStatefulsetAnnotation: sfs.Name,
			},
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds:   ptr[int64](300),
			TTLSecondsAfterFinished: ptr[int32](0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						istioExcludedIpRangesAnnotation: gcsfuseOutboundIPRange,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: sfs.Spec.Template.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Image: m.IamProbeImage,
							Name:  "iam-probe",
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	return m.Client.Create(ctx, probeJob)
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

func getFolderWithDisplayName(it *rm.FolderIterator, displayName string) (*rmpb.Folder, error) {
	for {
		folder, err := it.Next()
		if err != nil {
			return nil, err
		}
		if folder.DisplayName == displayName {
			return folder, nil
		}
	}
}

func (m *StatefulsetMutator) addStandardBuckets(ctx context.Context, team string, gc AccessGroupConfig, bucketMounts map[string]string) error {
	teamFolderIt := m.Folders.ListFolders(ctx, &rmpb.ListFoldersRequest{
		Parent: fmt.Sprintf("folders/%s", m.TeamsFolderNumber),
	})

	// TODO? there should only ever be one folder with a specfic display name in a folder,
	// do we need to check anything?
	folder, err := getFolderWithDisplayName(teamFolderIt, team)
	if err != nil {
		return fmt.Errorf("get folder %q: %w", team, err)
	}

	projectName, err := gc.ProjectTemplate.Execute(ProjectTemplateData{TeamName: team, Stage: m.Stage})
	if err != nil {
		return fmt.Errorf("execute template %s: %w", gc.ProjectTemplate.template.Name(), err)
	}

	projectIt := m.Projects.SearchProjects(ctx, &rmpb.SearchProjectsRequest{
		Query: fmt.Sprintf(`parent=%s AND state=ACTIVE AND displayName=%s`, folder.Name, projectName),
	})

	project, err := projectIt.Next()
	if err != nil {
		return fmt.Errorf("get project %q in folder %q: %w", team, folder.Name, err)
	}

	bucketPrefix := fmt.Sprintf("ssb-%s-data-", team)
	bucketIt := m.Storage.Buckets(ctx, project.ProjectId)
	bucketIt.Prefix = bucketPrefix

	for {
		bucket, err := bucketIt.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("get bucket: %w", err)
		}
		withoutPrefix := strings.TrimSuffix(
			strings.TrimPrefix(bucket.Name, bucketPrefix),
			fmt.Sprintf("-%s", m.Stage),
		)
		bucketMounts[withoutPrefix] = bucket.Name
	}

	return nil
}

func ensurePodAnnotations(podTemplate *corev1.PodTemplateSpec) {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = make(map[string]string, 2)
	}

	if podTemplate.Annotations["gke-gcsfuse/volumes"] != "true" {
		podTemplate.Annotations["gke-gcsfuse/volumes"] = "true"
	}

	// Add necessary istio outbound IP exclusion if missing
	excludeOutboundIpRangesValue, ok := podTemplate.Annotations[istioExcludedIpRangesAnnotation]
	if !ok {
		podTemplate.Annotations[istioExcludedIpRangesAnnotation] = gcsfuseOutboundIPRange
	} else {
		excludeOutboundIPRanges := strings.Split(excludeOutboundIpRangesValue, ",")
		if !slices.Contains(excludeOutboundIPRanges, gcsfuseOutboundIPRange) {
			excludeOutboundIPRanges = append(excludeOutboundIPRanges, gcsfuseOutboundIPRange)
			podTemplate.Annotations[istioExcludedIpRangesAnnotation] = strings.Join(excludeOutboundIPRanges, ",")
		}
	}
}
