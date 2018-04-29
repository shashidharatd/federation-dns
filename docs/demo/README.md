# DNS based cross-cluster service discovery demo

Follow the step below to bring federation and member clusters on minikube

## Prerequisite
   Federation container and config should have been built. for e.g:
```
# federation-image="<containerregistry>/<username>/<imagename>:<tagname>"
# cd ${GOPATH}/src/github.com/kubernetes-sigs/federation-v2
# apiserver-boot build container --generate=false --image=${federation-image}
# docker push ${federation-image}
# apiserver-boot build config --name federation --namespace federation --image=${federation-image}
```

Note: Switch back to the repo directory, after switching to federation-v2 directory in prerequisite section.

## Sequence
- 1. Bringup cluster "c1" in region "us" and zone "us1"
```
# DEMO_AUTO_RUN=true ./docs/demo/start-cluster.sh  c1  us1  us
```

- 2. Bringup cluster "c2" in region "eu" and zone "eu1"
```
# DEMO_AUTO_RUN=true ./docs/demo/start-cluster.sh  c2  eu1  eu
```

- 3. Bringup federation control plane in cluster "c1" and join "c1" & "c2" clusters to federation
```
# DEMO_AUTO_RUN=true ./docs/demo/start-federation.sh  c1  c1  c2
```

- 4. Start federation dns programmer (based on CoreDNS)
```
# DEMO_AUTO_RUN=true ./docs/demo/start-dns.sh  c1
```

- 5. Finally the demo
```
# ./docs/demo/demo.sh  c1  c1  c2
```
