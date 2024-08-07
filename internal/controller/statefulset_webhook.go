package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

//+kubebuilder:webhook:path=/mutate-apps-v1-statefulset,mutating=true,failurePolicy=fail,groups=apps,resources=statefulsets,verbs=create;update,versions=v1,name=mstatefulset.ssbucketeer.dapla.ssb.no,sideEffects=None,admissionReviewVersions=v1

var _ admission.Handler = (*StatefulsetMutator)(nil)

type StatefulsetMutator struct {
	Client  client.Client
	Decoder *admission.Decoder

	Storage  *storage.Client
	Projects *rm.ProjectsClient
	Folders  *rm.FoldersClient

	TeamsFolderNumber string
	Stage             string
}

var groupSuffixes = []string{"-developers", "-data-admins"}

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

	if sfs.Annotations[enabledssbucketeerAnnotation] != "true" {
		return admission.Allowed("skipping ssbucketeer mutation")
	}

	ensurePodAnnotations(&sfs.Spec.Template)

	// 1. Check if we want to impersonate a group SA
	group, hasGroupAnnotation := sfs.Annotations[impersonateGroupAnnotation]
	if hasGroupAnnotation {
		// Handle IAM bindings and k8s SA annotations
		if err := m.handleServiceAccount(ctx, req.Namespace, sfs.Spec.Template.Spec.ServiceAccountName, group); err != nil {
			log.Error(err, "handle serviceaccount")
			return admission.Denied("error handling service account")
		}
	}

	// 2. Mount buckets
	var bucketNames []string
	if annotationBuckets, ok := sfs.Annotations[mountBucketsAnnotation]; ok {
		bucketNames = strings.Split(annotationBuckets, ",")
	}

	// Find the service container, so we can add volumeMounts
	containerIndex := slices.IndexFunc(sfs.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == sfs.Annotations[serviceContainerAnnotation]
	})

	if containerIndex == -1 {
		err := fmt.Errorf("could not find container with name %q", sfs.Annotations[serviceContainerAnnotation])
		log.Error(err, "could not find service container")
		return admission.Errored(http.StatusBadRequest, err)
	}

	bucketMounts := make(map[string]string, len(bucketNames))
	for _, bucket := range bucketNames {
		bucketMounts[bucket] = bucket
	}

	// TODO: Use Dapla Team API for this?
	if sfs.Annotations[mountStandardBucketsAnnotation] == "true" {
		team := group
		for _, suffix := range groupSuffixes {
			team = strings.TrimSuffix(group, suffix)
			if team != group {
				if err := m.addStandardBuckets(ctx, team, bucketMounts); err != nil {
					log.Error(err, "failed to add standard buckets")
				}
				break
			}
		}
		if team == group {
			log.Error(errors.New("could not deduce team from group"), "team could not be deduced from group", "group", group)
		}
	}

	addBucketsToPodSpec(&sfs.Spec.Template.Spec, &sfs.Spec.Template.Spec.Containers[containerIndex], bucketMounts)

	if sfs.Annotations[probeCompletedAnnotation] != "true" {
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
								Image:   "europe-north1-docker.pkg.dev/artifact-registry-5n/dapla-lab-docker/alpine-curl:1.0.0",
								Name:    "iam-probe",
								Command: []string{"sh"},
								Args: []string{
									"-c",
									"until curl -sf -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token; do sleep 2s; done; curl -fsI -X POST http://localhost:15020/quitquitquit; exit 0",
								},
							},
						},
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
		}

		if err := m.Client.Create(ctx, probeJob); err != nil {
			log.Error(err, "could not create probe job")
		}

	}

	marshaledStatefulSet, err := json.Marshal(sfs)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledStatefulSet)
}

func (a *StatefulsetMutator) handleServiceAccount(ctx context.Context, namespace, name, group string) error {
	log := klog.FromContext(ctx)

	sa := &corev1.ServiceAccount{}
	if err := a.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, sa); err != nil {
		log.Error(err, "could not get ServiceAccount")
		return err
	}

	if sa.Annotations[impersonateGroupAnnotation] != group {
		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string, 1)
		}
		sa.Annotations[impersonateGroupAnnotation] = group
		if err := a.Client.Update(ctx, sa); err != nil {
			return err
		}
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

func (m *StatefulsetMutator) addStandardBuckets(ctx context.Context, team string, bucketMounts map[string]string) error {
	// teamFolderIt := m.Folders.SearchFolders(ctx, &rmpb.SearchFoldersRequest{
	// 	Query: fmt.Sprintf(`parent=folders/%s AND state=ACTIVE AND displayName=%s`, m.TeamsFolderNumber, team),
	// })

	teamFolderIt := m.Folders.ListFolders(ctx, &rmpb.ListFoldersRequest{
		Parent: fmt.Sprintf("folders/%s", m.TeamsFolderNumber),
	})

	// TODO? there should only ever be one folder with a specfic display name in a folder,
	// do we need to check anything?
	folder, err := getFolderWithDisplayName(teamFolderIt, team)
	if err != nil {
		return fmt.Errorf("get folder %q: %w", team, err)
	}

	projectIt := m.Projects.SearchProjects(ctx, &rmpb.SearchProjectsRequest{
		Query: fmt.Sprintf(`parent=%s AND state=ACTIVE AND displayName=%s`, folder.Name, fmt.Sprintf("%s-%s", team, string(m.Stage[0]))),
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
