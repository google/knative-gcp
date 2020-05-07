package reconciler

import (
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	deploymentCreated = "DeploymentCreated"
	deploymentUpdated = "DeploymentUpdated"
	serviceCreated = "ServiceCreated"
	serviceUpdated = "ServiceUpdated"
)

type ServiceReconciler struct {
	KubeClient kubernetes.Interface
	ServiceLister corev1listers.ServiceLister
	EndpointsLister corev1listers.EndpointsLister
	Recorder record.EventRecorder
}

type DeploymentReconciler struct {
	KubeClient kubernetes.Interface
	Lister appsv1listers.DeploymentLister
	Recorder record.EventRecorder
}

// ReconcileDeployment reconciles the K8s Deployment 'd'.
func (r *DeploymentReconciler) ReconcileDeployment(obj runtime.Object, d *v1.Deployment) (*v1.Deployment, error) {
	current, err := r.Lister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.AppsV1().Deployments(d.Namespace).Create(d)
		if apierrs.IsAlreadyExists(err) {
			return current, nil
		}
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, deploymentCreated, "Created deployment %s/%s", d.Namespace, d.Name)
		}
		return current, err
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		d, err := r.KubeClient.AppsV1().Deployments(desired.Namespace).Update(desired)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, deploymentUpdated, "Updated deployment %s/%s", d.Namespace, d.Name)
		}
		return d, err
	}
	return current, err
}

// ReconcileService reconciles the K8s Service 'svc'.
func (r *ServiceReconciler) ReconcileService(obj runtime.Object, svc *corev1.Service) (*corev1.Endpoints, error) {
	current, err := r.ServiceLister.Services(svc.Namespace).Get(svc.Name)

	if apierrs.IsNotFound(err) {
		current, err = r.KubeClient.CoreV1().Services(svc.Namespace).Create(svc)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, serviceCreated, "Created service %s/%s", svc.Namespace, svc.Name)
		}
		if err == nil || apierrs.IsAlreadyExists(err) {
			return r.EndpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
		}
		return nil, err
	} else if err != nil {
		return nil, err
	}

	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = svc.Spec
		current, err = r.KubeClient.CoreV1().Services(current.Namespace).Update(desired)
		if err == nil {
			r.Recorder.Eventf(obj, corev1.EventTypeNormal, serviceUpdated, "Updated service %s/%s", svc.Namespace, svc.Name)
		}
		if err != nil {
			return nil, err
		}
	}

	return r.EndpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}
