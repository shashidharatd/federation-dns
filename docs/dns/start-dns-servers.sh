#!/usr/bin/env bash

base_dir="$( cd "$(dirname "$0")/../.." && pwd )"
base_dir=${base_dir##$(pwd)/}
base_dir=.

source ${base_dir}/docs/common/util.sh

if [[ $# -lt 1 ]]; then
  echo "usage: $0  <cluster-name>  <mode(v1|v2)>"
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
run "helm install --namespace ${NS} --name andromeda -f ${base_dir}/docs/dns/config/coredns-chart-values.yaml stable/coredns"
wait_for_pods_to_be_ready "andromeda-coredns"

run "# Start Federation-DNS server"
run "helm install --namespace ${NS} --name milkyway -f ${base_dir}/docs/dns/config/federation-dns-chart-values.yaml ${base_dir}/chart/federation-dns --set variant=${Mode}"
wait_for_pods_to_be_ready "milkyway-federation-dns"

if [[ "${Mode}" == "v2" ]]; then
  run "# Start External-DNS"
  run "helm install --namespace ${NS} --name whirlpool -f ${base_dir}/docs/dns/config/external-dns-chart-values.yaml stable/external-dns"
  wait_for_pods_to_be_ready "whirlpool-external-dns"
fi

run "kubectl -n ${NS} get pods"

global_dns_server=$(kubectl -n ${NS} get svc andromeda-coredns -o jsonpath={.status.loadBalancer.ingress[0].ip})

cat <<EOF | kubectl apply -f -
apiVersion: core.federation.k8s.io/v1alpha1
kind: FederatedConfigMap
metadata:
  name: coredns
  namespace: kube-system
spec:
  template:
    metadata:
      name: coredns
      namespace: kube-system
    data:
      Corefile: |
        .:53 {
            errors
            health
            kubernetes cluster.local in-addr.arpa ip6.arpa {
               pods insecure
               upstream
               fallthrough in-addr.arpa ip6.arpa
            }
            prometheus :9153
            proxy . /etc/resolv.conf
            cache 30
            reload
            proxy dzone.io ${global_dns_server}
            federation cluster.local {
               galactic dzone.io
            }
        }
EOF

cat <<EOF | kubectl apply -f -
apiVersion: core.federation.k8s.io/v1alpha1
kind: FederatedConfigMapPlacement
metadata:
  name: coredns
  namespace: kube-system
spec:
  clusternames:
  - "minikube"
  - "secondary"
EOF