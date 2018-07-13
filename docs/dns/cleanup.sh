#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/../.." && pwd )"
base_dir=${base_dir##$(pwd)/}

source ${base_dir}/docs/common/util.sh

helm delete --purge andromeda
helm delete --purge milkyway
helm delete --purge whirlpool
