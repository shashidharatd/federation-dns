/*
Copyright 2018 The Federation v2 Authors.

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
package fake

import (
	v1alpha1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMultiClusterDNSLbs implements MultiClusterDNSLbInterface
type FakeMultiClusterDNSLbs struct {
	Fake *FakeMulticlusterdnsV1alpha1
	ns   string
}

var multiclusterdnslbsResource = schema.GroupVersionResource{Group: "multiclusterdns.k8s.io", Version: "v1alpha1", Resource: "multiclusterdnslbs"}

var multiclusterdnslbsKind = schema.GroupVersionKind{Group: "multiclusterdns.k8s.io", Version: "v1alpha1", Kind: "MultiClusterDNSLb"}

// Get takes name of the multiClusterDNSLb, and returns the corresponding multiClusterDNSLb object, and an error if there is any.
func (c *FakeMultiClusterDNSLbs) Get(name string, options v1.GetOptions) (result *v1alpha1.MultiClusterDNSLb, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(multiclusterdnslbsResource, c.ns, name), &v1alpha1.MultiClusterDNSLb{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MultiClusterDNSLb), err
}

// List takes label and field selectors, and returns the list of MultiClusterDNSLbs that match those selectors.
func (c *FakeMultiClusterDNSLbs) List(opts v1.ListOptions) (result *v1alpha1.MultiClusterDNSLbList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(multiclusterdnslbsResource, multiclusterdnslbsKind, c.ns, opts), &v1alpha1.MultiClusterDNSLbList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MultiClusterDNSLbList{}
	for _, item := range obj.(*v1alpha1.MultiClusterDNSLbList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested multiClusterDNSLbs.
func (c *FakeMultiClusterDNSLbs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(multiclusterdnslbsResource, c.ns, opts))

}

// Create takes the representation of a multiClusterDNSLb and creates it.  Returns the server's representation of the multiClusterDNSLb, and an error, if there is any.
func (c *FakeMultiClusterDNSLbs) Create(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (result *v1alpha1.MultiClusterDNSLb, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(multiclusterdnslbsResource, c.ns, multiClusterDNSLb), &v1alpha1.MultiClusterDNSLb{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MultiClusterDNSLb), err
}

// Update takes the representation of a multiClusterDNSLb and updates it. Returns the server's representation of the multiClusterDNSLb, and an error, if there is any.
func (c *FakeMultiClusterDNSLbs) Update(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (result *v1alpha1.MultiClusterDNSLb, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(multiclusterdnslbsResource, c.ns, multiClusterDNSLb), &v1alpha1.MultiClusterDNSLb{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MultiClusterDNSLb), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMultiClusterDNSLbs) UpdateStatus(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (*v1alpha1.MultiClusterDNSLb, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(multiclusterdnslbsResource, "status", c.ns, multiClusterDNSLb), &v1alpha1.MultiClusterDNSLb{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MultiClusterDNSLb), err
}

// Delete takes name of the multiClusterDNSLb and deletes it. Returns an error if one occurs.
func (c *FakeMultiClusterDNSLbs) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(multiclusterdnslbsResource, c.ns, name), &v1alpha1.MultiClusterDNSLb{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMultiClusterDNSLbs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(multiclusterdnslbsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.MultiClusterDNSLbList{})
	return err
}

// Patch applies the patch and returns the patched multiClusterDNSLb.
func (c *FakeMultiClusterDNSLbs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MultiClusterDNSLb, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(multiclusterdnslbsResource, c.ns, name, data, subresources...), &v1alpha1.MultiClusterDNSLb{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MultiClusterDNSLb), err
}
