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

package dns

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	feddnsv1alpha1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset"
	fakefedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset/fake"

	"github.com/shashidharatd/federation-dns/pkg/controller/util/test"
	"github.com/shashidharatd/federation-dns/pkg/dnsprovider"
	"github.com/shashidharatd/federation-dns/pkg/dnsprovider/providers/google/clouddns" // Only for unit testing purposes.
)

const (
	dnsZone     = "example.com"
	federation  = "ufp"
	name        = "nginx"
	namespace   = "test"
	recordA     = "A"
	recordCNAME = "CNAME"
	DNSTTL      = "180"

	retryInterval = 100 * time.Millisecond

	OP_ADD    = "ADD"
	OP_UPDATE = "UPDATE"
	OP_DELETE = "DELETE"

	c1 = "c1"
	c2 = "c2"

	c1Region = "us"
	c2Region = "eu"

	c1Zone = "us1"
	c2Zone = "eu1"

	servicedns = "multiclusterservicednsrecords"

	lb1 = "10.20.30.1"
	lb2 = "10.20.30.2"
	lb3 = "10.20.30.3"
)

type step struct {
	operation string
	status    feddnsv1alpha1.MultiClusterServiceDNSRecordStatus
	expected  sets.String
}

type testDeployment struct {
	steps []step
}

type NetWrapperMock struct {
	result map[string][]string
}

func (mock *NetWrapperMock) LookupHost(host string) (addrs []string, err error) {

	// If nothing to return, return empty list
	if mock.result == nil || len(mock.result) == 0 {
		return make([]string, 0), fmt.Errorf("Mock error response")
	}

	return mock.result[host], nil
}

func (mock *NetWrapperMock) AddHost(host string, addrs []string) {
	// Initialise if null
	if mock.result == nil {
		mock.result = make(map[string][]string)
	}

	mock.result[host] = addrs
}

var instanceCounter uint64 = 0

func createTestServiceDeployments() map[string]testDeployment {
	globalDNSName := strings.Join([]string{name, namespace, federation, "svc", dnsZone}, ".")
	c1RegionDNSName := strings.Join([]string{name, namespace, federation, "svc", c1Region, dnsZone}, ".")
	c1ZoneDNSName := strings.Join([]string{name, namespace, federation, "svc", c1Zone, c1Region, dnsZone}, ".")
	c2RegionDNSName := strings.Join([]string{name, namespace, federation, "svc", c2Region, dnsZone}, ".")
	c2ZoneDNSName := strings.Join([]string{name, namespace, federation, "svc", c2Zone, c2Region, dnsZone}, ".")

	tests := map[string]testDeployment{
		"ObjectWithNoIngress": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}}},

		"ObjectWithSingleLBIngress": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb1}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordCNAME, DNSTTL, "[" + globalDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordCNAME, DNSTTL, "[" + c2RegionDNSName + "]"}, ":"),
			),
		}}},

		// Tests getResolvedEndpoints DNS lookup
		"ObjectWithAWSAndGCPMultipleLBIngress": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{Hostname: "a9.us-west-2.elb.amazonaws.com"}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb1 + " " + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
			),
		}}},

		"ObjectWithNoLBIngress": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, c1RegionDNSName, recordCNAME, DNSTTL, "[" + globalDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordCNAME, DNSTTL, "[" + c1RegionDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordCNAME, DNSTTL, "[" + globalDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordCNAME, DNSTTL, "[" + c2RegionDNSName + "]"}, ":"),
			),
		}}},

		"ObjectWithMultipleLBIngress": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb1}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb1 + " " + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
			),
		}}},

		"ObjectWithLBIngressAndServiceDeleted": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb1}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb1 + " " + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
			),
		}, {
			operation: OP_DELETE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb1}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				// TODO: Ideally we should expect that there are no DNS records
				strings.Join([]string{dnsZone, c1RegionDNSName, recordCNAME, DNSTTL, "[" + globalDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordCNAME, DNSTTL, "[" + c1RegionDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordCNAME, DNSTTL, "[" + globalDNSName + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordCNAME, DNSTTL, "[" + c2RegionDNSName + "]"}, ":"),
			),
		}}},

		"ObjectWithLBIngressAndLBIngressModifiedOvertime": {steps: []step{{
			operation: OP_ADD,
			expected:  sets.NewString(),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb1}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb1 + " " + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb1 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
			),
		}, {
			operation: OP_UPDATE,
			status: feddnsv1alpha1.MultiClusterServiceDNSRecordStatus{
				DNS: []feddnsv1alpha1.ClusterDNS{
					{
						Cluster: c1, Zone: c1Zone, Region: c1Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb3}}},
					},
					{
						Cluster: c2, Zone: c2Zone, Region: c2Region,
						LoadBalancer: v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb2}}},
					},
				}},
			expected: sets.NewString(
				strings.Join([]string{dnsZone, globalDNSName, recordA, DNSTTL, "[" + lb2 + " " + lb3 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1RegionDNSName, recordA, DNSTTL, "[" + lb3 + "]"}, ":"),
				strings.Join([]string{dnsZone, c1ZoneDNSName, recordA, DNSTTL, "[" + lb3 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2RegionDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
				strings.Join([]string{dnsZone, c2ZoneDNSName, recordA, DNSTTL, "[" + lb2 + "]"}, ":"),
			),
		}}},
	}

	return tests
}

