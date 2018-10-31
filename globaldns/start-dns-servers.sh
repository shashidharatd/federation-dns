#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/../.." && pwd )"
base_dir=${base_dir##$(pwd)/}
base_dir=.

source ${base_dir}/common/util.sh

if [[ $# -lt 1 ]]; then
  echo "usage: $0  <cluster-name>"
  exit 1
fi

Cluster=$1
Mode="v1"
if [[ -n "$2" ]]; then
  Mode=$2
fi
NS="federation-system"

read -s
clear

function wait_for_pods_to_be_ready() {
  Deployment=$1
  while [ "0" == "$(kubectl -n ${NS} get deployments ${Deployment} -o jsonpath="{.status.availableReplicas}")" ]; do
    echo "Waiting for ${Deployment} pod to be Running"
    sleep 3;
  done
  run "kubectl -n ${NS} get pods"
}

kubectl config use-context ${Cluster}

run "helm init"

run "helm version 2>/dev/null"
while [ $? -ne 0 ]; do
    sleep 3
    helm version 2>/dev/null
done

run "# Start ETCD server. Used as backend for CoreDNS server"
run "kubectl -n ${NS} run etcd --image=quay.io/coreos/etcd:v3.3 --env="ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379" --env="ETCD_ADVERTISE_CLIENT_URLS=http://etcd.${NS}:2379" --port=2379 --expose"
wait_for_pods_to_be_ready "etcd"


run "# Start CoreDNS server (Global DNS)"
run "helm install --namespace ${NS} --name coredns -f ${base_dir}/globaldns/config/coredns-chart-values.yaml stable/coredns"
wait_for_pods_to_be_ready "coredns-coredns"

run "# Start External-DNS"
run "helm install --namespace ${NS} --name globaldns -f ${base_dir}/globaldns/config/globaldns-coredns-external-dns-chart-values.yaml stable/external-dns"
wait_for_pods_to_be_ready "globaldns-external-dns"

run "kubectl -n ${NS} get pods"

global_dns_server=$(kubectl -n federation-system get svc coredns-coredns -o jsonpath={.status.loadBalancer.ingress[0].ip})

echo "global_dns_server=${global_dns_server}"