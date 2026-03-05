package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"cloud.google.com/go/storage"
	"github.com/statisticsnorway/ssbucketeer/internal/template"
	"google.golang.org/api/iterator"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;get;list;watch;update;patch

//+kubebuilder:webhook:path=/mutate-core-v1-pod,mutating=true,failurePolicy=fail,groups=core,resources=pods,verbs=create;update,versions=v1,name=mpod.ssbucketeer.dapla.ssb.no,sideEffects=None,admissionReviewVersions=v1

var _ admission.Handler = (*PodMutator)(nil)

type PodMutator struct {
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

func (m *PodMutator) SetupWithManager(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register("/mutate-core-v1-pod", &admission.Webhook{Handler: m})
}

func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := klog.FromContext(ctx)
	pod := &corev1.Pod{}
	err := m.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !pod.GetDeletionTimestamp().IsZero() {
		return admission.Allowed("pod is being deleted")
	}

	group, ok := pod.Annotations[impersonateGroupAnnotation]
	if !ok {
		return admission.Allowed("skipping ssbucketeer mutation")
	}

	ensurePodAnnotations(pod)

	// Find the service container, so we can add volumeMounts
	serviceContainer := getServiceContainer(pod, pod.Annotations[serviceContainerAnnotation])
	if serviceContainer == nil {
		err := fmt.Errorf("could not find container with name %q", pod.Annotations[serviceContainerAnnotation])
		log.Error(err, "could not find service container")
		return admission.Errored(http.StatusBadRequest, err)
	}

	saAnnotations := map[string]string{
		impersonateGroupAnnotation: group,
		accessReasonAnnotation:     pod.Annotations[accessReasonAnnotation],
	}

	groupConfig := m.GroupConfigs.GetConfig(group)
	if groupConfig == nil {
		return admission.Denied(fmt.Sprintf("no configuration found for group: %s", group))
	}
	team := groupConfig.ToTeam(group)

	if groupConfig.ReasonRequired {
		saAnnotations[requestedServiceDurationAnnotation] = pod.Annotations[requestedServiceDurationAnnotation]
	}

	// Handle IAM bindings and k8s SA annotations
	if err := m.handleServiceAccount(ctx, req.Namespace, pod.Spec.ServiceAccountName, saAnnotations); err != nil {
		log.Error(err, "handle serviceaccount")
		return admission.Denied("error handling service account")
	}

	if pod.Labels == nil {
		pod.Labels = make(map[string]string, 2)
	}
	pod.Labels[daplaTeamLabel] = team
	pod.Labels[daplaGroupLabel] = group

	if m.ADCGroupEnvName != "" {
		if !slices.ContainsFunc(serviceContainer.Env, func(e corev1.EnvVar) bool {
			return e.Name == m.ADCGroupEnvName
		}) {
			serviceContainer.Env = append(serviceContainer.Env, corev1.EnvVar{Name: m.ADCGroupEnvName, Value: group})
		}
	}

	accessExpired := false
	if groupConfig.ReasonRequired {
		parsedDuration, err := time.ParseDuration(pod.Annotations[requestedServiceDurationAnnotation])
		if err != nil {
			log.Error(err, "failed to parse requested duration", "durationAnnotation", pod.Annotations[requestedServiceDurationAnnotation])
			return admission.Denied("invalid requested duration format")
		}

		accessExpired = time.Now().After(pod.CreationTimestamp.Add(parsedDuration))
	}

	if accessExpired {
		removeBucketsFromPodSpec(&pod.Spec, serviceContainer)
		removePrecreator(&pod.Spec)
	} else {
		bucketMounts := getExtraBucketMounts(pod.Annotations)

		if sharedBuckets, ok := pod.Annotations[mountSharedBucketsAnnotation]; ok {
			if err := m.addSharedBuckets(bucketMounts, sharedBuckets); err != nil {
				log.Error(err, "failed to add shared buckets")
			}
		}

		// TODO: Use Dapla Team API for this?
		if pod.Annotations[mountStandardBucketsAnnotation] == "true" {
			if err := m.addStandardBuckets(ctx, team, *groupConfig, bucketMounts); err != nil {
				log.Error(err, "failed to add standard buckets")
			}
		}

		addBucketsToPodSpec(&pod.Spec, serviceContainer, bucketMounts)
		if m.PrecreatorImage != nil {
			addOrUpdatePrecreator(&pod.Spec, bucketMounts, *m.PrecreatorImage)
		} else {
			removePrecreator(&pod.Spec)
		}
	}

	if m.IamProbeImage != "" && !accessExpired {
		addOrUpdateIamProbe(&pod.Spec, m.IamProbeImage)
	} else {
		removeIamProbe(&pod.Spec)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
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

func getServiceContainer(pod *corev1.Pod, name string) *corev1.Container {
	idx := slices.IndexFunc(pod.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == name
	})
	if idx == -1 {
		return nil
	}
	return &pod.Spec.Containers[idx]
}

func (a *PodMutator) handleServiceAccount(ctx context.Context, namespace, name string, saAnnotations map[string]string) error {
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

func (m *PodMutator) addStandardBuckets(ctx context.Context, team string, gc AccessGroupConfig, bucketMounts map[string]string) error {
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
		return fmt.Errorf("execute template %s: %w", gc.ProjectTemplate.Name(), err)
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
		if strings.HasPrefix(withoutPrefix, "delt-delomat-") {
			continue
		}
		bucketMounts[withoutPrefix] = bucket.Name
	}

	return nil
}

func (m *PodMutator) addSharedBuckets(bucketMounts map[string]string, sharedBuckets string) error {
	bucketSpecs := []SharedBucketSpec{}

	if err := json.Unmarshal([]byte(sharedBuckets), &bucketSpecs); err != nil {
		return err
	}

	for _, bucket := range bucketSpecs {
		bucket.Team = strings.TrimSpace(bucket.Team)
		bucket.ShortName = strings.TrimSpace(bucket.ShortName)
		if bucket.Team == "" || bucket.ShortName == "" {
			// Ignore empty specs
			continue
		}

		mountPoint := fmt.Sprintf("shared/%s/%s", bucket.Team, bucket.ShortName)
		bucket, err := m.SharedBucketTemplate.Execute(SharedBucketTemplateData{
			TeamName:        bucket.Team,
			BucketShortName: bucket.ShortName,
			Stage:           m.Stage,
		})
		if err != nil {
			return err
		}
		bucketMounts[mountPoint] = bucket
	}
	return nil
}

func ensurePodAnnotations(pod *corev1.Pod) {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string, 2)
	}

	pod.Annotations["gke-gcsfuse/volumes"] = "true"

	// Add necessary istio outbound IP exclusion if missing
	excludeOutboundIpRangesValue, ok := pod.Annotations[istioExcludedIpRangesAnnotation]
	if !ok {
		pod.Annotations[istioExcludedIpRangesAnnotation] = gcsfuseOutboundIPRange
	} else {
		excludeOutboundIPRanges := strings.Split(excludeOutboundIpRangesValue, ",")
		if !slices.Contains(excludeOutboundIPRanges, gcsfuseOutboundIPRange) {
			excludeOutboundIPRanges = append(excludeOutboundIPRanges, gcsfuseOutboundIPRange)
			pod.Annotations[istioExcludedIpRangesAnnotation] = strings.Join(excludeOutboundIPRanges, ",")
		}
	}
}
