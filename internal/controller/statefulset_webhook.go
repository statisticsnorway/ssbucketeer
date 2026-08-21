package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"cloud.google.com/go/storage"
	"github.com/statisticsnorway/ssbucketeer/internal/template"
	"google.golang.org/api/iterator"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=create;get;list;watch;update;patch

// +kubebuilder:webhook:path=/mutate-apps-v1-statefulset,mutating=true,failurePolicy=fail,groups=apps,resources=statefulsets,verbs=create;update,versions=v1,name=mstatefulset.ssbucketeer.dapla.ssb.no,sideEffects=None,admissionReviewVersions=v1

var _ admission.Defaulter[*appsv1.StatefulSet] = (*StatefulsetMutator)(nil)

type StatefulsetMutator struct {
	Client  client.Client
	Decoder admission.Decoder

	Storage  *storage.Client
	Projects *rm.ProjectsClient
	Folders  *rm.FoldersClient

	TeamsFolderNumber     string
	Stage                 string
	IamProbeImage         string
	PrecreatorImage       *string
	ADCGroupEnvName       string
	TeamGcpProjectEnvName string

	GroupConfigs AccessGroupConfigs

	SharedBucketTemplate template.AnonymousTemplate[SharedBucketTemplateData]
}

type SharedBucketSpec struct {
	Team      string `json:"team"`
	ShortName string `json:"sharedBucket"`
}

func (m *StatefulsetMutator) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &appsv1.StatefulSet{}).WithDefaulter(m).Complete()
}

