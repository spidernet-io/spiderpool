#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -eo pipefail
if [[ $1 = "--help" ]] || [[ $1 = "-h" ]];then
    echo "example: -n|--number-of-compute 5"
    exit 0
fi

while true; do
  case "$1" in
    -n|--number-of-compute)
      NUMBER_OF_COMPUTE_NODES=$2
      break
      ;;
    *)
      echo "define argument -n (number of compute nodes)"
      exit 1
  esac
done

HERE="$(dirname "$(readlink --canonicalize ${BASH_SOURCE[0]})")"
ROOT="$(readlink --canonicalize "$HERE/..")"
MULTUS_DAEMONSET=multus-daemonset.yml
CNIS_DAEMONSET_URL=cni-install.yml
#MULTUS_DAEMONSET_URL="https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset.yml"
#CNIS_DAEMONSET_URL="https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/e2e/cni-install.yml"
TIMEOUT_K8="5000s"
RETRY_MAX=10
INTERVAL=10
TIMEOUT=300
TIMEOUT_K8="${TIMEOUT}s"
KIND_CLUSTER_NAME="whereabouts"
OCI_BIN="${OCI_BIN:-"docker"}"
IMG_PROJECT="whereabouts"
IMG_REGISTRY="ghcr.io/k8snetworkplumbingwg"
IMG_TAG="latest-amd64"
IMG_NAME="$IMG_REGISTRY/$IMG_PROJECT:$IMG_TAG"

create_cluster() {
workers="$(for i in $(seq $NUMBER_OF_COMPUTE_NODES); do echo "  - role: worker"; done)"
  # deploy cluster with kind
  cat <<EOF | kind create cluster --name $KIND_CLUSTER_NAME --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
$workers
EOF
}


check_requirements() {
  for cmd in "$OCI_BIN" kind kubectl; do
    if ! command -v "$cmd" &> /dev/null; then
      echo "$cmd is not available"
      exit 1
    fi
  done
}

retry() {
  local status=0
  local retries=${RETRY_MAX:=5}
  local delay=${INTERVAL:=5}
  local to=${TIMEOUT:=20}
  cmd="$*"

  while [ $retries -gt 0 ]
  do
    status=0
    timeout $to bash -c "echo $cmd && $cmd" || status=$?
    if [ $status -eq 0 ]; then
      break;
    fi
    echo "Exit code: '$status'. Sleeping '$delay' seconds before retrying"
    sleep $delay
    let retries--
  done
  return $status
}

echo "## checking requirements"
check_requirements
echo "## delete existing KinD cluster if it exists"
kind delete clusters $KIND_CLUSTER_NAME
echo "## start KinD cluster"
create_cluster
kind export kubeconfig --name $KIND_CLUSTER_NAME
echo "## wait for coreDNS"
kubectl -n kube-system wait --for=condition=available deploy/coredns --timeout=$TIMEOUT_K8
echo "## install multus"
retry kubectl create -f "${MULTUS_DAEMONSET_URL}"
retry kubectl -n kube-system wait --for=condition=ready -l name="multus" pod --timeout=$TIMEOUT_K8
echo "## install CNIs"
retry kubectl create -f "${CNIS_DAEMONSET_URL}"
# kubectl get ds install-cni-plugins -n kube-system -oyaml > cni-plugins.yaml
# sed -i 's/imagePullPolicy: Always/imagePullPolicy: IfNotPresent/g' cni-plugins.yaml
# kubectl delete ds install-cni-plugins -n kube-system
# retry kubectl create -f ./cni-plugins.yaml
# docker pull docker.io/library/alpine:latest
# kind load docker-image alpine:latest --name "whereabouts"
# retry kubectl -n kube-system wait --for=condition=ready -l name="cni-plugins" pod --timeout=$TIMEOUT_K8

echo "## load image into KinD"
trap "rm /tmp/whereabouts-img.tar || true" EXIT
"$OCI_BIN" save -o /tmp/whereabouts-img.tar "$IMG_NAME"
kind load image-archive --name "$KIND_CLUSTER_NAME" /tmp/whereabouts-img.tar

echo "## install whereabouts"
for file in "daemonset-install.yaml" "ip-reconciler-job.yaml" "whereabouts.cni.cncf.io_ippools.yaml" "whereabouts.cni.cncf.io_overlappingrangeipreservations.yaml"; do
  retry kubectl apply -f "$ROOT/doc/crds/$file"
done
retry kubectl wait -n kube-system --for=condition=ready -l app=whereabouts pod --timeout=$TIMEOUT_K8
echo "## done"