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
	"github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset/scheme"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	rest "k8s.io/client-go/rest"
)

type MulticlusterdnsV1alpha1Interface interface {
	RESTClient() rest.Interface
	MultiClusterDNSLbsGetter
}

// MulticlusterdnsV1alpha1Client is used to interact with features provided by the multiclusterdns.k8s.io group.
type MulticlusterdnsV1alpha1Client struct {
	restClient rest.Interface
}

func (c *MulticlusterdnsV1alpha1Client) MultiClusterDNSLbs(namespace string) MultiClusterDNSLbInterface {
	return newMultiClusterDNSLbs(c, namespace)
}

// NewForConfig creates a new MulticlusterdnsV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*MulticlusterdnsV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &MulticlusterdnsV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new MulticlusterdnsV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *MulticlusterdnsV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new MulticlusterdnsV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *MulticlusterdnsV1alpha1Client {
	return &MulticlusterdnsV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *MulticlusterdnsV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
