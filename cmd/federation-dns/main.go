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

package main

import (
	"flag"
	"fmt"

	"github.com/golang/glog"

	controllerlib "github.com/kubernetes-incubator/apiserver-builder/pkg/controller"
	fedclientset "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/clientset"
	"github.com/shashidharatd/federation-dns/pkg/controller/dns"
	"github.com/shashidharatd/federation-dns/pkg/dnsprovider"
	"github.com/shashidharatd/federation-dns/pkg/source"
	restclient "k8s.io/client-go/rest"

	// DNS providers
	_ "github.com/shashidharatd/federation-dns/pkg/dnsprovider/providers/aws/route53"
	_ "github.com/shashidharatd/federation-dns/pkg/dnsprovider/providers/azure/azuredns"
	_ "github.com/shashidharatd/federation-dns/pkg/dnsprovider/providers/coredns"
	_ "github.com/shashidharatd/federation-dns/pkg/dnsprovider/providers/google/clouddns"
)

var kubeconfig = flag.String("kubeconfig", "", "path to kubeconfig")
var federationName = flag.String("federation-name", "", "Federation name.")
var zoneName = flag.String("zone-name", "", "Zone name, like example.com.")
var zoneID = flag.String("zone-id", "", "Zone ID, needed if the zone name is not unique.")
var dnsSuffix = flag.String("dns-suffix", "", "DNS Suffix to use when publishing federated service names.  Defaults to zone-name")
var provider = flag.String("provider", "", "DNS provider. Valid values are: "+fmt.Sprintf("%q", dnsprovider.RegisteredDnsProviders()))
var configFile = flag.String("provider-config", "", "Path to config file for configuring DNS provider.")
var variant = flag.String("variant", "v1", "v1 or v2")
var servicePort = flag.String("service-port", "8080", "port on which to make listing dns endpoint service available")

func main() {
	flag.Parse()
	config, err := controllerlib.GetConfig(*kubeconfig)
	if err != nil {
		glog.Fatalf("Could not create Config for talking to the apiserver: %v", err)
	}

	stopChan := make(chan struct{})

	userAgent := "federation-dns"
	restclient.AddUserAgent(config, userAgent)
	fedClient := fedclientset.NewForConfigOrDie(config)

	switch *variant {
	case "v1":
		controller, err := dns.NewDNSController(fedClient, *provider, *configFile, *federationName, *dnsSuffix, *zoneName, *zoneID)
		if err != nil {
			glog.Fatalf("Error starting dns controller: %v", err)
		}

		glog.Infof("Starting dns controller")
		controller.Run(3, stopChan)

		// Blockforever
		select {}
	case "v2":
		controller, err := source.NewFederationSource(fedClient.MulticlusterdnsV1alpha1(), "", *federationName, *zoneName, *servicePort)
		if err != nil {
			glog.Fatalf("Error starting dns controller: %v", err)
		}
		controller.Run()
	default:
		glog.Fatalf("incorrect variant %s, should be v1 or v2", *variant)
	}
}
