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
package v1alpha1

import (
	v1alpha1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	scheme "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// MultiClusterDNSLbsGetter has a method to return a MultiClusterDNSLbInterface.
// A group's client should implement this interface.
type MultiClusterDNSLbsGetter interface {
	MultiClusterDNSLbs(namespace string) MultiClusterDNSLbInterface
}

// MultiClusterDNSLbInterface has methods to work with MultiClusterDNSLb resources.
type MultiClusterDNSLbInterface interface {
	Create(*v1alpha1.MultiClusterDNSLb) (*v1alpha1.MultiClusterDNSLb, error)
	Update(*v1alpha1.MultiClusterDNSLb) (*v1alpha1.MultiClusterDNSLb, error)
	UpdateStatus(*v1alpha1.MultiClusterDNSLb) (*v1alpha1.MultiClusterDNSLb, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.MultiClusterDNSLb, error)
	List(opts v1.ListOptions) (*v1alpha1.MultiClusterDNSLbList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MultiClusterDNSLb, err error)
	MultiClusterDNSLbExpansion
}

// multiClusterDNSLbs implements MultiClusterDNSLbInterface
type multiClusterDNSLbs struct {
	client rest.Interface
	ns     string
}

// newMultiClusterDNSLbs returns a MultiClusterDNSLbs
func newMultiClusterDNSLbs(c *MulticlusterdnsV1alpha1Client, namespace string) *multiClusterDNSLbs {
	return &multiClusterDNSLbs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the multiClusterDNSLb, and returns the corresponding multiClusterDNSLb object, and an error if there is any.
func (c *multiClusterDNSLbs) Get(name string, options v1.GetOptions) (result *v1alpha1.MultiClusterDNSLb, err error) {
	result = &v1alpha1.MultiClusterDNSLb{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of MultiClusterDNSLbs that match those selectors.
func (c *multiClusterDNSLbs) List(opts v1.ListOptions) (result *v1alpha1.MultiClusterDNSLbList, err error) {
	result = &v1alpha1.MultiClusterDNSLbList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested multiClusterDNSLbs.
func (c *multiClusterDNSLbs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a multiClusterDNSLb and creates it.  Returns the server's representation of the multiClusterDNSLb, and an error, if there is any.
func (c *multiClusterDNSLbs) Create(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (result *v1alpha1.MultiClusterDNSLb, err error) {
	result = &v1alpha1.MultiClusterDNSLb{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		Body(multiClusterDNSLb).
		Do().
		Into(result)
	return
}

// Update takes the representation of a multiClusterDNSLb and updates it. Returns the server's representation of the multiClusterDNSLb, and an error, if there is any.
func (c *multiClusterDNSLbs) Update(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (result *v1alpha1.MultiClusterDNSLb, err error) {
	result = &v1alpha1.MultiClusterDNSLb{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		Name(multiClusterDNSLb.Name).
		Body(multiClusterDNSLb).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *multiClusterDNSLbs) UpdateStatus(multiClusterDNSLb *v1alpha1.MultiClusterDNSLb) (result *v1alpha1.MultiClusterDNSLb, err error) {
	result = &v1alpha1.MultiClusterDNSLb{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		Name(multiClusterDNSLb.Name).
		SubResource("status").
		Body(multiClusterDNSLb).
		Do().
		Into(result)
	return
}

// Delete takes name of the multiClusterDNSLb and deletes it. Returns an error if one occurs.
func (c *multiClusterDNSLbs) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *multiClusterDNSLbs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched multiClusterDNSLb.
func (c *multiClusterDNSLbs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.MultiClusterDNSLb, err error) {
	result = &v1alpha1.MultiClusterDNSLb{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("multiclusterdnslbs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
