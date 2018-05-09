/*
Copyright 2018 The Kubernetes Authors.

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

package source

import (
	"encoding/gob"
	"fmt"
	"net"
	"strings"

	"github.com/golang/glog"

	feddnsv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	fedclient "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset/typed/multiclusterdns/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// minDNSTTL is the minimum safe DNS TTL value to use (in seconds).  We use this as the TTL for all DNS records.
	minDNSTTL = 180
)

const (
	// RecordTypeA is a RecordType enum value
	RecordTypeA = "A"
	// RecordTypeCNAME is a RecordType enum value
	RecordTypeCNAME = "CNAME"
)

// Targets is a representation of a list of targets for an endpoint.
type Targets []string

// TTL is a structure defining the TTL of a DNS record
type TTL int64

// Labels store metadata related to the endpoint
// it is then stored in a persistent storage via serialization
type Labels map[string]string

// NewLabels returns empty Labels
func NewLabels() Labels {
	return map[string]string{}
}

// Endpoint is a high-level way of a connection between a service and an IP
type Endpoint struct {
	// The hostname of the DNS record
	DNSName string
	// The targets the DNS record points to
	Targets Targets
	// RecordType type of record, e.g. CNAME, A, TXT etc
	RecordType string
	// TTL for the record
	RecordTTL TTL
	// Labels stores labels defined for the Endpoint
	Labels Labels
}

// Abstracting away the internet
type NetWrapper interface {
	LookupHost(host string) (addrs []string, err error)
}

type NetWrapperDefaultImplementation struct{}

func (r *NetWrapperDefaultImplementation) LookupHost(host string) (addrs []string, err error) {
	return net.LookupHost(host)
}

var netWrapper = &NetWrapperDefaultImplementation{}

// federationDNSSource is an implementation of Source for federation MultiClusterDNSLb objects.
// It will construct the DNS names based on the availability of target endpoints for the service
// in zone, region and global levels.
type federationDNSSource struct {
	client     fedclient.MulticlusterdnsV1alpha1Interface
	namespace  string
	federation string
	dnsSuffix  string
	listener   net.Listener
}

// NewFederationSource creates a new federation source with the given config.
func NewFederationSource(client fedclient.MulticlusterdnsV1alpha1Interface, namespace, federation, dnsSuffix, port string) (*federationDNSSource, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		glog.Errorf("Failed to listen on port %s, err: %v", err)
		return nil, err
	}

	return &federationDNSSource{
		client:     client,
		namespace:  namespace,
		federation: federation,
		dnsSuffix:  dnsSuffix,
		listener:   ln,
	}, nil
}

// Run runs forever waiting for accepting client connection and handle client requests
func (ds *federationDNSSource) Run() {
	for {
		// Accept a new connection from client
		conn, err := ds.listener.Accept()
		if err != nil {
			continue
		}
		go ds.HandleRequest(conn)
	}
}

// HandleRequest handle the single request from the client
func (ds *federationDNSSource) HandleRequest(conn net.Conn) {
	enc := gob.NewEncoder(conn)

	endpoints, err := ds.Endpoints()
	if err != nil {
		glog.Errorf("Failed to list DNS endpoints: %v", err)
		return
	}

	glog.V(5).Infof("DNS Endpoints: %#v", endpoints)

	// Encode and send the endpoints to client
	enc.Encode(endpoints)
}

// Endpoints returns endpoint objects for each MultiClusterDNSLb object that should be processed.
func (ds *federationDNSSource) Endpoints() ([]*Endpoint, error) {
	dnsObjects, err := ds.client.MultiClusterDNSLbs(ds.namespace).List(v1.ListOptions{})
	if err != nil {
		glog.Errorf("Listing DNS objects failed: %v", err)
		return nil, err
	}

	endpoints := []*Endpoint{}

	for _, dnsObject := range dnsObjects.Items {
		eps, err := getNamesForDNSObject(&dnsObject, ds.federation, ds.dnsSuffix)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, eps...)
	}

	return endpoints, nil
}

func getNamesForDNSObject(dnsObject *feddnsv1a1.MultiClusterDNSLb, federation, dnsSuffix string) ([]*Endpoint, error) {
	endpoints := []*Endpoint{}

	commonPrefix := strings.Join([]string{dnsObject.Name, dnsObject.Namespace, federation, "svc"}, ".")
	for _, clusterDNS := range dnsObject.Status.DNS {
		zone := clusterDNS.Zone
		region := clusterDNS.Region

		dnsNames := []string{
			strings.Join([]string{commonPrefix, zone, region, dnsSuffix}, "."), // zone level
			strings.Join([]string{commonPrefix, region, dnsSuffix}, "."),       // region level, one up from zone level
			strings.Join([]string{commonPrefix, dnsSuffix}, "."),               // global level, one up from region level
			"", // nowhere to go up from global level
		}

		zoneTargets, regionTargets, globalTargets := getHealthyTargets(zone, region, dnsObject)
		targets := [][]string{zoneTargets, regionTargets, globalTargets}

		for i, target := range targets {
			endpoint, err := generateEndpoint(dnsNames[i], target, dnsNames[i+1])
			if err != nil {
				return nil, err
			}
			endpoints = append(endpoints, endpoint)
		}
	}

	return endpoints, nil
}

// getHealthyTargets returns the hostnames and/or IP addresses of healthy endpoints for the service, at a zone, region and global level (or an error)
func getHealthyTargets(zone, region string, dnsObject *feddnsv1a1.MultiClusterDNSLb) (zoneTargets, regionTargets, globalTargets Targets) {
	// If federated dnsObject is deleted, return empty endpoints, so that DNS records are removed
	if dnsObject.DeletionTimestamp != nil {
		return zoneTargets, regionTargets, globalTargets
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		if clusterDNS.Zone == zone {
			zoneTargets = append(zoneTargets, extractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
		}
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		if clusterDNS.Region == region {
			regionTargets = append(regionTargets, extractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
		}
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		globalTargets = append(globalTargets, extractLoadBalancerTargets(clusterDNS.LoadBalancer)...)
	}

	return zoneTargets, regionTargets, globalTargets
}

func generateEndpoint(name string, targets Targets, uplevelCname string) (ep *Endpoint, err error) {
	ep = &Endpoint{
		DNSName:   name,
		RecordTTL: minDNSTTL,
		Labels:    NewLabels(),
	}

	if len(targets) > 0 {
		targets, err = getResolvedTargets(targets, netWrapper)
		if err != nil {
			return nil, err
		}
		ep.Targets = targets
		ep.RecordType = RecordTypeA
	} else {
		ep.Targets = []string{uplevelCname}
		ep.RecordType = RecordTypeCNAME
	}

	return ep, nil
}

// getResolvedTargets performs DNS resolution on the provided slice of endpoints (which might be DNS names
// or IPv4 addresses) and returns a list of IPv4 addresses.  If any of the endpoints are neither valid IPv4
// addresses nor resolvable DNS names, non-nil error is also returned (possibly along with a partially
// complete list of resolved endpoints.
func getResolvedTargets(targets Targets, netWrapper NetWrapper) (Targets, error) {
	resolvedTargets := sets.String{}
	for _, target := range targets {
		if net.ParseIP(target) == nil {
			// It's not a valid IP address, so assume it's a DNS name, and try to resolve it,
			// replacing its DNS name with its IP addresses in expandedEndpoints
			// through an interface abstracting the internet
			ipAddrs, err := netWrapper.LookupHost(target)
			if err != nil {
				glog.Errorf("Failed to resolve %s, err: %v", target, err)
				return resolvedTargets.List(), err
			}
			for _, ip := range ipAddrs {
				resolvedTargets = resolvedTargets.Union(sets.NewString(ip))
			}
		} else {
			resolvedTargets = resolvedTargets.Union(sets.NewString(target))
		}
	}
	return resolvedTargets.List(), nil
}

func extractLoadBalancerTargets(lbStatus corev1.LoadBalancerStatus) Targets {
	var targets Targets

	for _, lb := range lbStatus.Ingress {
		if lb.IP != "" {
			targets = append(targets, lb.IP)
		}
		if lb.Hostname != "" {
			targets = append(targets, lb.Hostname)
		}
	}

	return targets
}
