/*
Copyright 2026.

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

	schedulingv1 "github.com/PavIek/gpusched/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GPUJobReconciler reconciles a GPUJob object
type GPUJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=scheduling.gpusched.io,resources=gpujobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.gpusched.io,resources=gpujobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.gpusched.io,resources=gpujobs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GPUJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *GPUJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling GPUJob", "namespacedName", req.NamespacedName)

	var job schedulingv1.GPUJob
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GPUJob")
		return ctrl.Result{}, err
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingLabels{"gpujob-name": job.Name}); err != nil {
		logger.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	var running, succeeded, failed int32
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		}
	}

	currentPods := int32(len(pods.Items))
	desiredPods := job.Spec.Replicas
	if currentPods < desiredPods {
		for i := currentPods; i < desiredPods; i++ {
			pod := r.newPodForJob(&job, i)
			if err := r.Create(ctx, pod); err != nil {
				logger.Error(err, "Failed to create pod")
				return ctrl.Result{}, err
			}
			logger.Info("Created pod", "pod", pod.Name)
		}
	} else if currentPods > desiredPods {
		for i := desiredPods; i < currentPods; i++ {
			pod := &pods.Items[i]
			if err := r.Delete(ctx, pod); err != nil {
				logger.Error(err, "Failed to delete pod")
				return ctrl.Result{}, err
			}
			logger.Info("Deleted pod", "pod", pod.Name)
		}
	}

	job.Status.Scheduled = currentPods
	job.Status.Running = running
	job.Status.Succeeded = succeeded
	job.Status.Failed = failed

	if succeeded == desiredPods {
		job.Status.Phase = "Succeeded"
	} else if failed > 0 {
		job.Status.Phase = "Failed"
	} else if running > 0 {
		job.Status.Phase = "Running"
	} else if currentPods > 0 {
		job.Status.Phase = "Scheduling"
	} else {
		job.Status.Phase = "Pending"
	}

	if err := r.Status().Update(ctx, &job); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GPUJobReconciler) newPodForJob(job *schedulingv1.GPUJob, index int32) *corev1.Pod {
	labels := map[string]string{
		"gpujob-name": job.Name,
		"gpujob-uid":  string(job.UID),
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-worker-%d", job.Name, index),
			Namespace: job.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, schedulingv1.GroupVersion.WithKind("GPUJob")),
			},
		},
		Spec: corev1.PodSpec{
			SchedulerName: "gpusched-scheduler",
			Containers: []corev1.Container{
				{
					Name:    "gpu-worker",
					Image:   job.Spec.Image,
					Command: job.Spec.Command,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"nvidia.com/gpu": *resource.NewQuantity(int64(job.Spec.GPUsPerPod), resource.DecimalSI),
						},
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": *resource.NewQuantity(int64(job.Spec.GPUsPerPod), resource.DecimalSI),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	return pod
}

// SetupWithManager sets up the controller with the Manager.
func (r *GPUJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulingv1.GPUJob{}).
		Owns(&corev1.Pod{}).
		Named("gpujob").
		Complete(r)
}
