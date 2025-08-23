#!/bin/bash
# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

: <<EOF
set to all rdma interfaces:
    GPU_RDMA_PRIORITY=5 GPU_CNP_PRIORITY=6  ./set-rdma-qos.sh

query the configuration:
    ./set-rdma-qos.sh  q
EOF

# set -x
# set -o xtrace
set -o errexit

GPU_RDMA_PRIORITY=${GPU_RDMA_PRIORITY:-5}
GPU_RDMA_QOS=${GPU_RDMA_QOS:-""}
GPU_CNP_PRIORITY=${GPU_CNP_PRIORITY:-6}
GPU_CNP_QOS=${GPU_NIC_QOS:-""}

if [ "$1" == "q" ]; then
    # only for pf
    ALL_RDMA_NICS=$(rdma link | grep "netdev" | grep -oE "netdev .*" | awk '{print $2}')
    for dev in $ALL_RDMA_NICS; do
        # vf device
        [ -f "/sys/class/net/$dev/ecn/roce_np/cnp_dscp" ] || continue
        INFO=$(mlnx_qos -i $dev)
        RDMA_DEV=$(rdma link | grep " netdev ${dev} " | awk '{print $2}' | awk -F'/' '{print $1}')
        echo ""
        echo "======== show PF device $dev / ${RDMA_DEV}========"

        echo "$INFO" | grep "Priority trust state"
        echo "$INFO" | grep -A 3 "PFC configuration"
        for NUM in {0..7}; do
            echo "ECN Enabled for priority $NUM: /sys/class/net/$dev/ecn/roce_np/enable/${NUM} = $(cat /sys/class/net/$dev/ecn/roce_np/enable/${NUM})"
            echo "ECN Enabled for priority $NUM: /sys/class/net/$dev/ecn/roce_rp/enable/${NUM} = $(cat /sys/class/net/$dev/ecn/roce_rp/enable/${NUM})"
        done
        echo "QOS for CNP: /sys/class/net/$dev/ecn/roce_np/cnp_dscp = $(cat /sys/class/net/$dev/ecn/roce_np/cnp_dscp)"

    done

    # for pf and vf
    echo ""
    echo "======== show qos ==========="
    ALL_RDMA_DEVS=$(rdma link | awk '{print $2}' | awk -F'/' '{print $1}' | tr '\n' ' ')
    for RDMA_DEV in $ALL_RDMA_DEVS; do
        echo "rdma device ${RDMA_DEV}:   rdma_cm qos = $(cma_roce_tos –d ${RDMA_DEV})"
        echo "rdma device ${RDMA_DEV}:   libverbs qos: /sys/class/infiniband/${RDMA_DEV}/tc/1/traffic_class = $(cat /sys/class/infiniband/${RDMA_DEV}/tc/1/traffic_class)"
        echo ""
    done
    exit 0
fi

#=======================

echo -e "\e[31m Prepare rdma qos script \e[0m"

# dscp mapping to priority
# mlnx_qos -i enp11s0f0np0
declare -A prio_to_dscp=(
    [0]=" 7 6 5 4 3 2 1 0 "
    [1]=" 15 14 13 12 11 10 9 8 "
    [2]=" 23 22 21 20 19 18 17 16 "
    [3]=" 31 30 29 28 27 26 25 24 "
    [4]=" 39 38 37 36 35 34 33 32 "
    [5]=" 47 46 45 44 43 42 41 40 "
    [6]=" 55 54 53 52 51 50 49 48 "
    [7]=" 63 62 61 60 59 58 57 56 "
)

# Check if ecn_priority is within the range 0-7
if ! [[ "$GPU_RDMA_PRIORITY" =~ ^[0-7]$ ]]; then
    echo "error, GPU_RDMA_PRIORITY must be in the range of 0-7, but got $GPU_RDMA_PRIORITY"
    exit 1
fi

if ! [[ "$GPU_CNP_PRIORITY" =~ ^[0-7]$ ]]; then
    echo "error, GPU_CNP_PRIORITY must be in the range of 0-7, but got $GPU_CNP_PRIORITY"
    exit 1
fi

