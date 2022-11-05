package controllers

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	addPodNameLabelAnnotation = "cloud-demo/add-pod-name-label"
	podNameLabel              = "cloud-demo/pod-name"
	requeueTime               = 30 * time.Second
)

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch pods
	var foundPod v1.Pod
	err := r.Get(ctx, req.NamespacedName, &foundPod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Logging the error and re-queueing after the requeue time
			log.Log.Error(err, "Pod Not Found")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueTime}, nil
		}
		log.Log.Error(err, "Unexpected Error Getting Pod")
		return ctrl.Result{}, err

	} else {
		log.Log.Info("Found pod", "Pod", foundPod)
	}

	// Match pod actual state with desired state
	labelIsExpected := foundPod.Annotations[addPodNameLabelAnnotation] == "true"
	labelIsPresent := foundPod.Labels[podNameLabel] == foundPod.Name

	if labelIsExpected == labelIsPresent {
		log.Log.Info("Pod label is as expected, no action needed")
		return ctrl.Result{}, nil
	}

	// Update pod label based on annotation value
	if labelIsExpected {
		// Check for non-nil labels map and create it if nil
		if foundPod.Labels == nil {
			foundPod.Labels = make(map[string]string)
		}
		// Add label to pod
		foundPod.Labels[podNameLabel] = foundPod.Name
		log.Log.Info("Adding label to pod")

	} else {
		// If label is not expected, check if it exists
		if foundPod.Labels[podNameLabel] != "" {
			delete(foundPod.Labels, podNameLabel)
			log.Log.Info("Removing label from pod")
		}
	}

	err = r.Update(ctx, &foundPod)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		if errors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Pod{}).
		Complete(r)
}
