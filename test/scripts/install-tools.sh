#！/bin/bash
## SPDX-License-Identifier: Apache-2.0
## Copyright Authors of Spider


export PATH=$PATH:$(go env GOPATH)/bin
OS=$(uname | tr 'A-Z' 'a-z')

MISS_FLAG=""

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
    echo "pass   'kubectl' installed:  $(kubectl version | grep -E -o "Client.*GitVersion:\"[^[:space:]]+\"" | awk -F, '{print $NF}') "
fi

#==================

# Install Kind Bin
if ! kind &> /dev/null ; then
    echo "fail   'kind' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
        echo "try to install it"
        if [ -z $http_proxy ]; then
          curl -Lo /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/v0.12.0/kind-$OS-amd64
        else
          curl -x $http_proxy -Lo /usr/local/bin/kind https://github.com/kubernetes-sigs/kind/releases/download/v0.12.0/kind-$OS-amd64
        fi
        chmod +x /usr/local/bin/kind
        ! kind -h  &>/dev/null && echo "error, failed to install kind" && exit 1
        echo "finish"
    fi
    MISS_FLAG="true"
else
    echo "pass   'kind' installed:  $(kind --version) "
fi

#======================

# Install Helm
if ! helm > /dev/null 2>&1 ; then
    echo "fail   'helm' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
        echo "try to install it"
        if [ -z $http_proxy ]; then
          curl -Lo /tmp/helm.tar.gz "https://get.helm.sh/helm-v3.8.1-$OS-amd64.tar.gz"
        else
          curl -x $http_proxy -Lo /tmp/helm.tar.gz "https://get.helm.sh/helm-v3.8.1-$OS-amd64.tar.gz"
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

# Install p2ctl
if ! p2ctl --version &>/dev/null ; then
    echo "fail   'p2ctl' miss"
    if [ -z "$JUST_CLI_CHECK" ] ; then
      echo "try to install it"
      if [ -z $http_proxy ]; then
        curl -Lo /usr/local/bin/p2ctl https://github.com/wrouesnel/p2cli/releases/download/r13/p2-$OS-x86_64
      else
        curl -x $http_proxy -Lo /usr/local/bin/p2ctl https://github.com/wrouesnel/p2cli/releases/download/r13/p2-$OS-x86_64
      fi
      chmod +x /usr/local/bin/p2ctl
      ! p2ctl --help &>/dev/null && echo "error, failed to install p2ctl" && exit 1
      echo "finish"
    fi
    MISS_FLAG="true"
else
    echo "pass   'p2ctl' installed:  $( p2ctl --version 2>&1 ) "
fi

[ -n "$JUST_CLI_CHECK" ] &&  [ -n "$MISS_FLAG" ] && exit 1

exit 0
