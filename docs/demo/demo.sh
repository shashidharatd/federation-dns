#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")" && pwd )"
base_dir=${base_dir##$(pwd)/}

source ${base_dir}/util.sh

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

for file in $(ls ${base_dir}/fs1); do
  run "cat ${base_dir}/fs1/${file}"
done

run "kubectl apply -f ${base_dir}/fs1"

while [ "2" != "$(kubectl --context=${C1} get rs fr1 -o jsonpath="{.status.availableReplicas}")" ]; do
    sleep 3;
done
run "kubectl --context=${C1} get pods"

while [ "2" != "$(kubectl --context=${C2} get rs fr1 -o jsonpath="{.status.availableReplicas}")" ]; do
    sleep 3;
done
run "kubectl --context=${C2} get pods"

coredns_server=$(kubectl -n fed-dns get svc andromeda-coredns -o jsonpath={.status.loadBalancer.ingress[0].ip})
sed -i "1inameserver ${coredns_server}" /etc/resolv.conf
run "dig fs1.default.galactic.svc.dzone.io"
run "dig fs1.default.galactic.svc.eu1.eu.dzone.io"
run "dig fs1.default.galactic.svc.us1.us.dzone.io"

run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"

run "kubectl patch federatedreplicasetplacements fr1 -p '{\"spec\":{\"clusternames\":[\"c1\"]}}'"

run "dig fs1.default.galactic.svc.dzone.io"
run "dig fs1.default.galactic.svc.eu1.eu.dzone.io"
run "dig fs1.default.galactic.svc.us1.us.dzone.io"

run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"
run "curl fs1.default.galactic.svc.dzone.io"

sed -i "/nameserver ${coredns_server}/d" /etc/resolv.conf

run "kubectl delete -f ${base_dir}/fs1"
