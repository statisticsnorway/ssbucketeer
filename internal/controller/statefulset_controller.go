/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	rm "cloud.google.com/go/resourcemanager/apiv3"
	rmpb "cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
)

// StatefulSetReconciler reconciles a StatefulSet object
type StatefulSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Auth Auther

	Storage  *storage.Client
	Projects *rm.ProjectsClient
	Folders  *rm.FoldersClient

	DaplaGroupSaProject string
	TeamsFolderNumber   string
	Stage               string
}

var groupSuffixes = []string{"-developers", "-data-admins"}

//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)

	// Get the StatefulSet
	var sfs appsv1.StatefulSet
	if err := r.Get(ctx, req.NamespacedName, &sfs); err != nil {
		// Ignore NotFound errors, as the sfs may have been deleted
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "could not fetch StatefulSet")
		return ctrl.Result{}, err
	}

	// Don't do anything if missing an enabled:true annotation or being deleted
	if sfs.Annotations[enabledssbucketeerAnnotation] != "true" || !sfs.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	// 0. Add necessary pod annotations for gcsfuse

	if modified := ensurePodAnnotations(&sfs.Spec.Template); modified {
		if err := r.Update(ctx, &sfs); err != nil {
			log.Error(err, "failed to add annotations to pod template")
			return determineResult(ctrl.Result{}, err)
		}
	}

	// 1. Check if we want to impersonate a group SA
	group, hasGroupAnnotation := sfs.Annotations[impersonateGroupAnnotation]
	if hasGroupAnnotation {
		// We only accept namespaces which have the "user namespace prefix"
		user := strings.TrimPrefix(req.Namespace, userNamespacePrefix)
		if user == req.Namespace {
			log.Error(fmt.Errorf("attempted to impersonate group %q from non-user namespace %q", group, req.Namespace), "attempt to impersonate group from non-user namespace")
			return ctrl.Result{}, nil
		}
		// Do not allow non-members to impersonate a group SA
		isMember, err := r.Auth.UserIsMemberOf(user, group)
		if err != nil {
			log.Error(err, "failed to look up group membership")
			return ctrl.Result{}, err
		}
		if !isMember {
			log.Error(fmt.Errorf("user %q is not a member of group %s", user, group), "user tried to impersonate a group they are not a member of")
			return ctrl.Result{}, nil
		}
		// Handle IAM bindings and k8s SA annotations
		if res, err := r.handleServiceAccount(ctx, req.Namespace, sfs.Spec.Template.Spec.ServiceAccountName, group); err != nil {
			return determineResult(res, err)
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
		log.Error(fmt.Errorf("could not find container with name %q", sfs.Annotations[serviceContainerAnnotation]), "could not find service container")
		return ctrl.Result{}, nil
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
				if err := r.addStandardBuckets(ctx, team, bucketMounts); err != nil {
					log.Error(err, "failed to add standard buckets")
				}
				break
			}
		}
		if team == group {
			log.Error(errors.New("could not deduce team from group"), "team could not be deduced form group", "group", group)
		}
	}

	if modified := addBucketsToPodSpec(&sfs.Spec.Template.Spec, &sfs.Spec.Template.Spec.Containers[containerIndex], bucketMounts); modified {
		if err := r.Update(ctx, &sfs); err != nil {
			log.Error(err, "failed to update StatefulSet")
			return determineResult(ctrl.Result{}, err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatefulSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.StatefulSet{}).
		Complete(r)
}

// ptr is a convenience function for generating pointers of primitive types
func ptr[T any](t T) *T {
	return &t
}

func determineResult(res ctrl.Result, err error) (ctrl.Result, error) {
	if apierrors.IsConflict(err) {
		res.Requeue = true
		err = nil
	}

	return res, err
}

func (r *StatefulSetReconciler) addStandardBuckets(ctx context.Context, team string, bucketMounts map[string]string) error {
	teamFolderIt := r.Folders.SearchFolders(ctx, &rmpb.SearchFoldersRequest{
		Query: fmt.Sprintf(`parent=folders/%s AND state=ACTIVE AND displayName=%s`, r.TeamsFolderNumber, team),
	})

	// TODO? there should only ever be one folder with a specfic display name in a folder,
	// do we need to check anything?
	folder, err := teamFolderIt.Next()
	if err != nil {
		return fmt.Errorf("get folder %q: %w", team, err)
	}

	projectIt := r.Projects.SearchProjects(ctx, &rmpb.SearchProjectsRequest{
		Query: fmt.Sprintf(`parent=%s AND state=ACTIVE AND displayName=%s`, folder.Name, fmt.Sprintf("%s-%s", team, string(r.Stage[0]))),
	})

	project, err := projectIt.Next()
	if err != nil {
		return fmt.Errorf("get project %q in folder %q: %w", team, folder.Name, err)
	}

	bucketPrefix := fmt.Sprintf("ssb-%s-data-", team)
	bucketIt := r.Storage.Buckets(ctx, project.ProjectId)
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
			fmt.Sprintf("-%s", r.Stage),
		)
		bucketMounts[withoutPrefix] = bucket.Name
	}

	return nil
}

func ensurePodAnnotations(podTemplate *corev1.PodTemplateSpec) (modified bool) {
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = make(map[string]string, 2)
	}

	missingPodAnnotations := false
	if podTemplate.Annotations["gke-gcsfuse/volumes"] != "true" {
		missingPodAnnotations = true
		podTemplate.Annotations["gke-gcsfuse/volumes"] = "true"
	}

	// Add necessary istio outbound IP exclusion if missing
	excludeOutboundIpRangesValue, ok := podTemplate.Annotations[istioExcludedIpRangesAnnotation]
	if !ok {
		missingPodAnnotations = true
		podTemplate.Annotations[istioExcludedIpRangesAnnotation] = gcsfuseOutboundIPRange
	} else {
		excludeOutboundIPRanges := strings.Split(excludeOutboundIpRangesValue, ",")
		if !slices.Contains(excludeOutboundIPRanges, gcsfuseOutboundIPRange) {
			missingPodAnnotations = true
			excludeOutboundIPRanges = append(excludeOutboundIPRanges, gcsfuseOutboundIPRange)
			podTemplate.Annotations[istioExcludedIpRangesAnnotation] = strings.Join(excludeOutboundIPRanges, ",")
		}
	}

	return missingPodAnnotations
}

func (r *StatefulSetReconciler) handleServiceAccount(ctx context.Context, namespace, name, group string) (ctrl.Result, error) {
	log := klog.FromContext(ctx)

	var sa corev1.ServiceAccount
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sa); err != nil {
		log.Error(err, "could not get ServiceAccount")
		return ctrl.Result{}, err
	}

	if sa.Annotations[impersonateGroupAnnotation] != group {
		if sa.Annotations == nil {
			sa.Annotations = make(map[string]string, 1)
		}
		sa.Annotations[impersonateGroupAnnotation] = group
		if err := r.Update(ctx, &sa); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func ignoreErrs(err error, errs ...error) error {
	for _, ignore := range errs {
		if err == ignore {
			return nil
		}
	}
	return err
}
