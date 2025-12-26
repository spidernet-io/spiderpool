#!/bin/bash
# RDMA 带宽测试 - 服务端脚本
# 用法: ./rdma_bw_test_server.sh [rdma_dev1 rdma_dev2 ...]
# 
# 环境变量:
#   EXCLUDE_NICS  - 要排除的网卡列表，用逗号分隔 (例如: eth0,eth1)
#   DURATION      - 测试持续时间，单位秒 (默认: 10)
#   COMMAND       - RDMA 测试命令，默认 ib_write_bw，可设置为 ib_read_bw/ib_write_lat/ib_read_lat
#
# 示例:
#   ./rdma_bw_test_server.sh mlx5_0 mlx5_1           # 只启动 mlx5_0 和 mlx5_1
#   ./rdma_bw_test_server.sh                         # 启动所有设备
#   EXCLUDE_NICS=eth0,eth1 ./rdma_bw_test_server.sh  # 排除 eth0 和 eth1 对应的设备
#   DURATION=20 ./rdma_bw_test_server.sh             # 测试 20 秒

set -e

# RDMA 测试命令: 默认 ib_write_bw，可通过环境变量 COMMAND 覆盖
COMMAND="${COMMAND:-ib_write_bw}"

# 默认测试持续时间 (可通过环境变量覆盖)
DURATION="${DURATION:-10}"

# 获取 RDMA 设备列表
get_rdma_devices() {
    ls /sys/class/infiniband/ 2>/dev/null | grep "^mlx5_"
}

