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
COMPONENT_GOROUTINE_MAX=300
COMPONENT_PS_PROCESS_MAX=50


CONTROLLER_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace ${NAMESPACE} --selector app.kubernetes.io/component=spiderpool-controller --output jsonpath={.items[*].metadata.name} )
AGENT_POD_LIST=$( kubectl get pods --no-headers --kubeconfig ${E2E_KUBECONFIG}  --namespace ${NAMESPACE} --selector app.kubernetes.io/component=spiderpool-agent --output jsonpath={.items[*].metadata.name} )
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
    POD="spdierpool-init"
    echo "---------kubectl describe pod ${POD} -n ${NAMESPACE} "
    kubectl describe pod ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}

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
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}
      echo "--------- kubectl logs ${POD} -n ${NAMESPACE} --previous"
      kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} --previous
    done

    echo ""
    echo "=============== spiderpool-init logs ============== "
    POD="spdierpool-init"
    echo "--------- kubectl logs ${POD} -n ${NAMESPACE} "
    kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG}

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
    echo "=============== IPAM log  ============== "
    KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-"spider"}
    KIND_NODES=$(  kind get  nodes --name ${KIND_CLUSTER_NAME} )
    [ -z "$KIND_NODES" ] && echo "warning, failed to find nodes of kind cluster $KIND_CLUSTER_NAME " || true
    for NODE in $KIND_NODES ; do
        echo "--------- IPAM logs from node ${NODE}"
        docker exec $NODE cat /var/log/spidernet/spiderpool.log
        echo "--------- coordinator logs from node ${NODE}"
        docker exec $NODE cat /var/log/spidernet/coordinator.log
    done


elif [ "$TYPE"x == "error"x ] ; then
    CHECK_ERROR(){
        LOG_MARK="$1"
        POD="$2"
        NAMESPACE="$3"

        echo ""
        echo "---------${POD}--------"
        MESSAGE=` kubectl logs ${POD} -n ${NAMESPACE} --kubeconfig ${E2E_KUBECONFIG} |& grep -E -i "${LOG_MARK}" `
        if  [ -n "$MESSAGE" ] ; then
            echo "error, in ${POD}, found error, ${LOG_MARK} !!!!!!!"
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
