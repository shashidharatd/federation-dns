# DNS based cross-cluster service discovery demo

Follow the step below to bring federation and member clusters on minikube

## Prerequisite
- Ensure to bring-up atleast 2 kubernetes clusters say "cluster1" & "cluster2"
  (Provisioning LoadBalancer service should be possible in these clusters).

Note: the cluster which hosts federation should be >=v1.11.0

## Sequence
- 1. Attach Region & Zone labels to cluster nodes (for e.g. in minikube env, do as below)
```
$ kubectl --context cluster1 label node minikube \
    failure-domain.beta.kubernetes.io/region="us" \
    failure-domain.beta.kubernetes.io/zone="us1"
$ kubectl --context cluster2 label node minikube \
    failure-domain.beta.kubernetes.io/region="eu" \
    failure-domain.beta.kubernetes.io/zone="eu1"
```

- 2. Bringup federation control plane in cluster "cluster1" and join "cluster1" & "cluster2" clusters to federation
```
$ cd ${GOPATH}/src/github.com/kubernetes-sigs/federation-v2
$ kubectl config use-context cluster1
$ ./scripts/deploy-federation-latest.sh  cluster2
$ cd -
```

- 3. Start Global-DNS programmer (based on CoreDNS provider & External-DNS)
```
# DEMO_AUTO_RUN=true ./globaldns/start-dns-servers.sh cluster1
```

- 4. Make aware the in-cluster dns about federations & the global-dns server.
```
# apply the suitable federatedconfigmap in globaldns/config/
# don't forget to configure the global-dns server as upstream nameserver in case of CoreDNS
```

- 5. Finally the demo
```
# ./demo.sh  cluster1  cluster1  cluster2
```
