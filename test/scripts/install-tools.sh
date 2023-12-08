#！/bin/bash
## SPDX-License-Identifier: Apache-2.0
## Copyright Authors of Spider


export PATH=$PATH:$(go env GOPATH)/bin
OS=$(uname | tr 'A-Z' 'a-z')

MISS_FLAG=""
CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)

# kubectl
if ! kubectl help &>/dev/null  ; then
    echo "fail   'kubectl' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
        echo "try to install it"
        if [ -z $http_proxy ]; then
          curl -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$OS/amd64/kubectl
        else
          curl -x $http_proxy -Lo /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/$OS/amd64/kubectl
        fi
        chmod +x /usr/local/bin/kubectl
        ! kubectl -h  &>/dev/null && echo "error, failed to install kubectl" && exit 1
        echo "finish"
    fi
    MISS_FLAG="true"
else
    echo "pass   'kubectl' installed:  $(kubectl version --client=true | grep -E -o "Client.*GitVersion:\"[^[:space:]]+\"" | awk -F, '{print $NF}') "
fi

#==================

# Install Kind Bin
if ! kind &> /dev/null ; then
    echo "fail   'kind' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
        # Spiderpool will obtain the latest kind binary by default, but when the binary version is >= 0.20.0, 
        # the following problem may occur during installation: "Command Output: WARNING: Your kernel does not support cgroup namespaces. Cgroup namespace setting discarded."
        # For this, you can refer to issue: https://github.com/kubernetes-sigs/kind/issues/3311. 
        # Or change kind version to 0.19.0 to solve it.
        echo "try to install it"
        if [ -z $http_proxy ]; then
          curl -Lo /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/$(curl -s https://api.github.com/repos/kubernetes-sigs/kind/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')/kind-$OS-amd64
        else
          curl -x $http_proxy -Lo /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/$(curl -s https://api.github.com/repos/kubernetes-sigs/kind/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')/kind-$OS-amd64
        fi
        chmod +x /usr/local/bin/kind
        ! kind -h  &>/dev/null && echo "error, failed to install kind" && exit 1
        echo "finish"
    fi
    MISS_FLAG="true"
else
    echo "pass   'kind' installed:  $(kind --version) "
fi

# docker required by kind
if ! docker &> /dev/null ; then
    echo "fail   'docker' miss"
    MISS_FLAG="true"
else
    echo "pass   'docker' installed:  $(docker -v) "
fi

#======================

# Install Helm
if ! helm > /dev/null 2>&1 ; then
    echo "fail   'helm' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
        echo "try to install it"
        if [ -z $http_proxy ]; then
          curl -Lo /tmp/helm.tar.gz https://get.helm.sh/helm-$(curl -s https://api.github.com/repos/helm/helm/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')-$OS-amd64.tar.gz
        else
          curl -x $http_proxy -Lo /tmp/helm.tar.gz https://get.helm.sh/helm-$(curl -s https://api.github.com/repos/helm/helm/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')-$OS-amd64.tar.gz
        fi
        tar -xzvf /tmp/helm.tar.gz && mv $OS-amd64/helm  /usr/local/bin
        chmod +x /usr/local/bin/helm
        rm /tmp/helm.tar.gz
        rm $OS-amd64/LICENSE
        rm $OS-amd64/README.md
        ! helm version &>/dev/null && echo "error, failed to install helm" && exit 1
        echo "finish"
    fi
    MISS_FLAG="true"
else
    echo "pass   'helm' installed:  $( helm version | grep -E -o "Version:\"v[^[:space:]]+\"" ) "
fi

# Install yq
if ! yq --version &>/dev/null ; then
    echo "fail   'yq' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
      echo "try to install it"
      if [ -z $http_proxy ]; then
        curl -Lo /tmp/yq.tar.gz https://github.com/mikefarah/yq/releases/download/$(curl -s https://api.github.com/repos/mikefarah/yq/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')/yq_linux_amd64.tar.gz
      else
        curl -x $http_proxy -Lo /tmp/yq.tar.gz https://github.com/mikefarah/yq/releases/download/$(curl -s https://api.github.com/repos/mikefarah/yq/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')/yq_linux_amd64.tar.gz
      fi
      tar -C /usr/local/bin -xzf /tmp/yq.tar.gz
      mv /usr/local/bin/yq_linux_amd64 /usr/local/bin/yq
      chmod +x /usr/local/bin/yq
      ! yq -V &>/dev/null && echo "error, failed to install yq" && exit 1
    fi
    MISS_FLAG="true"
else
    echo "pass   'yq' installed:  $( yq -V 2>&1 ) "
fi

if [ -n "$JUST_CLI_CHECK" ] && [ -n "$MISS_FLAG" ]; then
   echo "fail, the required development tools are missing on localhost, please run $CURRENT_DIR_PATH/$CURRENT_FILENAME to install them."
   exit 1
fi

exit 0
