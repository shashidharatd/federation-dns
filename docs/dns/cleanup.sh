#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/../.." && pwd )"
base_dir=${base_dir##$(pwd)/}

source ${base_dir}/docs/common/util.sh

NS="fed-dns"

kubectl delete ns ${NS}
helm delete --purge andromeda
helm delete --purge milkyway
helm delete --purge whirlpool

for filename in $(ls ${base_dir}/docs/dns/config/kube-dns-configmap*.yaml); do
  kubectl delete -f "${filename}"
done
