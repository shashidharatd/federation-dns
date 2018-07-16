# DNS based cross-cluster service discovery demo

Follow the step below to bring federation and member clusters on minikube

## Prerequisite
- Ensure to bring-up atleast 2 kubernetes clusters (k8s version should be >=v1.11.0), say "minikube" & "secondary"
  (Provisioning LoadBalancer service should be possible in theses clusters).

## Sequence
- 1. Attach Region & Zone labels to cluster nodes (for e.g. in minikube env, do as below)
```
$ kubectl --context minikube label node minikube \
    failure-domain.beta.kubernetes.io/region="us" \
    failure-domain.beta.kubernetes.io/zone="us1"
$ kubectl --context secondary label node minikube \
    failure-domain.beta.kubernetes.io/region="eu" \
    failure-domain.beta.kubernetes.io/zone="eu1"
```

- 2. Bringup federation control plane in cluster "minikube" and join "minikube" & "secondary" clusters to federation
```
$ cd ${GOPATH}/src/github.com/kubernetes-sigs/federation-v2
$ kubectl config use-context minikube
$ ./scripts/deploy-federation-latest.sh  secondary
$ cd -
```

- 4.1. Start federation dns programmer (based on CoreDNS provider)
```
# DEMO_AUTO_RUN=true ./docs/dns/start-dns-servers.sh  minikube
```

OR

- 4.2. Start federation dns programmer (based on CoreDNS provider & External-DNS)
```
# DEMO_AUTO_RUN=true ./docs/dns/start-dns-servers.sh minikube  v2
```

- 5. Finally the demo
```
# ./docs/demo.sh  minikube  minikube  secondary
```
