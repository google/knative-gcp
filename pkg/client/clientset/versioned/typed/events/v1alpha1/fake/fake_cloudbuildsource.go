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

package fake

import (
	"context"

	v1alpha1 "github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCloudBuildSources implements CloudBuildSourceInterface
type FakeCloudBuildSources struct {
	Fake *FakeEventsV1alpha1
	ns   string
}

var cloudbuildsourcesResource = schema.GroupVersionResource{Group: "events.cloud.google.com", Version: "v1alpha1", Resource: "cloudbuildsources"}

var cloudbuildsourcesKind = schema.GroupVersionKind{Group: "events.cloud.google.com", Version: "v1alpha1", Kind: "CloudBuildSource"}

// Get takes name of the cloudBuildSource, and returns the corresponding cloudBuildSource object, and an error if there is any.
func (c *FakeCloudBuildSources) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.CloudBuildSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(cloudbuildsourcesResource, c.ns, name), &v1alpha1.CloudBuildSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CloudBuildSource), err
}

// List takes label and field selectors, and returns the list of CloudBuildSources that match those selectors.
func (c *FakeCloudBuildSources) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.CloudBuildSourceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(cloudbuildsourcesResource, cloudbuildsourcesKind, c.ns, opts), &v1alpha1.CloudBuildSourceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CloudBuildSourceList{ListMeta: obj.(*v1alpha1.CloudBuildSourceList).ListMeta}
	for _, item := range obj.(*v1alpha1.CloudBuildSourceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested cloudBuildSources.
func (c *FakeCloudBuildSources) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(cloudbuildsourcesResource, c.ns, opts))

}

// Create takes the representation of a cloudBuildSource and creates it.  Returns the server's representation of the cloudBuildSource, and an error, if there is any.
func (c *FakeCloudBuildSources) Create(ctx context.Context, cloudBuildSource *v1alpha1.CloudBuildSource, opts v1.CreateOptions) (result *v1alpha1.CloudBuildSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(cloudbuildsourcesResource, c.ns, cloudBuildSource), &v1alpha1.CloudBuildSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CloudBuildSource), err
}

// Update takes the representation of a cloudBuildSource and updates it. Returns the server's representation of the cloudBuildSource, and an error, if there is any.
func (c *FakeCloudBuildSources) Update(ctx context.Context, cloudBuildSource *v1alpha1.CloudBuildSource, opts v1.UpdateOptions) (result *v1alpha1.CloudBuildSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(cloudbuildsourcesResource, c.ns, cloudBuildSource), &v1alpha1.CloudBuildSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CloudBuildSource), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeCloudBuildSources) UpdateStatus(ctx context.Context, cloudBuildSource *v1alpha1.CloudBuildSource, opts v1.UpdateOptions) (*v1alpha1.CloudBuildSource, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(cloudbuildsourcesResource, "status", c.ns, cloudBuildSource), &v1alpha1.CloudBuildSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CloudBuildSource), err
}

// Delete takes name of the cloudBuildSource and deletes it. Returns an error if one occurs.
func (c *FakeCloudBuildSources) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(cloudbuildsourcesResource, c.ns, name), &v1alpha1.CloudBuildSource{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCloudBuildSources) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(cloudbuildsourcesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.CloudBuildSourceList{})
	return err
}

// Patch applies the patch and returns the patched cloudBuildSource.
func (c *FakeCloudBuildSources) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.CloudBuildSource, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(cloudbuildsourcesResource, c.ns, name, pt, data, subresources...), &v1alpha1.CloudBuildSource{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CloudBuildSource), err
}
