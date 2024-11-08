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
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	sfsName, ok := job.Annotations[iamProbeStatefulsetAnnotation]
	if !ok {
		return ctrl.Result{}, nil
	}

	// The job has not completed, and so we have nothing to do
	if job.Status.CompletionTime.IsZero() {
		return ctrl.Result{}, nil
	}

	// Get the StatefulSet the IAM probe job was spawned for
	var sfs appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: sfsName}, &sfs); err != nil {
		log.Error(err, "could not get StatefulSet")
		return ctrl.Result{}, err
	}

	// If the job status is `done`, we want to delete this job
	if sfs.Annotations[iamProbeStatus] == iamProbeDone {
		err := client.IgnoreNotFound(r.Delete(ctx, &job))
		if err != nil {
			log.Error(err, "failed to queue job deletion")
		}
		return ctrl.Result{}, err
	}

	replicas, err := strconv.Atoi(sfs.Annotations[iamProbeStatefulsetReplicas])
	if err != nil {
		log.Error(err, "invalid replicas value, defaulting to 1 replica", "replicas", sfs.Annotations[iamProbeStatus])
		replicas = 1
	}

	sfs.Spec.Replicas = ptr(int32(replicas))
	sfs.Annotations[iamProbeStatus] = iamProbeDone

	if err := r.Update(ctx, &sfs); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "could not update StatefulSet")
		return ctrl.Result{}, err
	}

	if err := r.Delete(ctx, &job); client.IgnoreNotFound(err) != nil {
		log.Error(err, "failed to queue job deletion")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		Complete(r)
}
