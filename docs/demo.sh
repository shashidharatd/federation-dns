#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/.." && pwd )"
base_dir=${base_dir##$(pwd)/}
base_dir=.

source ${base_dir}/docs/common/util.sh

if [[ $# -ne 3 ]]; then
  echo "usage: $0  <cluster-name>  <federation-member-1>  <federation-member-2>"
  exit 1
fi

Cluster=$1
C1=$2
C2=$3

read -s
clear

# Demo
kubectl config use-context ${Cluster}

if [[ -z "${DONT_SHOW_SPEC}" ]]; then
  for filename in $(ls ${base_dir}/docs/federatedapp); do
    run "cat ${base_dir}/docs/federatedapp/${filename}"
  done
fi

run "kubectl apply -f ${base_dir}/docs/federatedapp"

run "kubectl --context=${C1} get pods"
while [ "2" != "$(kubectl --context=${C1} get rs fr1 -o jsonpath="{.status.availableReplicas}")" ]; do
    sleep 3;
done
run "kubectl --context=${C1} get ep"

run "kubectl --context=${C2} get pods"
while [ "2" != "$(kubectl --context=${C2} get rs fr1 -o jsonpath="{.status.availableReplicas}")" ]; do
    sleep 3;
done
run "kubectl --context=${C2} get ep"

coredns_server=$(kubectl -n federation-system get svc andromeda-coredns -o jsonpath={.status.loadBalancer.ingress[0].ip})
sed -i "1inameserver ${coredns_server}" /etc/resolv.conf
run "curl fs1.default.galactic.svc.dzone.io"

#kubectl patch federatedreplicasetplacements fr1 -p '{"spec":{"clusternames":["c2"]}}'
run "kubectl patch federatedreplicasetplacements fr1 -p '{\"spec\":{\"clusternames\":[\"c2\"]}}'"

run "curl fs1.default.galactic.svc.dzone.io"

sed -i "/nameserver ${coredns_server}/d" /etc/resolv.conf

#kubectl run -it --rm --restart=Never --image=infoblox/dnstools:latest dnstools
