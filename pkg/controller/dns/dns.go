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
	"net"
	"strings"
	"time"

	"github.com/golang/glog"

	feddnsv1a1 "github.com/kubernetes-sigs/federation-v2/pkg/apis/multiclusterdns/v1alpha1"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset"
	"github.com/kubernetes-sigs/federation-v2/pkg/controller/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/client-go/util/workqueue"

	"github.com/shashidharatd/federation-dns/pkg/dnsprovider"
	"github.com/shashidharatd/federation-dns/pkg/dnsprovider/rrstype"
)

const (
	// minDNSTTL is the minimum safe DNS TTL value to use (in seconds).  We use this as the TTL for all DNS records.
	minDNSTTL = 180
)

// Abstracting away the internet
type NetWrapper interface {
	LookupHost(host string) (addrs []string, err error)
}

type NetWrapperDefaultImplementation struct{}

func (r *NetWrapperDefaultImplementation) LookupHost(host string) (addrs []string, err error) {
	return net.LookupHost(host)
}

type DNSController struct {
	// Client to federation api server
	fedClient      fedclientset.Interface
	dns            dnsprovider.Interface
	federationName string
	// dnsSuffix is the DNS suffix we use when publishing DNS names
	dnsSuffix string
	// zoneName and zoneID are used to identify the zone in which to put records
	zoneName string
	zoneID   string
	dnsZones dnsprovider.Zones
	// each federation should be configured with a single zone (e.g. "mycompany.com")
	dnsZone dnsprovider.Zone
	// Informer Store for DNS objects
	dnsObjectStore cache.Store
	// Informer controller for DNS objects
	dnsObjectController cache.Controller
	workQueue           workqueue.Interface
	deliverer           *util.DelayingDeliverer
	backoff             *flowcontrol.Backoff
	// Wraps DNS resolving so it can be mocked.
	netWrapper NetWrapper
}

// NewDNSController returns a new dns controller to manage DNS records
func NewDNSController(client fedclientset.Interface, dnsProvider, dnsProviderConfig, federationName,
	dnsSuffix, zoneName, zoneID string) (*DNSController, error) {

	dns, err := dnsprovider.InitDnsProvider(dnsProvider, dnsProviderConfig)
	if err != nil {
		runtime.HandleError(fmt.Errorf("DNS provider could not be initialized: %v", err))
		return nil, err
	}

	d := &DNSController{
		fedClient:      client,
		dns:            dns,
		federationName: federationName,
		dnsSuffix:      dnsSuffix,
		zoneName:       zoneName,
		zoneID:         zoneID,
		workQueue:      workqueue.New(),
		deliverer:      util.NewDelayingDeliverer(),
		backoff:        flowcontrol.NewBackOff(5*time.Second, time.Minute),
		netWrapper:     &NetWrapperDefaultImplementation{},
	}
	if err := d.validateConfig(); err != nil {
		runtime.HandleError(fmt.Errorf("Invalid configuration passed to DNS provider: %v", err))
		return nil, err
	}
	if err := d.retrieveOrCreateDNSZone(); err != nil {
		runtime.HandleError(fmt.Errorf("Failed to retrieve DNS zone: %v", err))
		return nil, err
	}

	// Start informer in federated API servers on DNS objects
	d.dnsObjectStore, d.dnsObjectController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (pkgruntime.Object, error) {
				return client.MulticlusterdnsV1alpha1().MultiClusterDNSLbs(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.MulticlusterdnsV1alpha1().MultiClusterDNSLbs(metav1.NamespaceAll).Watch(options)
			},
		},
		&feddnsv1a1.MultiClusterDNSLb{},
		util.NoResyncPeriod,
		util.NewTriggerOnAllChanges(func(obj pkgruntime.Object) { d.workQueue.Add(obj) }),
	)

	return d, nil
}