func init() {
	dnsprovider.RegisterDnsProvider("fake-clouddns", func(config io.Reader) (dnsprovider.Interface, error) {
		return clouddns.NewFakeInterface([]string{dnsZone})
	})
}

func TestServiceDNSController(t *testing.T) {
	tests := createTestServiceDeployments()

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			fakeClient, dnsObjWatch := setupFakeInfraForFederation()

			// Fake out the internet
			netmock := &NetWrapperMock{}
			netmock.AddHost("a9.us-west-2.elb.amazonaws.com", []string{lb1})

			dc, err := NewDNSController(fakeClient, "fake-clouddns", "",
				federation, "", dnsZone, "")
			dc.netWrapper = netmock
			if err != nil {
				t.Errorf("error initializing federation dns controller: %v", err)
			}
			stop := make(chan struct{})
			t.Logf("Running federation dns Controller")
			go dc.Run(2, stop)

			dnsObj := NewDNSObject(name)
			key := types.NamespacedName{Namespace: dnsObj.Namespace, Name: dnsObj.Name}.String()
			for _, step := range test.steps {
				switch step.operation {
				case OP_ADD:
					dnsObjWatch.Add(dnsObj)
					require.NoError(t, WaitForObjectUpdate(t, dc.serviceDNSObjectStore, key, dnsObj, dnsObjectStatusCompare))
				case OP_UPDATE:
					dnsObj.Status = step.status
					dnsObjWatch.Modify(dnsObj)
					require.NoError(t, WaitForObjectUpdate(t, dc.serviceDNSObjectStore, key, dnsObj, dnsObjectStatusCompare))
				case OP_DELETE:
					dnsObj.ObjectMeta.Finalizers = append(dnsObj.ObjectMeta.Finalizers, metav1.FinalizerOrphanDependents)
					dnsObj.DeletionTimestamp = &metav1.Time{Time: time.Now()}
					dnsObjWatch.Delete(dnsObj)
					require.NoError(t, WaitForObjectDeletion(dc.serviceDNSObjectStore, key))
				}

				waitForDNSRecords(t, dc, step.expected)
			}
			close(stop)
		})
	}
}

