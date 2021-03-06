#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/.." && pwd )"
base_dir=${base_dir##$(pwd)/}
base_dir=.

source ${base_dir}/common/util.sh

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

run "kubectl apply -f ${base_dir}/federated-service"

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

run "kubectl run dnstools --rm --restart=Never -i --image=infoblox/dnstools --command -- curl -s fs1.default.production"

run "kubectl patch federatedreplicasetplacements fr1 -p '{\"spec\":{\"clusternames\":[\"${C2}\"]}}'"

run "kubectl run dnstools --rm --restart=Never -i --image=infoblox/dnstools --command -- curl -s fs1.default.production"
