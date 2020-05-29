/*
Copyright 2020 Google LLC

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

package lib

import (
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/eventing/test/lib/resources"

	eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	inteventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

func (c *Client) CreateUnstructuredObjOrFail(spec *unstructured.Unstructured) {
	c.T.Helper()
	gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
	_, err := c.Core.Dynamic.Resource(gvr).Namespace(c.Namespace).Create(spec, v1.CreateOptions{})
	if err != nil {
		c.T.Fatalf("Failed to create object %s %s/%s: %v", spec.GroupVersionKind().String(), c.Namespace, spec.GetName(), err)
	}
	c.T.Logf("Created object: %s %s/%s", spec.GroupVersionKind().String(), c.Namespace, spec.GetName())
	c.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, c.Namespace, spec.GetName())
}

func (c *Client) CreateChannelOrFail(channel *messagingv1alpha1.Channel) {
	c.T.Helper()
	channels := c.KnativeGCP.MessagingV1alpha1().Channels(c.Namespace)
	_, err := channels.Create(channel)
	if err != nil {
		c.T.Fatalf("Failed to create channel %s/%s: %v", c.Namespace, channel.Name, err)
	}
	c.T.Logf("Created channel: %s/%s", c.Namespace, channel.Name)
	c.Tracker.AddObj(channel)
}

func (c *Client) CreateAuditLogsOrFail(auditlogs *eventsv1alpha1.CloudAuditLogsSource) {
	c.T.Helper()
	auditlogses := c.KnativeGCP.EventsV1alpha1().CloudAuditLogsSources(c.Namespace)
	_, err := auditlogses.Create(auditlogs)
	if err != nil {
		c.T.Fatalf("Failed to create auditlogs %s/%s: %v", c.Namespace, auditlogs.Name, err)
	}
	c.T.Logf("Created auditlogs: %s/%s", c.Namespace, auditlogs.Name)
	c.Tracker.AddObj(auditlogs)
}

func (c *Client) CreatePubSubOrFail(pubsub *eventsv1alpha1.CloudPubSubSource) {
	c.T.Helper()
	pubsubs := c.KnativeGCP.EventsV1alpha1().CloudPubSubSources(c.Namespace)
	_, err := pubsubs.Create(pubsub)
	if err != nil {
		c.T.Fatalf("Failed to create pubsub %s/%s: %v", c.Namespace, pubsub.Name, err)
	}
	c.T.Logf("Created pubsub: %s/%s", c.Namespace, pubsub.Name)
	c.Tracker.AddObj(pubsub)
}

func (c *Client) CreateStorageOrFail(storage *eventsv1alpha1.CloudStorageSource) {
	c.T.Helper()
	storages := c.KnativeGCP.EventsV1alpha1().CloudStorageSources(c.Namespace)
	_, err := storages.Create(storage)
	if err != nil {
		c.T.Fatalf("Failed to create storage %s/%s: %v", c.Namespace, storage.Name, err)
	}
	c.T.Logf("Created storage: %s/%s", c.Namespace, storage.Name)
	c.Tracker.AddObj(storage)
}

func (c *Client) CreatePullSubscriptionOrFail(pullsubscription *inteventsv1alpha1.PullSubscription) {
	c.T.Helper()
	pullsubscriptions := c.KnativeGCP.InternalV1alpha1().PullSubscriptions(c.Namespace)
	_, err := pullsubscriptions.Create(pullsubscription)
	if err != nil {
		c.T.Fatalf("Failed to create pullsubscription %s/%s: %v", c.Namespace, pullsubscription.Name, err)
	}
	c.T.Logf("Created pullsubscription: %s/%s", c.Namespace, pullsubscription.Name)
	c.Tracker.AddObj(pullsubscription)
}

func (c *Client) CreateSchedulerOrFail(scheduler *eventsv1alpha1.CloudSchedulerSource) {
	c.T.Helper()
	schedulers := c.KnativeGCP.EventsV1alpha1().CloudSchedulerSources(c.Namespace)
	_, err := schedulers.Create(scheduler)
	if err != nil {
		c.T.Fatalf("Failed to create scheduler %s/%s: %v", c.Namespace, scheduler.Name, err)
	}
	c.T.Logf("Created scheduler: %s/%s", c.Namespace, scheduler.Name)
	c.Tracker.AddObj(scheduler)
}

// WithServiceForJob returns an option that creates a Service binded with the given job.
func WithServiceForJob(name string) func(*batchv1.Job, *Client) error {
	return func(job *batchv1.Job, client *Client) error {
		svc := resources.ServiceDefaultHTTP(name, job.Spec.Template.Labels)

		svcs := client.Core.Kube.Kube.CoreV1().Services(job.Namespace)
		if _, err := svcs.Create(svc); err != nil {
			return err
		}
		client.T.Logf("Created service: %s/%s", job.Namespace, svc.Name)
		client.Tracker.Add("", "v1", "services", job.Namespace, name)
		return nil
	}
}

func (c *Client) CreateJobOrFail(job *batchv1.Job, options ...func(*batchv1.Job, *Client) error) {
	c.T.Helper()
	// set namespace for the job in case it's empty
	if job.Namespace == "" {
		job.Namespace = c.Namespace
	}
	// apply options on the job before creation
	for _, option := range options {
		if err := option(job, c); err != nil {
			c.T.Fatalf("Failed to configure job %s/%s: %v", job.Namespace, job.Name, err)
		}
	}

	jobs := c.Core.Kube.Kube.BatchV1().Jobs(job.Namespace)
	if _, err := jobs.Create(job); err != nil {
		c.T.Fatalf("Failed to create job %s/%s: %v", job.Namespace, job.Name, err)
	}
	c.T.Logf("Created job %s/%s", job.Namespace, job.Name)
	c.Tracker.Add("batch", "v1", "jobs", job.Namespace, job.Name)
}
