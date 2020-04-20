/*
Copyright 2019 The Knative Authors

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

package testing

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/reconciler/testing"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
	servingv1listers "knative.dev/serving/pkg/client/listers/serving/v1"
	servingv1alpha1listers "knative.dev/serving/pkg/client/listers/serving/v1alpha1"
	servingv1beta1listers "knative.dev/serving/pkg/client/listers/serving/v1beta1"

	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakeservingclientset "knative.dev/serving/pkg/client/clientset/versioned/fake"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	EventsV1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	MessagingV1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	policyv1alpha1 "github.com/google/knative-gcp/pkg/apis/policy/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	fakeeventsclientset "github.com/google/knative-gcp/pkg/client/clientset/versioned/fake"
	brokerlisters "github.com/google/knative-gcp/pkg/client/listers/broker/v1beta1"
	eventslisters "github.com/google/knative-gcp/pkg/client/listers/events/v1alpha1"
	messaginglisters "github.com/google/knative-gcp/pkg/client/listers/messaging/v1alpha1"
	policylisters "github.com/google/knative-gcp/pkg/client/listers/policy/v1alpha1"
	pubsublisters "github.com/google/knative-gcp/pkg/client/listers/pubsub/v1alpha1"

	fakeistioclientset "github.com/google/knative-gcp/pkg/client/istio/clientset/versioned/fake"
	istiov1beta1listers "github.com/google/knative-gcp/pkg/client/istio/listers/security/v1beta1"
	istiov1beta1 "istio.io/client-go/pkg/apis/security/v1beta1"
)

var sinkAddToScheme = func(scheme *runtime.Scheme) error {
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "testing.cloud.google.com", Version: "v1alpha1", Kind: "Sink"}, &unstructured.Unstructured{})
	return nil
}

var clientSetSchemes = []func(*runtime.Scheme) error{
	fakekubeclientset.AddToScheme,
	fakeeventsclientset.AddToScheme,
	fakeservingclientset.AddToScheme,
	fakeistioclientset.AddToScheme,
	sinkAddToScheme,
}

type Listers struct {
	sorter testing.ObjectSorter
	servingv1alpha1listers.ConfigurationLister
}

func NewListers(objs []runtime.Object) Listers {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}

	ls := Listers{
		sorter: testing.NewObjectSorter(scheme),
	}

	ls.sorter.AddObjects(objs...)

	return ls
}

func (l *Listers) indexerFor(obj runtime.Object) cache.Indexer {
	return l.sorter.IndexerForObjectType(obj)
}

func (l *Listers) GetKubeObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakekubeclientset.AddToScheme)
}

func (l *Listers) GetEventsObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakeeventsclientset.AddToScheme)
}

func (l *Listers) GetSinkObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(sinkAddToScheme)
}

func (l *Listers) GetAllObjects() []runtime.Object {
	all := l.GetSinkObjects()
	all = append(all, l.GetEventsObjects()...)
	all = append(all, l.GetKubeObjects()...)
	return all
}

func (l *Listers) GetServingObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakeservingclientset.AddToScheme)
}

func (l *Listers) GetIstioObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakeistioclientset.AddToScheme)
}

func (l *Listers) GetPullSubscriptionLister() pubsublisters.PullSubscriptionLister {
	return pubsublisters.NewPullSubscriptionLister(l.indexerFor(&pubsubv1alpha1.PullSubscription{}))
}

func (l *Listers) GetTopicLister() pubsublisters.TopicLister {
	return pubsublisters.NewTopicLister(l.indexerFor(&pubsubv1alpha1.Topic{}))
}

func (l *Listers) GetChannelLister() messaginglisters.ChannelLister {
	return messaginglisters.NewChannelLister(l.indexerFor(&MessagingV1alpha1.Channel{}))
}

func (l *Listers) GetJobLister() batchv1listers.JobLister {
	return batchv1listers.NewJobLister(l.indexerFor(&batchv1.Job{}))
}

func (l *Listers) GetCloudAuditLogsSourceLister() eventslisters.CloudAuditLogsSourceLister {
	return eventslisters.NewCloudAuditLogsSourceLister(l.indexerFor(&EventsV1alpha1.CloudAuditLogsSource{}))
}

func (l *Listers) GetCloudStorageSourceLister() eventslisters.CloudStorageSourceLister {
	return eventslisters.NewCloudStorageSourceLister(l.indexerFor(&EventsV1alpha1.CloudStorageSource{}))
}

func (l *Listers) GetCloudSchedulerSourceLister() eventslisters.CloudSchedulerSourceLister {
	return eventslisters.NewCloudSchedulerSourceLister(l.indexerFor(&EventsV1alpha1.CloudSchedulerSource{}))
}

func (l *Listers) GetCloudPubSubSourceLister() eventslisters.CloudPubSubSourceLister {
	return eventslisters.NewCloudPubSubSourceLister(l.indexerFor(&EventsV1alpha1.CloudPubSubSource{}))
}

func (l *Listers) GetCloudBuildSourceLister() eventslisters.CloudBuildSourceLister {
	return eventslisters.NewCloudBuildSourceLister(l.indexerFor(&EventsV1alpha1.CloudBuildSource{}))
}

func (l *Listers) GetDeploymentLister() appsv1listers.DeploymentLister {
	return appsv1listers.NewDeploymentLister(l.indexerFor(&appsv1.Deployment{}))
}

func (l *Listers) GetK8sServiceLister() corev1listers.ServiceLister {
	return corev1listers.NewServiceLister(l.indexerFor(&corev1.Service{}))
}

func (l *Listers) GetV1ServiceLister() servingv1listers.ServiceLister {
	return servingv1listers.NewServiceLister(l.indexerFor(&servingv1.Service{}))
}

func (l *Listers) GetV1alpha1ServiceLister() servingv1alpha1listers.ServiceLister {
	return servingv1alpha1listers.NewServiceLister(l.indexerFor(&servingv1alpha1.Service{}))
}

func (l *Listers) GetV1beta1ServiceLister() servingv1beta1listers.ServiceLister {
	return servingv1beta1listers.NewServiceLister(l.indexerFor(&servingv1beta1.Service{}))
}

func (l *Listers) GetNamespaceLister() corev1listers.NamespaceLister {
	return corev1listers.NewNamespaceLister(l.indexerFor(&corev1.Namespace{}))
}

func (l *Listers) GetServiceAccountLister() corev1listers.ServiceAccountLister {
	return corev1listers.NewServiceAccountLister(l.indexerFor(&corev1.ServiceAccount{}))
}

func (l *Listers) GetRoleBindingLister() rbacv1listers.RoleBindingLister {
	return rbacv1listers.NewRoleBindingLister(l.indexerFor(&rbacv1.RoleBinding{}))
}

func (l *Listers) GetEndpointsLister() corev1listers.EndpointsLister {
	return corev1listers.NewEndpointsLister(l.indexerFor(&corev1.Endpoints{}))
}

func (l *Listers) GetConfigMapLister() corev1listers.ConfigMapLister {
	return corev1listers.NewConfigMapLister(l.indexerFor(&corev1.ConfigMap{}))
}

func (l *Listers) GetHTTPPolicyLister() policylisters.HTTPPolicyLister {
	return policylisters.NewHTTPPolicyLister(l.indexerFor(&policyv1alpha1.HTTPPolicy{}))
}

func (l *Listers) GetHTTPPolicyBindingLister() policylisters.HTTPPolicyBindingLister {
	return policylisters.NewHTTPPolicyBindingLister(l.indexerFor(&policyv1alpha1.HTTPPolicyBinding{}))
}

func (l *Listers) GetEventPolicyLister() policylisters.EventPolicyLister {
	return policylisters.NewEventPolicyLister(l.indexerFor(&policyv1alpha1.EventPolicy{}))
}

func (l *Listers) GetEventPolicyBindingLister() policylisters.EventPolicyBindingLister {
	return policylisters.NewEventPolicyBindingLister(l.indexerFor(&policyv1alpha1.EventPolicyBinding{}))
}

func (l *Listers) GetRequestAuthenticationLister() istiov1beta1listers.RequestAuthenticationLister {
	return istiov1beta1listers.NewRequestAuthenticationLister(l.indexerFor(&istiov1beta1.RequestAuthentication{}))
}

func (l *Listers) GetAuthorizationPolicyLister() istiov1beta1listers.AuthorizationPolicyLister {
	return istiov1beta1listers.NewAuthorizationPolicyLister(l.indexerFor(&istiov1beta1.AuthorizationPolicy{}))
}

func (l *Listers) GetBrokerLister() brokerlisters.BrokerLister {
	return brokerlisters.NewBrokerLister(l.indexerFor(&brokerv1beta1.Broker{}))
}

func (l *Listers) GetTriggerLister() brokerlisters.TriggerLister {
	return brokerlisters.NewTriggerLister(l.indexerFor(&brokerv1beta1.Trigger{}))
}
