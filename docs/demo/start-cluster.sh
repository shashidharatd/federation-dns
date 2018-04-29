#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")" && pwd )"
base_dir=${base_dir##$(pwd)/}

source ${base_dir}/util.sh

if [[ $# -ne 3 ]]; then
  echo "usage: $0  <cluster-name>  <zone-name>  <region-name>"
  exit 1
fi

Cluster=$1
Zone=$2
Region=$3

read -s
clear

# Demo
run "minikube start -p ${Cluster}"

run "kubectl apply -f https://raw.githubusercontent.com/google/metallb/v0.6.1/manifests/metallb.yaml"

run "kubectl apply -f ${base_dir}/config/${Cluster}-metallb-configmap.yaml"

run "kubectl label node ${Cluster} failure-domain.beta.kubernetes.io/zone=${Zone} failure-domain.beta.kubernetes.io/region=${Region}"
