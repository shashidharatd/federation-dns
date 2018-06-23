#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/.." && pwd )"
base_dir=${base_dir##$(pwd)/}

kubectl delete -f ${base_dir}/docs/federatedapp 2>/dev/null
kubectl delete multiclusterservicednsrecords fs1 2>/dev/null
