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
	eventsv1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) GetPubSubOrFail(name string) *eventsv1.CloudPubSubSource {
	c.T.Helper()
	pubsubs := c.KnativeGCP.EventsV1().CloudPubSubSources(c.Namespace)
	existing, err := pubsubs.Get(name, metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Failed to get pubsub %s/%s: %v", c.Namespace, name, err)
	}
	return existing
}

func (c *Client) GetBuildOrFail(name string) *eventsv1.CloudBuildSource {
	c.T.Helper()
	builds := c.KnativeGCP.EventsV1().CloudBuildSources(c.Namespace)
	existing, err := builds.Get(name, metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Failed to get build %s/%s: %v", c.Namespace, name, err)
	}
	return existing
}

func (c *Client) GetSchedulerOrFail(name string) *eventsv1.CloudSchedulerSource {
	c.T.Helper()
	schedulers := c.KnativeGCP.EventsV1().CloudSchedulerSources(c.Namespace)
	existing, err := schedulers.Get(name, metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Failed to get scheduler %s/%s: %v", c.Namespace, name, err)
	}
	return existing
}

func (c *Client) GetStorageOrFail(name string) *eventsv1.CloudStorageSource {
	c.T.Helper()
	storages := c.KnativeGCP.EventsV1().CloudStorageSources(c.Namespace)
	existing, err := storages.Get(name, metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Failed to get storages %s/%s: %v", c.Namespace, name, err)
	}
	return existing
}

func (c *Client) GetAuditLogsOrFail(name string) *eventsv1.CloudAuditLogsSource {
	c.T.Helper()
	auditLogs := c.KnativeGCP.EventsV1().CloudAuditLogsSources(c.Namespace)
	existing, err := auditLogs.Get(name, metav1.GetOptions{})
	if err != nil {
		c.T.Fatalf("Failed to get auditlogs %s/%s: %v", c.Namespace, name, err)
	}
	return existing
}
