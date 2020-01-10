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
    "knative.dev/eventing/test/base/resources"

    eventsv1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
    messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

func (c *Client) CreateChannelOrFail(channel *messagingv1alpha1.Channel) {
    channels := c.KnativeGCP.MessagingV1alpha1().Channels(c.Namespace)
    _, err := channels.Create(channel)
    if err != nil {
        c.T.Fatalf("Failed to create channel %q: %v", channel.Name, err)
    }
    c.Tracker.AddObj(channel)
}

func (c *Client) CreatePubSubOrFail(pubsub *eventsv1alpha1.PubSub) {
    pubsubs := c.KnativeGCP.EventsV1alpha1().PubSubs(c.Namespace)
    _, err := pubsubs.Create(pubsub)
    if err != nil {
        c.T.Fatalf("Failed to create pubsub %q: %v", pubsub.Name, err)
    }
    c.Tracker.AddObj(pubsub)
}

// WithService returns an option that creates a Service binded with the given job.
func WithService(name string) func(*batchv1.Job, *Client) error {
    return func(job *batchv1.Job, client *Client) error {
        namespace := job.Namespace
        svc := resources.ServiceDefaultHTTP(name, job.Spec.Template.Labels)

        svcs := client.Core.Kube.Kube.CoreV1().Services(namespace)
        if _, err := svcs.Create(svc); err != nil {
            return err
        }
        client.Tracker.Add("", "v1", "services", namespace, name)
        return nil
    }
}

func (c *Client) CreateJobOrFail(job *batchv1.Job, options ...func(*batchv1.Job, *Client) error) {
    // set namespace for the job in case it's empty
    namespace := c.Namespace
    job.Namespace = namespace
    // apply options on the job before creation
    for _, option := range options {
        if err := option(job, c); err != nil {
            c.T.Fatalf("Failed to configure job %q: %v", job.Name, err)
        }
    }

    jobs := c.Core.Kube.Kube.BatchV1().Jobs(c.Namespace)
    if _, err := jobs.Create(job); err != nil {
        c.T.Fatalf("Failed to create job %q: %v", job.Name, err)
    }
    c.Tracker.Add("batch", "v1", "jobs", namespace, job.Name)
}
