#!/bin/bash

# SPDX-License-Identifier: Apache-2.0
# Copyright Authors of Spider

CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT_PATH=$( cd ${CURRENT_DIR_PATH}/../.. && pwd )

E2E_KUBECONFIG="$1"
# gops or detail
TYPE="$2"
E2E_FILE_NAME="$3"

[ -z "$E2E_KUBECONFIG" ] && echo "error, miss E2E_KUBECONFIG " && exit 1
[ ! -f "$E2E_KUBECONFIG" ] && echo "error, could not find file $E2E_KUBECONFIG " && exit 1
echo "$CURRENT_FILENAME : E2E_KUBECONFIG $E2E_KUBECONFIG "

NAMESPACE="kube-system"
COMPONENT_GOROUTINE_MAX=350
COMPONENT_PS_PROCESS_MAX=50


CONTROLLER_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace ${NAMESPACE} --selector app.kubernetes.io/component=spiderpool-controller --output jsonpath={.items[*].metadata.name} )
AGENT_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace ${NAMESPACE} --selector app.kubernetes.io/component=spiderpool-agent --output jsonpath={.items[*].metadata.name} )
KUBEVIRT_HANDLER_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace kubevirt --selector kubevirt.io=virt-handler --output jsonpath={.items[*].metadata.name} )
KDOCTOR_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace ${NAMESPACE} --selector app.kubernetes.io/instance=kdoctor --output jsonpath={.items[*].metadata.name} )
KRUISE_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace kruise-system --output jsonpath={.items[*].metadata.name} )

[ -z "$CONTROLLER_POD_LIST" ] && echo "error, failed to find any spider controller pod" && exit 1
[ -z "$AGENT_POD_LIST" ] && echo "error, failed to find any spider agent pod" && exit 1


if [ -n "$E2E_FILE_NAME" ] ; then
    echo "output debug information to $E2E_FILE_NAME"
    exec 6>&1
    exec >>${E2E_FILE_NAME} 2>&1
fi


RESUTL_CODE=0
if [ "$TYPE"x == "system"x ] ; then
    echo ""
    echo "=============== system data ============== "
    for POD in $CONTROLLER_POD_LIST $AGENT_POD_LIST ; do
      echo ""
      echo "--------- gops ${NAMESPACE}/${POD} "
      kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- gops stats 1
      kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- gops memstats 1

      echo ""
      echo "--------- ps ${NAMESPACE}/${POD} "
      kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- ps aux

      echo ""
      echo "--------- fd of pids ${NAMESPACE}/${POD} "
      kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- find /proc -print | grep -P '/proc/\d+/fd/' | grep -E -o "/proc/[0-9]+" | uniq -c | sort -rn | head

    done

