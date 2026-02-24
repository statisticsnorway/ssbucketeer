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
	"fmt"
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	klog "sigs.k8s.io/controller-runtime/pkg/log"
)

// JobReconciler reconciles a Job object
type JobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update

func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := klog.FromContext(ctx)

	var job batchv1.Job
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "could not fetch Job")
		return ctrl.Result{}, err
	}

	// If the Job does not have this annotation, it is not relevant to us
	sfsName, ok := job.Annotations[probeJobStatefulsetAnnotation]
	if !ok {
		return ctrl.Result{}, nil
	}

	// The job has not completed, or StatefulSet has been updated, so we have nothing to do
	if job.Status.CompletionTime.IsZero() || !controllerutil.ContainsFinalizer(&job, finalizerName) {
		return ctrl.Result{}, nil
	}

	// Get the StatefulSet the IAM probe job was spawned for
	var sfs appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: sfsName}, &sfs); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Delete(ctx, &job); err != nil {
				if apierrors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				log.Error(err, "could not delete Job for non-existent StatefulSet")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Error(err, "could not get StatefulSet")
		return ctrl.Result{}, err
	}

	// If the job status is `done`, we have nothing to do, should remove finalizer if set
	if sfs.Annotations[iamProbeStatus] == iamProbeDone {
		if controllerutil.RemoveFinalizer(&job, finalizerName) {
			if err := r.Update(ctx, &job); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Parse the IAM probe status to find the original replica count
	replicasStr := strings.TrimPrefix(sfs.Annotations[iamProbeStatus], iamProbeRunningPrefix)
	replicas, err := strconv.Atoi(replicasStr)
	if err != nil {
		log.Error(err, "invalid iamProbeStatus %q, defaulting to 1 replica")
		replicas = 1
	}

	sfs.Spec.Replicas = ptr.To(int32(replicas))
	sfs.Annotations[iamProbeStatus] = iamProbeDone

	// Find the service container, so we can add volumeMounts
	containerIndex := slices.IndexFunc(sfs.Spec.Template.Spec.Containers, func(c corev1.Container) bool {
		return c.Name == sfs.Annotations[serviceContainerAnnotation]
	})

	if containerIndex == -1 {
		err := fmt.Errorf("could not find container with name %q", sfs.Annotations[serviceContainerAnnotation])
		log.Error(err, "could not find service container")
		return ctrl.Result{}, err
	}
	refreshBucketsConfigMap := corev1.ConfigMap{
		Data: map[string]string{
			"refresh-buckets": "#!/bin/bash\ncurl http://localhost:8383/refresh-folders",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sfs.Namespace,
			Name:      fmt.Sprintf("%s-refresh-buckets", sfs.Name),
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "StatefulSet",
					APIVersion: "apps/v1",
					Name:       sfs.Name,
					UID:        sfs.UID,
				},
			},
		},
	}
	if err = r.Create(ctx, &refreshBucketsConfigMap); err != nil {
		return ctrl.Result{}, err
	}
	refreshBucketsVolume := corev1.Volume{
		Name: "refresh-buckets-command",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: refreshBucketsConfigMap.Name,
				},
				DefaultMode: ptr.To[int32](0o555), // Read, execute
			},
		},
	}
	refreshBucketsVolumeMount := corev1.VolumeMount{
		Name:      refreshBucketsVolume.Name,
		MountPath: "/usr/bin/refresh-buckets",
		SubPath:   "refresh-buckets",
	}
	sfs.Spec.Template.Spec.Volumes = append(sfs.Spec.Template.Spec.Volumes, refreshBucketsVolume)
	sfs.Spec.Template.Spec.Containers[containerIndex].VolumeMounts = append(sfs.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, refreshBucketsVolumeMount)

	if err := r.Update(ctx, &sfs); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "could not update StatefulSet")
		return ctrl.Result{}, err
	}

	if controllerutil.RemoveFinalizer(&job, finalizerName) {
		if err := r.Update(ctx, &job); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "failed to remove finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}