# 从 RDMA 设备获取 IP 地址
get_ip_from_rdma_dev() {
    local rdma_dev="$1"
    local device_path
    local pci_addr
    local netdev
    local ip_addr
    
    # 通过 sysfs 获取 PCI 地址
    if [ -e "/sys/class/infiniband/$rdma_dev/device" ]; then
        device_path=$(realpath "/sys/class/infiniband/$rdma_dev/device")
        pci_addr=$(basename "$device_path")
        
        # 查找对应的网络接口
        for net in /sys/class/net/*; do
            if [ -e "$net/device" ]; then
                net_pci=$(basename "$(realpath "$net/device")")
                if [ "$net_pci" = "$pci_addr" ]; then
                    netdev=$(basename "$net")
                    # 获取 IPv4 地址
                    ip_addr=$(ip -4 addr show "$netdev" 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | head -1)
                    if [ -n "$ip_addr" ]; then
                        echo "$netdev:$ip_addr"
                        return 0
                    fi
                fi
            fi
        done
    fi
    
    echo "unknown:N/A"
    return 1
}

# 检查网卡是否在排除列表中
# 输入: RDMA 设备名 (如 mlx5_0)
# 返回: 0 表示应该排除，1 表示不排除
should_exclude_device() {
    local rdma_dev="$1"
    
    # 如果没有设置 EXCLUDE_NICS，不排除任何设备
    if [ -z "$EXCLUDE_NICS" ]; then
        return 1
    fi
    
    # 获取该 RDMA 设备对应的网卡名
    local netdev_info
    netdev_info=$(get_ip_from_rdma_dev "$rdma_dev")
    local netdev
    netdev=$(echo "$netdev_info" | cut -d: -f1)
    
    # 将 EXCLUDE_NICS 按逗号分割
    IFS=',' read -ra EXCLUDE_LIST <<< "$EXCLUDE_NICS"
    
    # 检查网卡名是否在排除列表中
    for exclude_nic in "${EXCLUDE_LIST[@]}"; do
        # 去除空格
        exclude_nic=$(echo "$exclude_nic" | xargs)
        if [ "$netdev" = "$exclude_nic" ]; then
            return 0  # 应该排除
        fi
    done
    
    return 1  # 不排除
}

echo "=== RDMA 带宽测试服务端 ==="
echo "节点信息: $(hostname) ($(hostname -I | awk '{print $1}'))"
echo "测试命令: $COMMAND"
echo ""

# 判断是使用参数指定的设备还是所有设备
if [ "$#" -gt 0 ]; then
    # 使用命令行参数指定的设备
    RDMA_DEVS=("$@")
    echo "使用指定的 RDMA 设备: ${RDMA_DEVS[*]}"
    
    # 验证设备是否存在
    for dev in "${RDMA_DEVS[@]}"; do
        if [ ! -e "/sys/class/infiniband/$dev" ]; then
            echo "错误: RDMA 设备 $dev 不存在"
            exit 1
        fi
    done
else
    # 获取所有 RDMA 设备
    RDMA_DEVS=($(get_rdma_devices))
    
    if [ ${#RDMA_DEVS[@]} -eq 0 ]; then
        echo "错误: 未找到 RDMA 设备"
        exit 1
    fi
    
    echo "使用所有 RDMA 设备"
fi

echo ""

# 显示排除信息
if [ -n "$EXCLUDE_NICS" ]; then
    echo "排除的网卡: $EXCLUDE_NICS"
    echo ""
fi

echo "发现 ${#RDMA_DEVS[@]} 个 RDMA 设备:"
ACTIVE_DEVS=()
for dev in "${RDMA_DEVS[@]}"; do
    netdev_info=$(get_ip_from_rdma_dev "$dev")
    netdev=$(echo "$netdev_info" | cut -d: -f1)
    ip=$(echo "$netdev_info" | cut -d: -f2)
    
    if should_exclude_device "$dev"; then
        printf "  %-10s -> %-8s (%s) [已排除]\n" "$dev" "$netdev" "$ip"
    else
        printf "  %-10s -> %-8s (%s)\n" "$dev" "$netdev" "$ip"
        ACTIVE_DEVS+=("$dev")
    fi
done
echo ""

if [ ${#ACTIVE_DEVS[@]} -eq 0 ]; then
    echo "错误: 所有设备都被排除，没有可用的设备"
    exit 1
fi

echo "将启动服务的设备: ${#ACTIVE_DEVS[@]} 个"
echo ""

# 每个设备需要启动 8 次服务（因为每次客户端连接后服务端会退出）
TESTS_PER_DEVICE=8

echo "开始启动服务端 (每个设备启动 $TESTS_PER_DEVICE 次)..."
echo "按 Ctrl+C 停止"
echo ""

# 循环启动服务（只启动未被排除的设备）
for rdma_dev in "${ACTIVE_DEVS[@]}"; do
    netdev_info=$(get_ip_from_rdma_dev "$rdma_dev")
    netdev=$(echo "$netdev_info" | cut -d: -f1)
    ip=$(echo "$netdev_info" | cut -d: -f2)
    
    echo "----------------------------------------"
    echo "设备: $rdma_dev ($netdev - $ip)"
    echo "----------------------------------------"
    
    for i in $(seq 1 $TESTS_PER_DEVICE); do
        echo "[$(date '+%H:%M:%S')] 启动第 $i/$TESTS_PER_DEVICE 次服务..."
        
        # 根据命令类型选择输出格式和测试参数
        # 使用 -D 持续时间代替 -s/-n 参数
        OPTS="-D $DURATION"
        if [ "$COMMAND" = "ib_write_lat" ] || [ "$COMMAND" = "ib_read_lat" ]; then
            OPTS+=" --output=latency"
        else 
            OPTS+=" --output=bandwidth"
        fi
        OPTS+=" --report_gbits"
        
        # 打印本次测试将要执行的服务端命令
        SERVER_CMD="$COMMAND -d $rdma_dev $OPTS -F"
        echo "  服务端命令: $SERVER_CMD"

        # 启动 RDMA 测试服务端
        if eval "$SERVER_CMD" 2>&1; then
            echo "[$(date '+%H:%M:%S')] 第 $i 次测试完成"
        else
            echo "[$(date '+%H:%M:%S')] 第 $i 次测试失败或被中断"
        fi
        
        # 等待 1 秒再启动下一次
        if [ $i -lt $TESTS_PER_DEVICE ]; then
            sleep 1
        fi
    done
    
    echo ""
done

echo "=== 所有服务端测试完成 ==="
