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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	v1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	scheme "github.com/google/knative-gcp/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// CloudAuditLogsSourcesGetter has a method to return a CloudAuditLogsSourceInterface.
// A group's client should implement this interface.
type CloudAuditLogsSourcesGetter interface {
	CloudAuditLogsSources(namespace string) CloudAuditLogsSourceInterface
}

// CloudAuditLogsSourceInterface has methods to work with CloudAuditLogsSource resources.
type CloudAuditLogsSourceInterface interface {
	Create(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.CreateOptions) (*v1.CloudAuditLogsSource, error)
	Update(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.UpdateOptions) (*v1.CloudAuditLogsSource, error)
	UpdateStatus(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.UpdateOptions) (*v1.CloudAuditLogsSource, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.CloudAuditLogsSource, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.CloudAuditLogsSourceList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.CloudAuditLogsSource, err error)
	CloudAuditLogsSourceExpansion
}

// cloudAuditLogsSources implements CloudAuditLogsSourceInterface
type cloudAuditLogsSources struct {
	client rest.Interface
	ns     string
}

// newCloudAuditLogsSources returns a CloudAuditLogsSources
func newCloudAuditLogsSources(c *EventsV1Client, namespace string) *cloudAuditLogsSources {
	return &cloudAuditLogsSources{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the cloudAuditLogsSource, and returns the corresponding cloudAuditLogsSource object, and an error if there is any.
func (c *cloudAuditLogsSources) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.CloudAuditLogsSource, err error) {
	result = &v1.CloudAuditLogsSource{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of CloudAuditLogsSources that match those selectors.
func (c *cloudAuditLogsSources) List(ctx context.Context, opts metav1.ListOptions) (result *v1.CloudAuditLogsSourceList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.CloudAuditLogsSourceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested cloudAuditLogsSources.
func (c *cloudAuditLogsSources) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a cloudAuditLogsSource and creates it.  Returns the server's representation of the cloudAuditLogsSource, and an error, if there is any.
func (c *cloudAuditLogsSources) Create(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.CreateOptions) (result *v1.CloudAuditLogsSource, err error) {
	result = &v1.CloudAuditLogsSource{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cloudAuditLogsSource).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a cloudAuditLogsSource and updates it. Returns the server's representation of the cloudAuditLogsSource, and an error, if there is any.
func (c *cloudAuditLogsSources) Update(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.UpdateOptions) (result *v1.CloudAuditLogsSource, err error) {
	result = &v1.CloudAuditLogsSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		Name(cloudAuditLogsSource.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cloudAuditLogsSource).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *cloudAuditLogsSources) UpdateStatus(ctx context.Context, cloudAuditLogsSource *v1.CloudAuditLogsSource, opts metav1.UpdateOptions) (result *v1.CloudAuditLogsSource, err error) {
	result = &v1.CloudAuditLogsSource{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		Name(cloudAuditLogsSource.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(cloudAuditLogsSource).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the cloudAuditLogsSource and deletes it. Returns an error if one occurs.
func (c *cloudAuditLogsSources) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *cloudAuditLogsSources) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched cloudAuditLogsSource.
func (c *cloudAuditLogsSources) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.CloudAuditLogsSource, err error) {
	result = &v1.CloudAuditLogsSource{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("cloudauditlogssources").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