func (s *DNSController) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer s.workQueue.ShutDown()

	glog.Infof("Starting federation dns controller")

	s.deliverer.StartWithHandler(func(item *util.DelayingDelivererItem) {
		s.workQueue.Add(item.Value.(*feddnsv1a1.MultiClusterDNSLb))
	})
	defer s.deliverer.Stop()

	util.StartBackoffGC(s.backoff, stopCh)
	go s.dnsObjectController.Run(stopCh)

	for i := 0; i < workers; i++ {
		// Control loop running every second ensuring DNS records are in sync.
		go wait.Until(s.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Stopping federation dns controller")
}

// Adds backoff to delay if this delivery is related to some failure. Resets backoff if there was no failure.
func (s *DNSController) deliver(dnsObject *feddnsv1a1.MultiClusterDNSLb, delay time.Duration, failed bool) {
	key := fmt.Sprintf("%s/%s", dnsObject.Namespace, dnsObject.Name)
	if failed {
		s.backoff.Next(key, time.Now())
		delay = delay + s.backoff.Get(key)
	} else {
		s.backoff.Reset(key)
	}
	s.deliverer.DeliverAfter(key, dnsObject, delay)
}

func (s *DNSController) workerFunction() bool {
	item, quit := s.workQueue.Get()
	if quit {
		return true
	}
	defer s.workQueue.Done(item)

	dnsObject := item.(*feddnsv1a1.MultiClusterDNSLb)

	var err error
	for _, clusterDNS := range dnsObject.Status.DNS {
		err = s.ensureDNSRecords(clusterDNS.Cluster, clusterDNS.Zone, clusterDNS.Region, dnsObject)
		if err != nil {
			runtime.HandleError(fmt.Errorf("Error when ensuring DNS records for dnsObject %s/%s: %v", dnsObject.Namespace, dnsObject.Name, err))
			s.deliver(dnsObject, 0, true)
		}
	}
	return false
}

func (s *DNSController) worker() {
	for {
		if quit := s.workerFunction(); quit {
			glog.Infof("dns controller worker queue shutting down")
			return
		}
	}
}

func (s *DNSController) validateConfig() error {
	if s.federationName == "" {
		return fmt.Errorf("DNSController should not be run without federationName")
	}
	if s.zoneName == "" && s.zoneID == "" {
		return fmt.Errorf("DNSController must be run with either zone-name or zone-id")
	}
	if s.dnsSuffix == "" {
		if s.zoneName == "" {
			return fmt.Errorf("DNSController must be run with zoneName, if dns-suffix is not set")
		}
		s.dnsSuffix = s.zoneName
	}
	if s.dns == nil {
		return fmt.Errorf("DNSController should not be run without a dnsprovider")
	}
	zones, ok := s.dns.Zones()
	if !ok {
		return fmt.Errorf("the dns provider does not support zone enumeration, which is required for creating dns records")
	}
	s.dnsZones = zones
	return nil
}

func (s *DNSController) retrieveOrCreateDNSZone() error {
	matchingZones, err := getDNSZones(s.zoneName, s.zoneID, s.dnsZones)
	if err != nil {
		return fmt.Errorf("error querying for DNS zones: %v", err)
	}
	switch len(matchingZones) {
	case 0: // No matching zones for s.zoneName, so create one
		if s.zoneName == "" {
			return fmt.Errorf("DNSController must be run with zoneName to create zone automatically")
		}
		glog.Infof("DNS zone %q not found.  Creating DNS zone %q.", s.zoneName, s.zoneName)
		managedZone, err := s.dnsZones.New(s.zoneName)
		if err != nil {
			return err
		}
		zone, err := s.dnsZones.Add(managedZone)
		if err != nil {
			return err
		}
		glog.Infof("DNS zone %q successfully created.  Note that DNS resolution will not work until you have registered this name with "+
			"a DNS registrar and they have changed the authoritative name servers for your domain to point to your DNS provider", zone.Name())
	case 1: // s.zoneName matches exactly one DNS zone
		s.dnsZone = matchingZones[0]
	default: // s.zoneName matches more than one DNS zone
		return fmt.Errorf("Multiple matching DNS zones found for %q; please specify zoneID", s.zoneName)
	}
	return nil
}

func getTarget(ingress corev1.LoadBalancerIngress) string {
	var target string
	// We should get either an IP address or a hostname - use whichever one we get
	if ingress.IP != "" {
		target = ingress.IP
	} else if ingress.Hostname != "" {
		target = ingress.Hostname
	}
	return target
}

// getHealthyEndpoints returns the hostnames and/or IP addresses of healthy endpoints, at a zone, region and global level (or an error)
func (s *DNSController) getHealthyEndpoints(cluster, zone, region string, dnsObject *feddnsv1a1.MultiClusterDNSLb) (zoneEndpoints, regionEndpoints, globalEndpoints []string) {
	// If federated dnsObject is deleted, return empty endpoints, so that DNS records are removed
	if dnsObject.DeletionTimestamp != nil {
		return zoneEndpoints, regionEndpoints, globalEndpoints
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		if clusterDNS.Zone == zone {
			for _, ingress := range clusterDNS.LoadBalancer.Ingress {
				zoneEndpoints = append(zoneEndpoints, getTarget(ingress))
			}
		}
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		if clusterDNS.Region == region {
			for _, ingress := range clusterDNS.LoadBalancer.Ingress {
				regionEndpoints = append(regionEndpoints, getTarget(ingress))
			}
		}
	}

	for _, clusterDNS := range dnsObject.Status.DNS {
		for _, ingress := range clusterDNS.LoadBalancer.Ingress {
			globalEndpoints = append(globalEndpoints, getTarget(ingress))
		}
	}

	return zoneEndpoints, regionEndpoints, globalEndpoints
}

// getDNSZones returns the DNS zones matching dnsZoneName and dnsZoneID (if specified)
func getDNSZones(dnsZoneName string, dnsZoneID string, dnsZonesInterface dnsprovider.Zones) ([]dnsprovider.Zone, error) {
	// TODO: We need query-by-name and query-by-id functions
	dnsZones, err := dnsZonesInterface.List()
	if err != nil {
		return nil, err
	}

	var matches []dnsprovider.Zone
	findName := strings.TrimSuffix(dnsZoneName, ".")
	for _, dnsZone := range dnsZones {
		if dnsZoneID != "" {
			if dnsZoneID != dnsZone.ID() {
				continue
			}
		}
		if findName != "" {
			if strings.TrimSuffix(dnsZone.Name(), ".") != findName {
				continue
			}
		}
		matches = append(matches, dnsZone)
	}

	return matches, nil
}

func findRrset(list []dnsprovider.ResourceRecordSet, rrset dnsprovider.ResourceRecordSet) dnsprovider.ResourceRecordSet {
	for i, elem := range list {
		if dnsprovider.ResourceRecordSetsEquivalent(rrset, elem) {
			return list[i]
		}
	}
	return nil
}

/* getResolvedEndpoints performs DNS resolution on the provided slice of endpoints (which might be DNS names or IPv4 addresses)
   and returns a list of IPv4 addresses.  If any of the endpoints are neither valid IPv4 addresses nor resolvable DNS names,
   non-nil error is also returned (possibly along with a partially complete list of resolved endpoints.
*/
func getResolvedEndpoints(endpoints []string, netWrapper NetWrapper) ([]string, error) {
	resolvedEndpoints := sets.String{}
	for _, endpoint := range endpoints {
		if net.ParseIP(endpoint) == nil {
			// It's not a valid IP address, so assume it's a DNS name, and try to resolve it,
			// replacing its DNS name with its IP addresses in expandedEndpoints
			// through an interface abstracting the internet
			ipAddrs, err := netWrapper.LookupHost(endpoint)
			if err != nil {
				return resolvedEndpoints.List(), err
			}
			for _, ip := range ipAddrs {
				resolvedEndpoints = resolvedEndpoints.Union(sets.NewString(ip))
			}
		} else {
			resolvedEndpoints = resolvedEndpoints.Union(sets.NewString(endpoint))
		}
	}
	return resolvedEndpoints.List(), nil
}

/* ensureDNSRrsets ensures (idempotently, and with minimum mutations) that all of the DNS resource record sets for dnsName are consistent with endpoints.
   if endpoints is nil or empty, a CNAME record to uplevelCname is ensured.
*/
func (s *DNSController) ensureDNSRrsets(dnsZone dnsprovider.Zone, dnsName string, endpoints []string, uplevelCname string) error {
	rrsets, supported := dnsZone.ResourceRecordSets()
	if !supported {
		return fmt.Errorf("Failed to ensure DNS records for %s. DNS provider does not support the ResourceRecordSets interface", dnsName)
	}
	rrsetList, err := rrsets.Get(dnsName)
	if err != nil {
		return err
	}
	if len(rrsetList) == 0 {
		glog.V(4).Infof("No recordsets found for DNS name %q.  Need to add either A records (if we have healthy endpoints), or a CNAME record to %q", dnsName, uplevelCname)
		if len(endpoints) < 1 {
			glog.V(4).Infof("There are no healthy endpoint addresses at level %q, so CNAME to %q, if provided", dnsName, uplevelCname)
			if uplevelCname != "" {
				glog.V(4).Infof("Creating CNAME to %q for %q", uplevelCname, dnsName)
				newRrset := rrsets.New(dnsName, []string{uplevelCname}, minDNSTTL, rrstype.CNAME)
				glog.V(4).Infof("Adding recordset %v", newRrset)
				err = rrsets.StartChangeset().Add(newRrset).Apply()
				if err != nil {
					return err
				}
				glog.V(4).Infof("Successfully created CNAME to %q for %q", uplevelCname, dnsName)
			} else {
				glog.V(4).Infof("We want no record for %q, and we have no record, so we're all good.", dnsName)
			}
		} else {
			// We have valid endpoint addresses, so just add them as A records.
			// But first resolve DNS names, as some cloud providers (like AWS) expose
			// load balancers behind DNS names, not IP addresses.
			glog.V(4).Infof("We have valid endpoint addresses %v at level %q, so add them as A records, after resolving DNS names", endpoints, dnsName)
			// Resolve DNS through network
			resolvedEndpoints, err := getResolvedEndpoints(endpoints, s.netWrapper)
			if err != nil {
				return err // TODO: We could potentially add the ones we did get back, even if some of them failed to resolve.
			}

			newRrset := rrsets.New(dnsName, resolvedEndpoints, minDNSTTL, rrstype.A)
			glog.V(4).Infof("Adding recordset %v", newRrset)
			err = rrsets.StartChangeset().Add(newRrset).Apply()
			if err != nil {
				return err
			}
			glog.V(4).Infof("Successfully added recordset %v", newRrset)
		}
	} else {
		// the rrsets already exists, so make it right.
		glog.V(4).Infof("Recordset %v already exists. Ensuring that it is correct.", rrsetList)
		if len(endpoints) < 1 {
			// Need an appropriate CNAME record.  Check that we have it.
			newRrset := rrsets.New(dnsName, []string{uplevelCname}, minDNSTTL, rrstype.CNAME)
			glog.V(4).Infof("No healthy endpoints for %s. Have recordsets %v. Need recordset %v", dnsName, rrsetList, newRrset)
			found := findRrset(rrsetList, newRrset)
			if found != nil {
				// The existing rrset is equivalent to the required one - our work is done here
				glog.V(4).Infof("Existing recordset %v is equivalent to needed recordset %v, our work is done here.", rrsetList, newRrset)
				return nil
			} else {
				// Need to replace the existing one with a better one (or just remove it if we have no healthy endpoints).
				glog.V(4).Infof("Existing recordset %v not equivalent to needed recordset %v removing existing and adding needed.", rrsetList, newRrset)
				changeSet := rrsets.StartChangeset()
				for i := range rrsetList {
					changeSet = changeSet.Remove(rrsetList[i])
				}
				if uplevelCname != "" {
					changeSet = changeSet.Add(newRrset)
					if err := changeSet.Apply(); err != nil {
						return err
					}
					glog.V(4).Infof("Successfully replaced needed recordset %v -> %v", found, newRrset)
				} else {
					if err := changeSet.Apply(); err != nil {
						return err
					}
					glog.V(4).Infof("Successfully removed existing recordset %v", found)
					glog.V(4).Infof("Uplevel CNAME is empty string. Not adding recordset %v", newRrset)
				}
			}
		} else {
			// We have an rrset in DNS, possibly with some missing addresses and some unwanted addresses.
			// And we have healthy endpoints.  Just replace what's there with the healthy endpoints, if it's not already correct.
			glog.V(4).Infof("%s: Healthy endpoints %v exist. Recordset %v exists.  Reconciling.", dnsName, endpoints, rrsetList)
			resolvedEndpoints, err := getResolvedEndpoints(endpoints, s.netWrapper)
			if err != nil { // Some invalid addresses or otherwise unresolvable DNS names.
				return err // TODO: We could potentially add the ones we did get back, even if some of them failed to resolve.
			}
			newRrset := rrsets.New(dnsName, resolvedEndpoints, minDNSTTL, rrstype.A)
			glog.V(4).Infof("Have recordset %v. Need recordset %v", rrsetList, newRrset)
			found := findRrset(rrsetList, newRrset)
			if found != nil {
				glog.V(4).Infof("Existing recordset %v is equivalent to needed recordset %v, our work is done here.", found, newRrset)
				// TODO: We could be more thorough about checking for equivalence to avoid unnecessary updates, but in the
				//       worst case we'll just replace what's there with an equivalent, if not exactly identical record set.
				return nil
			} else {
				// Need to replace the existing one with a better one
				glog.V(4).Infof("Existing recordset %v is not equivalent to needed recordset %v, removing existing and adding needed.", found, newRrset)
				changeSet := rrsets.StartChangeset()
				for i := range rrsetList {
					changeSet = changeSet.Remove(rrsetList[i])
				}
				changeSet = changeSet.Add(newRrset)
				if err = changeSet.Apply(); err != nil {
					return err
				}
				glog.V(4).Infof("Successfully replaced recordset %v -> %v", found, newRrset)
			}
		}
	}
	return nil
}

/* ensureDNSRecords ensures (idempotently, and with minimum mutations) that all of the DNS records for a service in a given cluster are correct,
given the current state of that service in that cluster.  This should be called every time the state of a service might have changed
(either w.r.t. its loadbalancer address, or if the number of healthy backend endpoints for that service transitioned from zero to non-zero
(or vice versa).  Only shards of the service which have both a loadbalancer ingress IP address or hostname AND at least one healthy backend endpoint
are included in DNS records for that service (at all of zone, region and global levels). All other addresses are removed.  Also, if no shards exist
in the zone or region of the cluster, a CNAME reference to the next higher level is ensured to exist. */
func (s *DNSController) ensureDNSRecords(cluster, zone, region string, dnsObject *feddnsv1a1.MultiClusterDNSLb) error {
	// Quinton: Pseudocode....
	// See https://github.com/kubernetes/kubernetes/pull/25107#issuecomment-218026648
	// For each dnsObject we need the following DNS names:
	// mysvc.myns.myfed.svc.z1.r1.mydomain.com  (for zone z1 in region r1)
	//         - an A record to IP address of specific shard in that zone (if that shard exists and has healthy endpoints)
	//         - OR a CNAME record to the next level up, i.e. mysvc.myns.myfed.svc.r1.mydomain.com  (if a healthy shard does not exist in zone z1)
	// mysvc.myns.myfed.svc.r1.mydomain.com
	//         - a set of A records to IP addresses of all healthy shards in region r1, if one or more of these exist
	//         - OR a CNAME record to the next level up, i.e. mysvc.myns.myfed.svc.mydomain.com (if no healthy shards exist in region r1)
	// mysvc.myns.myfed.svc.mydomain.com
	//         - a set of A records to IP addresses of all healthy shards in all regions, if one or more of these exist.
	//         - no record (NXRECORD response) if no healthy shards exist in any regions
	//
	// Each dnsObject has the current known state of loadbalancer ingress for the federated cluster stored in annotations.
	// So generate the DNS records based on the current state and ensure those desired DNS records match the
	// actual DNS records (add new records, remove deleted records, and update changed records).
	//
	dnsObjectName := dnsObject.Name
	namespaceName := dnsObject.Namespace
	commonPrefix := dnsObjectName + "." + namespaceName + "." + s.federationName + ".svc"

	zoneDNSName := strings.Join([]string{commonPrefix, zone, region, s.dnsSuffix}, ".") // zone level
	regionDNSName := strings.Join([]string{commonPrefix, region, s.dnsSuffix}, ".")     // region level, one up from zone level
	globalDNSName := strings.Join([]string{commonPrefix, s.dnsSuffix}, ".")             // global level, one up from region level

	zoneEndpoints, regionEndpoints, globalEndpoints := s.getHealthyEndpoints(cluster, zone, region, dnsObject)
	if err := s.ensureDNSRrsets(s.dnsZone, zoneDNSName, zoneEndpoints, regionDNSName); err != nil {
		return err
	}
	if err := s.ensureDNSRrsets(s.dnsZone, regionDNSName, regionEndpoints, globalDNSName); err != nil {
		return err
	}
	if err := s.ensureDNSRrsets(s.dnsZone, globalDNSName, globalEndpoints, ""); err != nil {
		return err
	}
	return nil
}
