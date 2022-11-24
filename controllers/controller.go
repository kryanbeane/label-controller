package controllers

import (
	"context"
	"github.com/sirupsen/logrus"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	// Annotations
	addPodLabelAnnotation = "label-controller/add-label"

	// Labels
	podNameLabel     = "label-controller/pod-name"
	podNodeNameLabel = "label-controller/pod-node-name"
	podIpLabel       = "label-controller/pod-ip"

	requeueTime = 30 * time.Second
)

type Controller struct {
	client.Client
	*runtime.Scheme
}

func (c Controller) Add(mgr manager.Manager) error {
	controller, err := ctr.New("label-controller", mgr,
		ctr.Options{Reconciler: &Controller{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}})
	if err != nil {
		logrus.Errorf("failed to create the label controller: %v", err)
		return err
	}

	err = controller.Watch(
		&source.Kind{Type: &v1.Pod{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		logrus.Errorf("failed to watch for pods: %v", err)
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

func (c Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// Fetch the pod being reconciled
	var foundPod v1.Pod

	err := c.Get(ctx, request.NamespacedName, &foundPod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Logging the error and re-queueing after the requeue time
			logrus.Errorf("pod Not Found %s", err)
			return ctrl.Result{Requeue: true, RequeueAfter: requeueTime}, nil
		}
		logrus.Errorf("unexpected Error Getting Pod %v", err)
		return ctrl.Result{}, err

	} else {
		logrus.Infof("reconciling pod %s", foundPod.Name)
	}

	// Sync Pod Name Label
	nameLabelExpected := foundPod.Annotations[addPodLabelAnnotation] == "pod-name"
	nameLabelPresent := foundPod.Labels[podNameLabel] == foundPod.Name
	nameAnnotationMissing := foundPod.Annotations[addPodLabelAnnotation] == ""
	if nameLabelExpected && !nameLabelPresent {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod name label is not present when it should be, adding it to pod %s", foundPod.Name)
		res := c.syncPodNameLabel(ctx, foundPod, true)
		return res, err

	} else if !nameLabelExpected && nameLabelPresent || (nameAnnotationMissing && nameLabelPresent) {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod name label is present when it shouldn't be, removing it from pod %s", foundPod.Name)
		res := c.syncPodNameLabel(ctx, foundPod, false)
		return res, err
	}

	// Sync Node Name Label
	nodeNameLabelExpected := foundPod.Annotations[addPodLabelAnnotation] == "node-name"
	nodeNameLabelPresent := foundPod.Labels[podNodeNameLabel] == foundPod.Spec.NodeName
	nodeNameAnnotationMissing := foundPod.Annotations[addPodLabelAnnotation] == ""
	if nodeNameLabelExpected && !nodeNameLabelPresent {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod node name label is not present when it should be, adding it to pod %s", foundPod.Name)
		if err := c.syncPodNodeNameLabel(ctx, foundPod, true); err != nil {
			return ctrl.Result{}, err
		}
	} else if !nodeNameLabelExpected && nodeNameLabelPresent || (nodeNameAnnotationMissing && nodeNameLabelPresent) {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod node name label is present when it shouldn't be, removing it from pod %s", foundPod.Name)
		if err := c.syncPodNodeNameLabel(ctx, foundPod, false); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync Pod IP Label
	ipLabelExpected := foundPod.Annotations[addPodLabelAnnotation] == "pod-ip"
	ipLabelPresent := foundPod.Labels[podIpLabel] == foundPod.Status.PodIP
	ipAnnotationMissing := foundPod.Annotations[addPodLabelAnnotation] == ""
	if ipLabelExpected && !ipLabelPresent {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod ip label is not present when it should be, adding it to pod %s", foundPod.Name)
		if err := c.syncPodIp(ctx, foundPod, true); err != nil {
			return reconcile.Result{}, err
		}
	} else if !ipLabelExpected && ipLabelPresent || (ipAnnotationMissing && ipLabelPresent) {
		logrus.Infof("LABEL-CONTROLLER-ACTION: pod ip label is present when it shouldn't be, removing it from pod %s", foundPod.Name)
		if err := c.syncPodIp(ctx, foundPod, false); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (c Controller) syncPodNameLabel(ctx context.Context, pod v1.Pod, add bool) reconcile.Result {
	// If labels is nil then init it
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	if add {
		// Add the label to the pod
		pod.Labels[podNameLabel] = pod.Name
		logrus.Infof("LABEL-CONTROLLER-ACTION: adding name label to pod %s", pod.Name)
	} else if !add {
		// Remove the label from the pod
		delete(pod.Labels, podNameLabel)
		logrus.Infof("LABEL-CONTROLLER-ACTION: removing name label from pod %s", pod.Name)
	}

	if err := c.patchPod(ctx, pod); err != nil {
		return reconcile.Result{Requeue: true}
	}

	return reconcile.Result{}
}

func (c Controller) syncPodNodeNameLabel(ctx context.Context, pod v1.Pod, add bool) error {
	// If labels is nil then init it
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	if add {
		// Add the label to the pod
		pod.Labels[podNodeNameLabel] = pod.Spec.NodeName
		logrus.Infof("LABEL-CONTROLLER-ACTION: adding node name label to pod %s", pod.Spec.NodeName)
	} else if !add {
		// Remove the label from the pod
		delete(pod.Labels, podNodeNameLabel)
		logrus.Infof("LABEL-CONTROLLER-ACTION: removing node name label from pod %s", pod.Spec.NodeName)
	}

	if err := c.patchPod(ctx, pod); err != nil {
		return err
	}

	return nil
}

func (c Controller) syncPodIp(ctx context.Context, pod v1.Pod, add bool) error {
	// If labels is nil then init it
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	if add {
		// Add the label to the pod
		pod.Labels[podIpLabel] = pod.Status.PodIP
		logrus.Infof("LABEL-CONTROLLER-ACTION: adding pod ip label to pod %s", pod.Status.PodIP)

	} else if !add {
		// Remove the label from the pod
		delete(pod.Labels, podIpLabel)
		logrus.Infof("LABEL-CONTROLLER-ACTION: removing pod ip label from pod %s", pod.Status.PodIP)
	}

	if err := c.patchPod(ctx, pod); err != nil {
		return err
	}

	return nil
}

func (c Controller) patchPod(ctx context.Context, pod v1.Pod) error {
	if err := c.Update(ctx, &pod); err != nil {
		// Check if pod was not found (has been deleted since initial fetch)
		if errors.IsNotFound(err) {
			logrus.Warnf("pod Not Found %s", err)
			return err
		}
		// Check if pod has been changed in the meantime
		if errors.IsConflict(err) {
			logrus.Warnf("conflict while updating pod %s, the pod reference is outdated, retry", pod.Name)
			return err
		}
	}
	logrus.Infof("LABEL-CONTROLLER-ACTION: successfully updated pod %s", pod.Name)
	return nil
}
