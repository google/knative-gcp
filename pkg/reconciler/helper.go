package reconciler

import (
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// ReconcileDeployment reconciles the K8s Deployment 'd'.
func ReconcileDeployment(kubeClient kubernetes.Interface, lister appsv1listers.DeploymentLister, d *v1.Deployment) (*v1.Deployment, error) {
	current, err := lister.Deployments(d.Namespace).Get(d.Name)
	if apierrs.IsNotFound(err) {
		current, err = kubeClient.AppsV1().Deployments(d.Namespace).Create(d)
		if apierrs.IsAlreadyExists(err) {
			return current, nil
		}
		return current, err
	} else if err != nil {
		return nil, err
	} else if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		// Don't modify the informers copy.
		desired := current.DeepCopy()
		desired.Spec = d.Spec
		return kubeClient.AppsV1().Deployments(desired.Namespace).Update(desired)
	}
	return current, err
}

// ReconcileService reconciles the K8s Service 'svc'.
func ReconcileService(kubeClient kubernetes.Interface, svcLister corev1listers.ServiceLister, endpointsLister corev1listers.EndpointsLister, svc *corev1.Service) (*corev1.Endpoints, error) {
	current, err := svcLister.Services(svc.Namespace).Get(svc.Name)

	if apierrs.IsNotFound(err) {
		current, err = kubeClient.CoreV1().Services(svc.Namespace).Create(svc)
		if err == nil || apierrs.IsAlreadyExists(err) {
			return endpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
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
		current, err = kubeClient.CoreV1().Services(current.Namespace).Update(desired)
		if err != nil {
			return nil, err
		}
	}

	return endpointsLister.Endpoints(svc.Namespace).Get(svc.Name)
}