func (m *StatefulsetMutator) Default(ctx context.Context, sfs *appsv1.StatefulSet) error {
	log := klog.FromContext(ctx)

	if !sfs.GetDeletionTimestamp().IsZero() {
		return nil
	}

	group, ok := sfs.Annotations[impersonateGroupAnnotation]
	if !ok {
		return nil
	}

	ensurePodAnnotations(&sfs.Spec.Template)

	// Find the service container, so we can add volumeMounts
	serviceContainer := getServiceContainer(&sfs.Spec.Template, sfs.Annotations[serviceContainerAnnotation])
	if serviceContainer == nil {
		err := fmt.Errorf("could not find container with name %q", sfs.Annotations[serviceContainerAnnotation])
		log.Error(err, "could not find service container")
		return err
	}

	saAnnotations := map[string]string{
		impersonateGroupAnnotation: group,
		accessReasonAnnotation:     sfs.Annotations[accessReasonAnnotation],
	}

	groupConfig := m.GroupConfigs.GetConfig(group)
	if groupConfig == nil {
		return fmt.Errorf("no configuration found for group: %s", group)
	}
	team := groupConfig.ToTeam(group)

	if groupConfig.ReasonRequired {
		saAnnotations[requestedServiceDurationAnnotation] = sfs.Annotations[requestedServiceDurationAnnotation]
	}

	// Handle IAM bindings and k8s SA annotations
	if err := m.handleServiceAccount(ctx, sfs.Namespace, sfs.Spec.Template.Spec.ServiceAccountName, saAnnotations); err != nil {
		log.Error(err, "handle serviceaccount")
		return fmt.Errorf("error handling service account: %w", err)
	}

	if sfs.Spec.Template.Labels == nil {
		sfs.Spec.Template.Labels = make(map[string]string, 2)
	}
	sfs.Spec.Template.Labels[daplaTeamLabel] = team
	sfs.Spec.Template.Labels[daplaGroupLabel] = group

	if m.ADCGroupEnvName != "" {
		if !slices.ContainsFunc(serviceContainer.Env, func(e corev1.EnvVar) bool {
			return e.Name == m.ADCGroupEnvName
		}) {
			serviceContainer.Env = append(serviceContainer.Env, corev1.EnvVar{Name: m.ADCGroupEnvName, Value: group})
		}
	}

	teamGoogleProject, err := m.getStandardProject(ctx, team, *groupConfig)
	if err != nil {
		log.Error(err, "could not find standard project")
	}

	if m.TeamGcpProjectEnvName != "" {
		teamGoogleProjectId := "-1"
		if teamGoogleProject != nil {
			teamGoogleProjectId = teamGoogleProject.ProjectId
		}

		envIndex := slices.IndexFunc(serviceContainer.Env, func(e corev1.EnvVar) bool {
			return e.Name == m.TeamGcpProjectEnvName
		})
		if envIndex == -1 {
			serviceContainer.Env = append(serviceContainer.Env, corev1.EnvVar{Name: m.TeamGcpProjectEnvName, Value: teamGoogleProjectId})
		} else {
			serviceContainer.Env[envIndex].Value = teamGoogleProjectId
		}
	}

	accessExpired := false
	if groupConfig.ReasonRequired {
		parsedDuration, err := time.ParseDuration(sfs.Annotations[requestedServiceDurationAnnotation])
		if err != nil {
			log.Error(err, "failed to parse requested duration", "durationAnnotation", sfs.Annotations[requestedServiceDurationAnnotation])
			return fmt.Errorf("invalid requested duration format %q", sfs.Annotations[requestedServiceDurationAnnotation])
		}

		accessExpired = time.Now().After(sfs.CreationTimestamp.Add(parsedDuration))
	}

	if accessExpired {
		removeBucketsFromPodSpec(&sfs.Spec.Template.Spec, serviceContainer)
		removePrecreator(&sfs.Spec.Template.Spec)
	} else {
		bucketMounts := getExtraBucketMounts(sfs.Annotations)

		if sharedBuckets, ok := sfs.Annotations[mountSharedBucketsAnnotation]; ok {
			if err := m.addSharedBuckets(bucketMounts, sharedBuckets); err != nil {
				log.Error(err, "failed to add shared buckets")
			}
		}

		// TODO: Use Dapla Team API for this?
		if sfs.Annotations[mountStandardBucketsAnnotation] == "true" && teamGoogleProject != nil { //nolint:goconst
			if err := m.addStandardBuckets(ctx, team, teamGoogleProject.ProjectId, bucketMounts); err != nil {
				log.Error(err, "failed to add standard buckets")
			}
		}

		addBucketsToPodSpec(&sfs.Spec.Template.Spec, serviceContainer, bucketMounts)
		if m.PrecreatorImage != nil {
			addOrUpdatePrecreator(&sfs.Spec.Template.Spec, bucketMounts, *m.PrecreatorImage)
		} else {
			removePrecreator(&sfs.Spec.Template.Spec)
		}
	}

	if m.IamProbeImage != "" && sfs.Annotations[iamProbeStatus] != iamProbeDone {
		if err := m.launchIamProbe(ctx, sfs); err != nil {
			log.Error(err, "could not start iam probe")
		}
	}

	return nil
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
	sfs.Spec.Replicas = ptr.To[int32](0)

	probeJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-iam-probe", sfs.Name),
			Namespace: sfs.Namespace,
			Annotations: map[string]string{
				probeJobStatefulsetAnnotation: sfs.Name,
			},
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds:   ptr.To[int64](300),
			TTLSecondsAfterFinished: ptr.To[int32](0),
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
	// Ensure Job isn't deleted before we've revived the StatefulSet
	controllerutil.AddFinalizer(probeJob, finalizerName)

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

func (m *StatefulsetMutator) getStandardProject(ctx context.Context, team string, gc AccessGroupConfig) (*rmpb.Project, error) {
	teamFolderIt := m.Folders.ListFolders(ctx, &rmpb.ListFoldersRequest{
		Parent: fmt.Sprintf("folders/%s", m.TeamsFolderNumber),
	})

	// TODO? there should only ever be one folder with a specific display name in a folder,
	// do we need to check anything?
	folder, err := getFolderWithDisplayName(teamFolderIt, team)
	if err != nil {
		return nil, fmt.Errorf("get folder %q: %w", team, err)
	}

	projectName, err := gc.ProjectTemplate.Execute(ProjectTemplateData{TeamName: team, Stage: m.Stage})
	if err != nil {
		return nil, fmt.Errorf("execute template %s: %w", gc.ProjectTemplate.Name(), err)
	}

	projectIt := m.Projects.SearchProjects(ctx, &rmpb.SearchProjectsRequest{
		Query: fmt.Sprintf(`parent=%s AND state=ACTIVE AND displayName=%s`, folder.Name, projectName),
	})

	project, err := projectIt.Next()
	if err != nil {
		return nil, fmt.Errorf("get project %q in folder %q: %w", team, folder.Name, err)
	}

	return project, nil
}

func (m *StatefulsetMutator) addStandardBuckets(ctx context.Context, team string, projectId string, bucketMounts map[string]string) error {
	bucketPrefix := fmt.Sprintf("ssb-%s-data-", team)
	bucketIt := m.Storage.Buckets(ctx, projectId)
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

func (m *StatefulsetMutator) addSharedBuckets(bucketMounts map[string]string, sharedBuckets string) error {
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

func ensurePodAnnotations(podTemplate *corev1.PodTemplateSpec) {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = make(map[string]string, 2)
	}

	podTemplate.Annotations["gke-gcsfuse/volumes"] = "true"

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