elif [ "$TYPE"x == "detail"x ] ; then

    echo "=============== nodes status ============== "
    echo "-------- kubectl get node -o wide"
    kubectl get node -o wide --kubeconfig ${E2E_KUBECONFIG} --show-labels

    echo "=============== pods status ============== "
    echo "-------- kubectl get pod -A -o wide"
    kubectl get pod -A -o wide --kubeconfig ${E2E_KUBECONFIG} --show-labels

    echo ""
    echo "=============== event ============== "
    echo "------- kubectl get events -n ${NAMESPACE}"
    kubectl get events -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}

    echo "=============== event of error pod ============== "
    ERROR_POD=`kubectl get pod -o wide -A --kubeconfig ${E2E_KUBECONFIG} | sed '1 d' | grep -Ev "Running|Completed" | awk '{printf "%s,%s\n",$1,$2}' `
    if [ -n "$ERROR_POD" ]; then
          echo "error pod:"
          echo "${ERROR_POD}"
          for LINE in ${ERROR_POD}; do
              NS_NAME=${LINE//,/ }
              echo "---------------error pod: ${NS_NAME}------------"
              kubectl describe pod -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG}
          done
    fi

    echo ""
    echo "=============== spiderpool-controller describe ============== "
    for POD in $CONTROLLER_POD_LIST ; do
      echo ""
      echo "--------- kubectl describe pod ${POD} -n ${NAMESPACE}"
      kubectl describe pod ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
    done

    echo ""
    echo "=============== spiderpool-agent describe ============== "
    for POD in $AGENT_POD_LIST ; do
      echo ""
      echo "---------kubectl describe pod ${POD} -n ${NAMESPACE} "
      kubectl describe pod ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
    done

    echo ""
    echo "=============== spiderpool-init describe ============== "
    echo "---------kubectl describe pod -l job-name=spiderpool-init -n ${NAMESPACE} "
    kubectl describe pod -l job-name=spiderpool-init -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== spiderpool-controller logs ============== "
    for POD in $CONTROLLER_POD_LIST ; do
      echo ""
      echo "---------kubectl logs ${POD} -n ${NAMESPACE} "
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} --previous"
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo ""
    echo "=============== spiderpool-agent logs ============== "
    for POD in $AGENT_POD_LIST ; do
      echo ""
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} "
      kubectl logs ${POD} -c spiderpool-agent -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} --previous"
      kubectl logs ${POD} -c spiderpool-agent -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo ""
    echo "=============== spiderpool-init logs ============== "
    POD="spiderpool-init"
    echo "--------- kubectl logs -l job-name=spiderpool-init -n ${NAMESPACE} "
    kubectl logs -l job-name=spiderpool-init -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== spiderpool crd spiderippool ============== "
    echo "--------- kubectl get spiderippool -o wide"
    kubectl get spiderippool -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get spiderippool -o json"
    kubectl get spiderippool -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== spiderpool crd spiderendpoint ============== "
    echo "-------- kubectl get spiderendpoint -o wide "
    kubectl get spiderendpoint -A -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "-------- kubectl get spiderendpoint -o json "
    kubectl get spiderendpoint -A -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== spiderpool crd spiderreservedips ============== "
    echo "-------- kubectl get spiderreservedips -o wide "
    kubectl get spiderreservedips -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "-------- kubectl get spiderreservedips -o json "
    kubectl get spiderreservedips -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== spiderpool crd spidersubnet ============== "
    echo "-------- kubectl get spidersubnet -o wide "
    kubectl get spidersubnet -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "-------- kubectl get spidersubnet -o json "
    kubectl get spidersubnet -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get spidermultusconfig -o wide -A"
    kubectl get spidermultusconfig -A -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get spidermultusconfig -A -o json"
    kubectl get spidermultusconfig -A -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get network-attachment-definitions.k8s.cni.cncf.io -A -o wide"
    kubectl get network-attachment-definitions.k8s.cni.cncf.io -A -o wide --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get network-attachment-definitions.k8s.cni.cncf.io -A -o json"
    kubectl get network-attachment-definitions.k8s.cni.cncf.io -A -o json --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "--------- kubectl get configmaps -n kube-system spiderpool-conf -ojson"
    kubectl get configmaps -n kube-system spiderpool-conf -ojson --kubeconfig ${E2E_KUBECONFIG}

    echo ""
    echo "=============== IPAM log  ============== "
    KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-"spider"}
    KIND_NODES=$(  kind get  nodes --name ${KIND_CLUSTER_NAME} )
    [ -z "$KIND_NODES" ] && echo "warning, failed to find nodes of kind cluster $KIND_CLUSTER_NAME " || true
    for NODE in $KIND_NODES ; do
        echo "--------- IPAM logs from node ${NODE}"
        docker exec $NODE cat /var/log/spidernet/spiderpool.log
        echo "--------- coordinator logs from node ${NODE}"
        docker exec $NODE cat /var/log/spidernet/coordinator.log
        echo "--------- ip rule from ${NODE}"
        docker exec $NODE ip rule || true
        docker exec $NODE ip -6 rule || true
        echo "--------- ip n from ${NODE}"
        docker exec $NODE ip n || true
        docker exec $NODE ip -6 n || true
        echo "--------- ip route show table 500 from ${NODE}"
        docker exec $NODE ip route show table 500 || true 
        docker exec $NODE ip -6 route show table 500 || true 
        echo "--------- ip link show from ${NODE}"
        docker exec $NODE ip link show
    done

    echo ""
    echo "=============== api-server logs ============== "
    CHECK_POD=$(kubectl get pod -o wide -n kube-system --kubeconfig ${E2E_KUBECONFIG} | grep kube-apiserver | awk '{print $1}')
    for POD in $CHECK_POD ; do
      echo ""
      echo "--------- kubectl logs ${POD} -n kube-system "
      kubectl logs ${POD} -n kube-system --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n kube-system --previous"
      kubectl logs ${POD} -n kube-system --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo ""
    echo "=============== kubevirt handler logs ============== "
    for POD in $KUBEVIRT_HANDLER_POD_LIST ; do
      echo ""
      echo "--------- kubectl logs ${POD} -n kubevirt "
      kubectl logs ${POD} -n kubevirt --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} --previous"
      kubectl logs ${POD} -n kubevirt --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo "=============== Check the network information of the pod ============== "
    CHECK_POD=$(kubectl get pod -o wide -A --kubeconfig ${E2E_KUBECONFIG} | sed '1 d' | grep -Ev "kube-system|kruise-system|kubevirt|local-path-storage" | awk '{printf "%s,%s\n",$1,$2}')
    if [ -n "$CHECK_POD" ]; then
          echo "check pod:"
          echo "${CHECK_POD}"
          for LINE in ${CHECK_POD}; do
              NS_NAME=${LINE//,/ }
              echo "--------------- execute ip a in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip a
              echo "--------------- execute ip link show in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip link show
              echo "--------------- execute ip n in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip n
              echo "--------------- execute ip -6 n in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip -6 n
              echo "--------------- execute ip rule in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip rule
              echo "--------------- execute ip -6 rule in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip -6 rule
              echo "--------------- execute ip route in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip route
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip route show table 100
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip route show table 101
              echo "--------------- execute ip -6 route in pod: ${NS_NAME} ------------"
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip -6 route
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip -6 route table 100
              kubectl exec -ti -n ${NS_NAME} --kubeconfig ${E2E_KUBECONFIG} -- ip -6 route table 101
          done
    fi

    echo ""
    echo "=============== kdoctor logs ============== "
    for POD in $KDOCTOR_POD_LIST ; do
      echo ""
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} "
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} --previous"
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo ""
    echo "=============== open kruise logs ============== "
    for POD in $KRUISE_POD_LIST ; do
      echo ""
      echo "--------- kubectl logs ${POD} -n kruise-system "
      kubectl logs ${POD} -n kruise-system --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n kruise-system --previous"
      kubectl logs ${POD} -n kruise-system --kubeconfig ${E2E_KUBECONFIG} --previous
    done
    
    echo ""
    echo "=============== kdoctor netreach details ============== "
    kubectl get netreach --kubeconfig ${E2E_KUBECONFIG}
    kubectl get netreach --kubeconfig ${E2E_KUBECONFIG} -o yaml

