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
	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

func (c *Client) CreateUnstructuredObjOrFail(spec *unstructured.Unstructured) {
	gvr, _ := meta.UnsafeGuessKindToResource(spec.GroupVersionKind())
	_, err := c.Core.Dynamic.Resource(gvr).Namespace(c.Namespace).Create(spec, v1.CreateOptions{})
	if err != nil {
		c.T.Fatalf("Failed to create object %q: %v", spec.GetName(), err)
	}
	c.Tracker.Add(gvr.Group, gvr.Version, gvr.Resource, c.Namespace, spec.GetName())
}

func (c *Client) CreateChannelOrFail(channel *messagingv1alpha1.Channel) {
	channels := c.KnativeGCP.MessagingV1alpha1().Channels(c.Namespace)
	_, err := channels.Create(channel)
	if err != nil {
		c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
	}
	c.Tracker.AddObj(channel)
}

func (c *Client) CreateAuditLogsOrFail(auditlogs *eventsv1alpha1.AuditLogsSource) {
	auditlogses := c.KnativeGCP.EventsV1alpha1().AuditLogsSources(c.Namespace)
	_, err := auditlogses.Create(auditlogs)
	if err != nil {
		c.T.Fatalf("Failed to create auditlogs %q: %v", auditlogs.Name, err)
	}
	c.Tracker.AddObj(auditlogs)
}

func (c *Client) CreatePubSubOrFail(pubsub *eventsv1alpha1.PubSub) {
	pubsubs := c.KnativeGCP.EventsV1alpha1().PubSubs(c.Namespace)
	_, err := pubsubs.Create(pubsub)
	if err != nil {
		c.T.Fatalf("Failed to create pubsub %q: %v", pubsub.Name, err)
	}
	c.Tracker.AddObj(pubsub)
}

func (c *Client) CreateStorageOrFail(storage *eventsv1alpha1.Storage) {
	storages := c.KnativeGCP.EventsV1alpha1().Storages(c.Namespace)
	_, err := storages.Create(storage)
	if err != nil {
		c.T.Fatalf("Failed to create storage %q: %v", storage.Name, err)
	}
	c.Tracker.AddObj(storage)
}

func (c *Client) CreatePullSubscriptionOrFail(pullsubscription *pubsubv1alpha1.PullSubscription) {
	pullsubscriptions := c.KnativeGCP.PubsubV1alpha1().PullSubscriptions(c.Namespace)
	_, err := pullsubscriptions.Create(pullsubscription)
	if err != nil {
		c.T.Fatalf("Failed to create pullsubscription %q: %v", pullsubscription.Name, err)
	}
	c.Tracker.AddObj(pullsubscription)
}

func (c *Client) CreateSchedulerOrFail(scheduler *eventsv1alpha1.Scheduler) {
	schedulers := c.KnativeGCP.EventsV1alpha1().Schedulers(c.Namespace)
	_, err := schedulers.Create(scheduler)
	if err != nil {
		c.T.Fatalf("Failed to create schedulers %q: %v", scheduler.Name, err)
	}
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
		client.Tracker.Add("", "v1", "services", job.Namespace, name)
		return nil
	}
}

func (c *Client) CreateJobOrFail(job *batchv1.Job, options ...func(*batchv1.Job, *Client) error) {
	// set namespace for the job in case it's empty
	if job.Namespace == "" {
		job.Namespace = c.Namespace
	}
	// apply options on the job before creation
	for _, option := range options {
		if err := option(job, c); err != nil {
			c.T.Fatalf("Failed to configure job %q: %v", job.Name, err)
		}
	}

	jobs := c.Core.Kube.Kube.BatchV1().Jobs(job.Namespace)
	if _, err := jobs.Create(job); err != nil {
		c.T.Fatalf("Failed to create job %q: %v", job.Name, err)
	}
	c.Tracker.Add("batch", "v1", "jobs", job.Namespace, job.Name)
}