if [[ "$GPU_RDMA_PRIORITY" -eq "$GPU_CNP_PRIORITY" ]]; then
    echo "error, GPU_RDMA_PRIORITY and GPU_CNP_PRIORITY cannot be the same"
    exit 1
fi

[ -z "$GPU_RDMA_QOS" ] && GPU_RDMA_QOS=$((GPU_RDMA_PRIORITY * 8))
[ -z "$GPU_CNP_QOS" ] && GPU_CNP_QOS=$((GPU_CNP_PRIORITY * 8))

if [[ ! " ${prio_to_dscp[$GPU_RDMA_PRIORITY]} " =~ " $GPU_RDMA_QOS " ]]; then
    echo "error, GPU_RDMA_QOS ($GPU_RDMA_QOS) is not in the mapping table for GPU_RDMA_PRIORITY $GPU_RDMA_PRIORITY, Qos should be in ${prio_to_dscp[$GPU_RDMA_PRIORITY]}"
    exit 1
fi

if [[ ! " ${prio_to_dscp[$GPU_CNP_PRIORITY]} " =~ " $GPU_CNP_QOS " ]]; then
    echo "error, GPU_CNP_QOS ($GPU_CNP_QOS) is not in the mapping table for GPU_CNP_PRIORITY ($GPU_CNP_PRIORITY), Qos should be in ${prio_to_dscp[$GPU_CNP_PRIORITY]}"
    exit 1
fi

if [[ "$GPU_RDMA_QOS" -eq "$GPU_CNP_QOS" ]]; then
    echo "error, GPU_RDMA_QOS and GPU_CNP_QOS cannot be the same"
    exit 1
fi

# ##################### wait unit all tools are ready ################################
if ! which mlnx_qos >/dev/null; then
    echo "mlnx_qos is not ready..."
    exit 1
fi
echo "mlnx_qos is ready"

if ! ibdev2netdev >/dev/null; then
    echo "ibdev2netdev is not ready..."
    exit 1
fi
echo "ibdev2netdev is ready"

modprobe rdma_cm
if ! cma_roce_tos >/dev/null; then
    echo "cma_roce_tos is not ready..."
    exit 1
fi
echo "cma_roce_tos is ready"

echo "GPU_RDMA_PRIORITY=${GPU_RDMA_PRIORITY}"
echo "GPU_CNP_PRIORITY=${GPU_CNP_PRIORITY}"
echo "GPU_RDMA_QOS=${GPU_RDMA_QOS}"
echo "GPU_CNP_QOS=${GPU_CNP_QOS}"

cat <<G_EOF >/usr/local/bin/rdma_qos.sh
#!/bin/bash

# set -x
# set -o xtrace
set -o errexit

GPU_RDMA_PRIORITY="$GPU_RDMA_PRIORITY"
GPU_CNP_PRIORITY="$GPU_CNP_PRIORITY"
GPU_RDMA_QOS="$GPU_RDMA_QOS"
GPU_CNP_QOS="$GPU_CNP_QOS"
G_EOF

cat <<"S_EOF" >>/usr/local/bin/rdma_qos.sh
RUN_ONCE=${RUN_ONCE:-false}
DEBUG_LOG=${DEBUG_LOG:-false}