elif [ "$TYPE"x == "error"x ] ; then
    CHECK_ERROR(){
        LLOG_MARK="$1"
        LPOD="$2"
        LNAMESPACE="$3"

        if [[ "${LPOD}" == *spiderpool-agent* ]] ; then
            LPOD="${LPOD} -c spiderpool-agent "
        fi

        echo ""
        echo "---------${LPOD}--------"
        MESSAGE=` kubectl logs ${LPOD} -n ${LNAMESPACE} --kubeconfig ${E2E_KUBECONFIG} |& grep -E -i "${LLOG_MARK}" `
        if  [ -n "$MESSAGE" ] ; then
            echo "error, in ${LPOD}, found error, ${LLOG_MARK} !!!!!!!"
            echo "${MESSAGE}"
            RESUTL_CODE=1
        else
            echo "no error "
        fi
    }

    DATA_RACE_LOG_MARK="WARNING: DATA RACE"
    LOCK_LOG_MARK="Goroutine took lock"
    PANIC_LOG_MARK="panic .* runtime error"

    echo ""
    echo "=============== check kinds of error  ============== "
    for POD in $CONTROLLER_POD_LIST $AGENT_POD_LIST ; do
        echo ""
        echo "----- check data race in ${NAMESPACE}/${POD} "
        CHECK_ERROR "${DATA_RACE_LOG_MARK}" "${POD}" "${NAMESPACE}"

        echo ""
        echo "----- check long lock in ${NAMESPACE}/${POD} "
        CHECK_ERROR "${LOCK_LOG_MARK}" "${POD}" "${NAMESPACE}"

        echo ""
        echo "----- check panic in ${NAMESPACE}/${POD} "
        CHECK_ERROR "${PANIC_LOG_MARK}" "${POD}" "${NAMESPACE}"

        echo ""
        echo "----- check gorouting leak in ${NAMESPACE}/${POD} "
        GOROUTINE_NUM=`kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- gops stats 1 | grep "goroutines:" | grep -E -o "[0-9]+" `
        if [ -z "$GOROUTINE_NUM" ] ; then
            echo "warning, failed to find GOROUTINE_NUM in ${NAMESPACE}/${POD} "
        elif (( GOROUTINE_NUM >= COMPONENT_GOROUTINE_MAX )) ; then
             echo "maybe goroutine leak, found ${GOROUTINE_NUM} goroutines in ${NAMESPACE}/${POD} , which is bigger than default ${COMPONENT_GOROUTINE_MAX}"
             RESUTL_CODE=1
        fi

        echo ""
        echo "----- check pod restart in ${NAMESPACE}/${POD}"
        RESTARTS=` kubectl get pod ${POD} -n ${NAMESPACE} -o wide --kubeconfig ${E2E_KUBECONFIG} | sed '1 d'  | awk '{print $4}' `
        if [ -z "$RESTARTS" ] ; then
            echo "warning, failed to find RESTARTS in ${NAMESPACE}/${POD} "
        elif (( RESTARTS != 0 )) ; then
             echo "found pod restart event"
             RESUTL_CODE=1
        fi

        echo ""
        echo "----- check process number in ${NAMESPACE}/${POD}"
        PROCESS_NUM=` kubectl exec ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} -- ps aux | wc -l `
        if [ -z "$PROCESS_NUM" ] ; then
            echo "warning, failed to find process in ${NAMESPACE}/${POD} "
        elif (( PROCESS_NUM >= COMPONENT_PS_PROCESS_MAX )) ; then
             echo "error, found ${PROCESS_NUM} process more than default $COMPONENT_PS_PROCESS_MAX "
             RESUTL_CODE=1
        fi

        echo ""
        echo "----- check some warning log in ${NAMESPACE}/${POD}"
        WARNING_LOG="ERROR.*exhaust all retries"
        NUMBER=` kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} |& grep -E -i "${WARNING_LOG}" | wc -l `
        if  (( NUMBER != 0 )) ; then
            echo "warning, in ${POD}, found $NUMBER line log with ${WARNING_LOG} !!!!!!!"
            # no fail the CI
        else
            echo "no warning log "
        fi

        echo ""
        echo "----- check ERROR level log in ${NAMESPACE}/${POD}"
        WARNING_LOG='"level":"ERROR"'
        NUMBER=` kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} |& grep -E -i "${WARNING_LOG}" | wc -l `
        if  (( NUMBER != 0 )) ; then
            echo "in ${POD}, found $NUMBER line log with ${WARNING_LOG} !!!!!!!"
            # no fail the CI
        else
            echo "no warning log "
        fi

    done


else
    echo "error, unknown type $TYPE "
    RESUTL_CODE=1
fi

exit $RESUTL_CODE