// waitForDNSRecords waits for DNS records in fakedns to match expected DNS records
func waitForDNSRecords(t *testing.T, d *DNSController, expectedDNSRecords sets.String) {
	fakednsZones, ok := d.dns.Zones()
	if !ok {
		t.Error("Unable to fetch zones")
	}
	zones, err := fakednsZones.List()
	if err != nil {
		t.Errorf("error querying zones: %v", err)
	}

	// Dump every record to a testable-by-string-comparison form
	availableDNSRecords := sets.NewString()
	err = wait.PollImmediate(retryInterval, 5*time.Second, func() (bool, error) {
		for _, z := range zones {
			zoneName := z.Name()

			rrs, ok := z.ResourceRecordSets()
			if !ok {
				t.Errorf("cannot get rrs for zone %q", zoneName)
			}

			rrList, err := rrs.List()
			if err != nil {
				t.Errorf("error querying rr for zone %q: %v", zoneName, err)
			}
			for _, rr := range rrList {
				rrdatas := rr.Rrdatas()

				// Put in consistent (testable-by-string-comparison) order
				sort.Strings(rrdatas)
				availableDNSRecords.Insert(fmt.Sprintf("%s:%s:%s:%d:%s", zoneName, rr.Name(), rr.Type(), rr.Ttl(), rrdatas))
			}
		}

		if !availableDNSRecords.Equal(expectedDNSRecords) {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Errorf("Actual DNS records does not match expected. \n  Actual=%v \nExpected=%v", availableDNSRecords, expectedDNSRecords)
	}
}

func TestServiceDNSControllerInitParams(t *testing.T) {
	tests := map[string]struct {
		registeredProvider string
		useProvider        string
		registeredZones    []string
		useZone            string
		federationName     string
		serviceDNSSuffix   string
		zoneId             string
		expectError        bool
	}{
		"AllValidParams": {
			federationName: "ufp",
			expectError:    false,
		},
		"EmptyFederationName": {
			federationName: "",
			expectError:    true,
		},
		"NoneExistingDNSProvider": {
			federationName: "ufp",
			useProvider:    "non-existent",
			expectError:    true,
		},
		"MultipleRegisteredZonesWithDifferentNames": {
			federationName:  "ufp",
			registeredZones: []string{"abc.com", "xyz.com"},
			expectError:     false,
		},
		"MultipleRegisteredZonesWithSameNames": {
			federationName:  "ufp",
			registeredZones: []string{"abc.com", "abc.com"},
			expectError:     true,
		},

		"MultipleRegisteredZonesWithSameNamesUseZoneId": {
			federationName:  "ufp",
			registeredZones: []string{"abc.com", "abc.com"},
			useZone:         "abc.com",
			zoneId:          "1",
			expectError:     true, // TODO: "google-clouddns" does not support multiple managed zones with same names
		},

		"UseNonExistentZone": {
			federationName:  "ufp",
			registeredZones: []string{"abc.com", "xyz.com"},
			useZone:         "example.com",
			expectError:     false,
		},
		"WithServiceDNSSuffix": {
			federationName:   "ufp",
			serviceDNSSuffix: "federation.example.com",
			expectError:      false,
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			if test.registeredProvider == "" {
				test.registeredProvider = "fake-dns-" + testName + strconv.FormatUint(instanceCounter, 10)
				atomic.AddUint64(&instanceCounter, 1)
			}
			if test.useProvider == "" {
				test.useProvider = test.registeredProvider
			}
			if test.registeredZones == nil {
				test.registeredZones = append(test.registeredZones, "example.com")
			}
			if test.useZone == "" {
				test.useZone = test.registeredZones[0]
			}

			dnsprovider.RegisterDnsProvider(test.registeredProvider, func(config io.Reader) (dnsprovider.Interface, error) {
				return clouddns.NewFakeInterface(test.registeredZones)
			})

			_, err := NewDNSController(&fakefedclientset.Clientset{},
				test.useProvider,
				"",
				test.federationName,
				test.serviceDNSSuffix,
				test.useZone,
				test.zoneId)
			if err != nil {
				if !test.expectError {
					t.Errorf("expected to succeed but got error: %v", err)
				}
			} else {
				if test.expectError {
					t.Errorf("expected to return error but succeeded")
				}
			}
		})
	}
}

type compareFunc func(actual, desired runtime.Object) (match bool)

func dnsObjectStatusCompare(actualObj, desiredObj runtime.Object) bool {
	actual := actualObj.(*feddnsv1alpha1.MultiClusterServiceDNSRecord)
	desired := desiredObj.(*feddnsv1alpha1.MultiClusterServiceDNSRecord)
	if !reflect.DeepEqual(actual.Status, desired.Status) {
		return false
	}
	return true
}

// WaitForObjectUpdate waits for updates to DNS object to match the desired status.
func WaitForObjectUpdate(t *testing.T, store cache.Store, key string, desired runtime.Object, match compareFunc) error {
	var actual interface{}
	err := wait.PollImmediate(retryInterval, wait.ForeverTestTimeout, func() (exist bool, err error) {
		actual, exist, err = store.GetByKey(key)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if !exist {
			return false, nil
		}

		return match(actual.(runtime.Object), desired), nil
	})
	if err != nil {
		t.Logf("Desired object: %#v", desired)
		t.Logf("Actual object: %#v", actual.(runtime.Object))
	}
	return err
}

func NewDNSObject(name string) *feddnsv1alpha1.MultiClusterServiceDNSRecord {
	return &feddnsv1alpha1.MultiClusterServiceDNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// WaitForObjectDeletion waits for the object to be deleted.
func WaitForObjectDeletion(store cache.Store, key string) error {
	err := wait.PollImmediate(retryInterval, wait.ForeverTestTimeout, func() (bool, error) {
		_, found, _ := store.GetByKey(key)
		if !found {
			return true, nil
		}
		return false, nil
	})
	return err
}

func setupFakeInfraForFederation() (fedclientset.Interface, *testutil.WatcherDispatcher) {
	client := &fakefedclientset.Clientset{}
	testutil.RegisterFakeList(servicedns, &client.Fake,
		&feddnsv1alpha1.MultiClusterServiceDNSRecordList{Items: []feddnsv1alpha1.MultiClusterServiceDNSRecord{}})
	dnsWatch := testutil.RegisterFakeWatch(servicedns, &client.Fake)
	testutil.RegisterFakeOnCreate(servicedns, &client.Fake, dnsWatch)
	testutil.RegisterFakeOnUpdate(servicedns, &client.Fake, dnsWatch)
	testutil.RegisterFakeOnDelete(servicedns, &client.Fake, dnsWatch, serviceDNSObjectGetter)

	return client, dnsWatch
}

// serviceDNSObjectGetter gives dummy ServiceDNS objects to use with RegisterFakeOnDelete
func serviceDNSObjectGetter(name, namespace string) runtime.Object {
	serviceDNSObject := new(feddnsv1alpha1.MultiClusterServiceDNSRecord)
	serviceDNSObject.Name = name
	serviceDNSObject.Namespace = namespace
	return serviceDNSObject
}