function set_rdma_qos() {
    #  $GPU_RDMA_PRIORITY $GPU_RDMA_QOS $GPU_CNP_QOS $DEBUG_LOG

    rdma_priority=$1
    if [ -z "$rdma_priority" ]; then
        echo "error, rdma_priority is empty"
        exit 1
    fi

    rdma_qos=$2
    if [ -z "$rdma_qos" ]; then
        echo "error, rdma_qos is empty"
        exit 1
    fi

    cnp_qos=$3
    if [ -z "$cnp_qos" ]; then
        echo "error, cnp_qos is empty"
        exit 1
    fi

    debug_log=$4
    if [ -z "$debug_log" ]; then
        echo "error, debug_log is empty"
        exit 1
    fi


    qos_queues=(0 0 0 0 0 0 0 0)
    qos_queues[$rdma_priority]=1
    pfc_queue=$(echo "${qos_queues[*]}" | sed 's? ?,?g' | tr -d ' ')
    $debug_log && echo "Qos Parameters: rdma_priority: $rdma_priority, rdma_qos: $rdma_qos, cnp_priority: $cnp_priority, cnp_qos: $cnp_qos, pfc_queue: $pfc_queue"

    rdma_int_list=$( rdma link | grep "netdev" | grep -oE "netdev .*" | awk '{print $2}'  )
    for nic_item in ${rdma_int_list} ; do

        ip a show "$nic_item" | grep link/infiniband &>/dev/null && {
            $debug_log && echo "ignore infiniband nic $nic_item"
            continue
        }
        ip link set $nic_item up 

        # ------------ for pf ---------------------------
        ECN_ROOT="/sys/class/net/$nic_item/ecn"
        [ -d "${ECN_ROOT}" ] || continue

        $debug_log && echo -e "\e[31minfo, start to apply QoS and ecn for nic $nic_item, rdma device $rdma_dev ...\e[0m"
        mlnx_qos -i "$nic_item" --trust=dscp --pfc ${pfc_queue} &> /dev/null
        $debug_log && mlnx_qos -i "$nic_item" 

        $debug_log && echo "echo 1 >${ECN_ROOT}/roce_np/enable/$rdma_priority"
        echo 1 >"${ECN_ROOT}"/roce_np/enable/$rdma_priority

        $debug_log && echo "echo 1 >${ECN_ROOT}/roce_rp/enable/$rdma_priority"
        echo 1 >"${ECN_ROOT}"/roce_rp/enable/$rdma_priority

        $debug_log && echo "echo $cnp_qos >${ECN_ROOT}/roce_np/cnp_dscp"
        echo $cnp_qos >"${ECN_ROOT}"/roce_np/cnp_dscp

    done

    rdma_dev_list=$( rdma link | awk '{print $2}' | awk -F'/' '{print $1}'  | tr '\n' ' '   )
    for rdma_dev in ${rdma_dev_list} ; do
        # ------------ for vf and pf ---------------------------
        $debug_log && echo -e "\e[31minfo, start to apply cma_roce_tox for port ${rdma_dev}\e[0m"
        traffic_class=$((rdma_qos << 2))

        $debug_log && echo "cma_roce_tos -d $rdma_dev -t $traffic_class"
        cma_roce_tos -d $rdma_dev -t $traffic_class &> /dev/null
        cma_roce_mode -d $rdma_dev -p 1 -m 2 &> /dev/null
 
        $debug_log && echo "echo $traffic_class >/sys/class/infiniband/$rdma_dev/tc/1/traffic_class"
        echo $traffic_class >/sys/class/infiniband/$rdma_dev/tc/1/traffic_class
    done    
}


# sometime, it encounter: cma_roce_tos : Module rdma_cm is not loaded or does not support configfs
modprobe rdma_cm

while true ; do
    set_rdma_qos $GPU_RDMA_PRIORITY $GPU_RDMA_QOS $GPU_CNP_QOS $DEBUG_LOG

    sysctl -w net.ipv4.tcp_ecn=1 &> /dev/null
    if [ "$RUN_ONCE" = true ] ; then
        exit 0
    fi

    echo "Done, sleep 60s"
    sleep 20
done
S_EOF

chmod +x /usr/local/bin/rdma_qos.sh
echo -e "\e[31m Pre-run rdma_qos.sh once \e[0m"
RUN_ONCE=true DEBUG_LOG=true /usr/local/bin/rdma_qos.sh || {
    echo "error, failed to pre-set qos"
    exit 1
}

echo -e "\e[31m Prepare rdma qos systemd unit file \e[0m"

cat <<"SYS_EOF" >/etc/systemd/system/rdma-qos.service
[Unit]
Description=RDMA QoS Configuration Service
After=network.target

[Service]
Type=simple
Restart=always
ExecStart=/bin/bash /usr/local/bin/rdma_qos.sh
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SYS_EOF

echo -e "\e[31m Start rdma-qos systemd service \e[0m"
systemctl daemon-reload
systemctl enable rdma-qos.service
systemctl restart rdma-qos.service
echo -e "\e[31m Done \e[0m"
