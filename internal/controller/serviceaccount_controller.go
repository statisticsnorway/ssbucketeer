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
	"net/http"
	"slices"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceAccountReconciler reconciles a ServiceAccount object
type ServiceAccountReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Auth Memberships
	Iam  *iam.Service

	DaplaGroupSaProject string
	ClusterProjectId    string

	GroupConfigs []AccessGroupConfig
}

// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/finalizers,verbs=update

func (r *ServiceAccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sa corev1.ServiceAccount
	if err := r.Get(ctx, req.NamespacedName, &sa); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "could not fetch ServiceAccount")
		return ctrl.Result{}, err
	}

	// We don't care about SAs that don't have this annotation
	group, ok := sa.Annotations[impersonateGroupAnnotation]
	if !ok {
		return ctrl.Result{}, nil
	}

	// 1. Finalizer handling
	if sa.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(&sa, finalizerName) {
			if controllerutil.AddFinalizer(&sa, finalizerName) {
				if err := r.Update(ctx, &sa); err != nil {
					log.Error(err, "failed to add finalizer")
					return ctrl.Result{}, err
				}
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&sa, finalizerName) {
			if res, err := r.removeIamBinding(ctx, group, req.NamespacedName); err != nil {
				return res, err
			}

			if controllerutil.RemoveFinalizer(&sa, finalizerName) {
				if err := r.Update(ctx, &sa); err != nil {
					log.Error(err, "failed to remove finalizer")
					return ctrl.Result{}, err
				}
			}
		}

		return ctrl.Result{}, nil
	}

	// Handle the needed IAM bindings
	if res, err := r.handleGcpSa(ctx, &sa, group, req.NamespacedName); err != nil {
		return res, err
	}

	// Add GKE WI annotation to SA
	gcpSa := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", group, r.DaplaGroupSaProject)
	if sa.Annotations[gkeWIAnnotation] != gcpSa {
		sa.Annotations[gkeWIAnnotation] = gcpSa
		if err := r.Update(ctx, &sa); err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "failed to add GKE WI annotation to ServiceAccount")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceAccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		Named("serviceaccount").
		Complete(r)
}

