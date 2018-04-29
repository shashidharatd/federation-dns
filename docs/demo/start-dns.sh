#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")" && pwd )"
base_dir=${base_dir##$(pwd)/}

source ${base_dir}/util.sh

if [[ $# -ne 1 ]]; then
  echo "usage: $0  <cluster-name>"
  exit 1
fi

Cluster=$1

read -s
clear

#DNS Controller
kubectl config use-context ${Cluster}

run "kubectl create ns fed-dns"

run "kubectl -n fed-dns run etcd --image=quay.io/coreos/etcd:v3.3 --env="ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379" --env="ETCD_ADVERTISE_CLIENT_URLS=http://etcd.fed-dns:2379" --port=2379 --expose"

while [ "1" != "$(kubectl -n fed-dns get deployments etcd -o jsonpath="{.status.availableReplicas}")" ]; do
    echo "Waiting for etcd pod to be Running"
    sleep 3;
done
run "kubectl -n fed-dns get pods"

run "helm init"

run "helm version"
while [ $? -ne 0 ]; do
    sleep 3
    helm version
done

run "helm install --namespace fed-dns --name andromeda -f ${base_dir}/config/coredns-chart-values.yaml stable/coredns"

run "helm install --namespace fed-dns --name milkyway -f ${base_dir}/config/federation-dns-chart-values.yaml ${base_dir}/../../chart/federation-dns"

run "kubectl -n fed-dns get pods"
