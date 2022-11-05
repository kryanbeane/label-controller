package controllers

import (
	"context"

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
)

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the Pod
	var foundPod v1.Pod
	err := r.Get(ctx, req.NamespacedName, &foundPod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Logging the error and returning nil to requeue the reconciler
			log.Log.Error(err, "Pod Not Found")
			return ctrl.Result{}, nil
		}
		log.Log.Error(err, "Unexpected Error Getting Pod")
		return ctrl.Result{}, err
	}

	log.Log.Info("Found Pod", "Pod", foundPod)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Pod{}).
		Complete(r)
}