func (r *ServiceAccountReconciler) removeIamBinding(ctx context.Context, group string, nn types.NamespacedName) (ctrl.Result, error) {
	gcpSaRef := toGcpSaRef(r.DaplaGroupSaProject, group)
	k8sSaRef := toK8sSaRef(r.ClusterProjectId, nn.Namespace, nn.Name)
	log := logf.FromContext(ctx, "googleSaRef", gcpSaRef, "kubernetesSaRef", k8sSaRef)

	// Get the current IAM policy set on this SA
	policy, err := r.Iam.Projects.ServiceAccounts.GetIamPolicy(gcpSaRef).OptionsRequestedPolicyVersion(3).Do()
	if err != nil {
		// We don't care about NotFound errors
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == http.StatusNotFound {
			log.Info("service account no longer exists")
			return ctrl.Result{}, nil
		}

		log.Error(err, "could not get IAM policy")
		return ctrl.Result{}, err
	}

	// We need to use a "dummy condition" to uniquely identify this K8s SA's binding
	conditionString := iamConditionString(nn)

	// No policy set on SA
	if policy == nil {
		return ctrl.Result{}, nil
	}

	lenBefore := len(policy.Bindings)
	policy.Bindings = slices.DeleteFunc(policy.Bindings,
		func(b *iam.Binding) bool { return b.Condition.Title == conditionString },
	)

	if len(policy.Bindings) == lenBefore {
		return ctrl.Result{}, nil
	}

	if res, err := r.updateIamPolicy(ctx, policy, gcpSaRef); err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) handleGcpSa(ctx context.Context, sa *corev1.ServiceAccount, group string, nn types.NamespacedName) (ctrl.Result, error) {
	gcpSaRef := toGcpSaRef(r.DaplaGroupSaProject, group)
	k8sSaRef := toK8sSaRef(r.ClusterProjectId, nn.Namespace, nn.Name)
	log := logf.FromContext(ctx, "googleSaRef", gcpSaRef, "kubernetesSaRef", k8sSaRef)

	// Get the current IAM policy set on this SA
	policy, err := r.Iam.Projects.ServiceAccounts.GetIamPolicy(gcpSaRef).OptionsRequestedPolicyVersion(3).Do()
	if err != nil {
		// Ignore non-existent SAs, but log a warning
		var gErr *googleapi.Error
		if errors.As(err, &gErr) && gErr.Code == http.StatusNotFound {
			log.Info("cannot set IAM policy on non-existent SA")
			return ctrl.Result{}, nil
		}
		log.Error(err, "could not get IAM policy")
		return ctrl.Result{}, err
	}

	// We need to use a "dummy condition" to uniquely identify this K8s SA's binding
	conditionString := iamConditionString(nn)

	// Fetch the impersonation duration directly from the SA annotation (set by StatefulSet webhook)
	saImpersonationDurationStr := sa.Annotations[requestedServiceDurationAnnotation]
	expirationExpr := "true" //nolint:goconst
	if saImpersonationDurationStr != "" {
		parsedDuration, err := time.ParseDuration(saImpersonationDurationStr)
		if err != nil {
			log.Error(err, "failed to parse ServiceAccount impersonation duration", "duration", saImpersonationDurationStr)
			return ctrl.Result{}, fmt.Errorf("invalid duration: %s", saImpersonationDurationStr)
		}
		expirationTime := sa.CreationTimestamp.Add(parsedDuration).UTC().Format(time.RFC3339)
		expirationExpr = fmt.Sprintf("request.time < timestamp('%s')", expirationTime)
	}

	// No policy set on SA
	if policy == nil {
		policy = &iam.Policy{}
	}

	// Check if the binding already exists in the policy
	var binding *iam.Binding
	if bindingIndex := slices.IndexFunc(policy.Bindings,
		func(b *iam.Binding) bool { return b.Condition.Title == conditionString },
	); bindingIndex != -1 {
		binding = policy.Bindings[bindingIndex]
	}

	wantedBinding := &iam.Binding{
		Role:    wiRole,
		Members: []string{k8sSaRef},
		Condition: &iam.Expr{
			Title:      conditionString,
			Expression: expirationExpr,
		},
	}

	if modified := ensureCorrectBinding(ctx, policy, binding, wantedBinding); !modified {
		return ctrl.Result{}, nil
	}

	if res, err := r.updateIamPolicy(ctx, policy, gcpSaRef); err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (r *ServiceAccountReconciler) updateIamPolicy(ctx context.Context, policy *iam.Policy, gcpSaRef string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	policy.Version = 3
	policy, err := r.Iam.Projects.ServiceAccounts.SetIamPolicy(gcpSaRef, &iam.SetIamPolicyRequest{
		Policy:     policy,
		UpdateMask: "bindings,etag",
	}).Do()

	if policy != nil && policy.HTTPStatusCode == http.StatusConflict {
		log.Info("concurrency error in IAM API (policy was modified during reconcile), requeueing")
		return ctrl.Result{Requeue: true}, errIamConcurrencyError
	}
	return ctrl.Result{}, err
}

func ensureCorrectBinding(ctx context.Context, policy *iam.Policy, binding, wantedBinding *iam.Binding) (modified bool) {
	log := logf.FromContext(ctx)

	if binding == nil {
		if wantedBinding == nil {
			return false
		}

		policy.Bindings = append(policy.Bindings, wantedBinding)
		return true
	}

	if wantedBinding == nil {
		policy.Bindings = slices.DeleteFunc(policy.Bindings,
			func(b *iam.Binding) bool { return b.Condition.Title == binding.Condition.Title },
		)
		return true
	}

	shouldUpdate := false
	logIncorrect := func(s string) {
		log.Info(s,
			"role", binding.Role,
			"members", binding.Members,
			"conditionTitle", binding.Condition.Title,
			"conditionExpression", binding.Condition.Expression,
		)
	}
	if binding.Role != wiRole {
		logIncorrect("current binding has incorrect role, correcting")
		shouldUpdate = true
	}
	if len(binding.Members) != 1 || binding.Members[0] != wantedBinding.Members[0] {
		logIncorrect("member list has incorrect length or members, correcting")
		shouldUpdate = true
	}
	if binding.Condition.Expression != wantedBinding.Condition.Expression {
		logIncorrect("binding condition is incorrect, correcting")
		shouldUpdate = true
	}
	if shouldUpdate {
		*binding = *wantedBinding
	}
	return shouldUpdate
}

func toGcpSaRef(projectId, groupName string) string {
	const format = "projects/%[1]s/serviceAccounts/%[2]s@%[1]s.iam.gserviceaccount.com"
	return fmt.Sprintf(format, projectId, groupName)
}

func toK8sSaRef(clusterProjectId, namespace, name string) string {
	const format = "serviceAccount:%s.svc.id.goog[%s/%s]"
	return fmt.Sprintf(format, clusterProjectId, namespace, name)
}

func iamConditionString(nn types.NamespacedName) string {
	return fmt.Sprintf("%s=%s", iamConditionKey, nn.String())
}
