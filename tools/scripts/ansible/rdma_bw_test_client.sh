#!/bin/bash
# RDMA 带宽测试 - 客户端脚本
# 环境变量:
#   EXCLUDE_NICS  - 要排除的网卡列表，用逗号分隔 (例如: eth0,eth1)
#   DURATION      - 测试持续时间，单位秒 (默认: 10)
#   COMMAND       - RDMA 测试命令，默认 ib_write_bw，可设置为 ib_read_bw/ib_write_lat/ib_read_lat
#
# 示例:
#   ./rdma_bw_test_client.sh 10.10.1.12 10.10.2.12 10.10.3.12
#   EXCLUDE_NICS=eth0,eth1 ./rdma_bw_test_client.sh 10.10.1.12
#   DURATION=20 ./rdma_bw_test_client.sh 10.10.1.12 ...  # 测试 20 秒
#   COMMAND=ib_write_lat ./rdma_bw_test_client.sh 10.10.1.12 ...  # 延时测试

set -e

# 检查参数
if [ "$#" -lt 1 ]; then
    echo "用法: $0 <ip1> <ip2> ... <ipN>"
    echo ""
    echo "示例 (测试 8 个网卡):"
    echo "  $0 10.10.1.12 10.10.17.12 10.10.33.12 10.10.49.12 10.10.65.12 10.10.81.12 10.10.97.12 10.10.113.12"
    echo ""
    echo "示例 (只测试部分网卡):"
    echo "  $0 10.10.1.12 10.10.17.12"
    echo ""
    exit 1
fi

# 将参数转为数组
DEST_IPS=("$@")
TARGET_NAME="对端节点 (${#DEST_IPS[@]} 个 IP)"

# 默认测试持续时间 (可通过环境变量覆盖)
DURATION="${DURATION:-10}"

# RDMA 测试命令: 默认 ib_write_bw，可通过环境变量 COMMAND 覆盖
COMMAND="${COMMAND:-ib_write_bw}"

echo "接收到 ${#DEST_IPS[@]} 个目标 IP:"
for i in "${!DEST_IPS[@]}"; do
    echo "  [$i] ${DEST_IPS[$i]}"
done
echo ""

# 获取 RDMA 设备列表
get_rdma_devices() {
    ls /sys/class/infiniband/ 2>/dev/null | grep "^mlx5_" | sort
}

# 从 RDMA 设备获取网卡名和 IP 地址
get_netdev_ip_from_rdma() {
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
                    echo "$netdev:$ip_addr"
                    return 0
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
    netdev_info=$(get_netdev_ip_from_rdma "$rdma_dev")
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

# 创建结果目录
RESULT_DIR="./rdma_results_$(date '+%Y%m%d_%H%M%S')"
mkdir -p "$RESULT_DIR"

echo "=== RDMA 带宽测试客户端 ==="
echo "本地节点: $(hostname) ($(hostname -I | awk '{print $1}'))"
echo "目标节点: $TARGET_NAME"
echo "结果目录: $RESULT_DIR"
echo ""

# 获取所有本地 RDMA 设备
LOCAL_RDMA_DEVS=($(get_rdma_devices))

if [ ${#LOCAL_RDMA_DEVS[@]} -eq 0 ]; then
    echo "错误: 未找到本地 RDMA 设备"
    exit 1
fi

# 显示配置信息
echo "测试配置:"
echo "  消息大小: $SIZE_HUMAN ($MESSAGE_SIZE 字节)"
echo "  测试命令: $COMMAND"
if [ -n "$EXCLUDE_NICS" ]; then
    echo "  排除网卡: $EXCLUDE_NICS"
fi
echo ""

echo "本地 RDMA 设备:"
ACTIVE_LOCAL_DEVS=()
for dev in "${LOCAL_RDMA_DEVS[@]}"; do
    netdev_info=$(get_netdev_ip_from_rdma "$dev")
    netdev=$(echo "$netdev_info" | cut -d: -f1)
    ip=$(echo "$netdev_info" | cut -d: -f2)
    
    if should_exclude_device "$dev"; then
        printf "  %-10s -> %-8s (%s) [已排除]\n" "$dev" "$netdev" "$ip"
    else
        printf "  %-10s -> %-8s (%s)\n" "$dev" "$netdev" "$ip"
        ACTIVE_LOCAL_DEVS+=("$dev")
    fi
done
echo ""

if [ ${#ACTIVE_LOCAL_DEVS[@]} -eq 0 ]; then
    echo "错误: 所有本地设备都被排除，没有可用的设备"
    exit 1
fi

echo "将用于测试的本地设备: ${#ACTIVE_LOCAL_DEVS[@]} 个"
echo ""

# 判断是带宽测试还是延时测试
IS_LATENCY_TEST=0

# 根据命令类型选择输出格式和测试参数
# 使用 -D 持续时间代替 -s/-n 参数
OPTS="-D $DURATION"
if [ "$COMMAND" = "ib_write_lat" ] || [ "$COMMAND" = "ib_read_lat" ]; then
    IS_LATENCY_TEST=1
    OPTS+=" --output=latency"
    
    SUMMARY_FILE="$RESULT_DIR/latency_summary.txt"
    echo "RDMA 延时测试结果汇总" > "$SUMMARY_FILE"
    echo "测试时间: $(date)" >> "$SUMMARY_FILE"
    echo "本地节点: $(hostname)" >> "$SUMMARY_FILE"
    echo "目标节点: $TARGET_NAME" >> "$SUMMARY_FILE"
    echo "测试持续时间: ${DURATION} 秒" >> "$SUMMARY_FILE"
    echo "测试命令: ${COMMAND} -d <rdma_device> <target_ip> ${OPTS}" >> "$SUMMARY_FILE"
    echo "" >> "$SUMMARY_FILE"
    printf "%-18s %-18s %-26s %-26s %-15s %s\n" "源RDMA设备" "源网卡" "源IP" "目的IP" "测试类型" "结果" >> "$SUMMARY_FILE"
    echo "----------------------------------------------------------------------------------------------------" >> "$SUMMARY_FILE"
else 
    OPTS+=" --output=bandwidth"

    SUMMARY_FILE="$RESULT_DIR/summary.txt"
    echo "RDMA 带宽测试结果汇总" > "$SUMMARY_FILE"
    echo "测试时间: $(date)" >> "$SUMMARY_FILE"
    echo "本地节点: $(hostname)" >> "$SUMMARY_FILE"
    echo "目标节点: $TARGET_NAME" >> "$SUMMARY_FILE"
    echo "测试持续时间: ${DURATION} 秒" >> "$SUMMARY_FILE"
    echo "测试命令: ${COMMAND} -d <rdma_device> <target_ip> ${OPTS}" >> "$SUMMARY_FILE"
    echo "" >> "$SUMMARY_FILE"
    printf "%-18s %-18s %-26s %-26s %-15s %s\n" "源RDMA设备" "源网卡" "源IP" "目的IP" "测试类型" "结果" >> "$SUMMARY_FILE"
    echo "----------------------------------------------------------------------------------------------------" >> "$SUMMARY_FILE"
fi
OPTS+=" --report_gbits"



# 开始测试
total_tests=$((${#ACTIVE_LOCAL_DEVS[@]} * ${#DEST_IPS[@]}))
current_test=0
success_count=0
fail_count=0

# 用于计算平均值的累加变量（区分同轨/跨轨）
total_bw=0
total_lat=0
valid_samples=0

# 同轨统计
total_bw_same=0
total_lat_same=0
valid_samples_same=0
success_count_same=0
fail_count_same=0

# 跨轨统计
total_bw_cross=0
total_lat_cross=0
valid_samples_cross=0
success_count_cross=0
fail_count_cross=0

echo "开始带宽测试 (${#ACTIVE_LOCAL_DEVS[@]}x${#DEST_IPS[@]} = $total_tests 次)..."
echo ""

for src_idx in "${!ACTIVE_LOCAL_DEVS[@]}"; do
    SRC_RDMA="${ACTIVE_LOCAL_DEVS[$src_idx]}"
    
    # 获取源设备信息
    netdev_info=$(get_netdev_ip_from_rdma "$SRC_RDMA")
    SRC_NETDEV=$(echo "$netdev_info" | cut -d: -f1)
    SRC_IP=$(echo "$netdev_info" | cut -d: -f2)
    
    echo "========================================"
    echo "源设备: $SRC_RDMA ($SRC_NETDEV - $SRC_IP)"
    echo "========================================"
    
    for dst_idx in "${!DEST_IPS[@]}"; do
        DEST_IP="${DEST_IPS[$dst_idx]}"
        DEST_NETDEV="ib$dst_idx"
        
        current_test=$((current_test + 1))
        
        # 判断是同轨还是跨轨
        if [ "$src_idx" -eq "$dst_idx" ]; then
            TEST_TYPE="同轨"
        else
            TEST_TYPE="跨轨"
        fi
        
        echo -n "[$current_test/$total_tests] $SRC_RDMA -> $DEST_IP ($DEST_NETDEV) [$TEST_TYPE] ... "

        # 打印本次测试将要执行的客户端命令
        CLIENT_CMD="$COMMAND -d $SRC_RDMA $DEST_IP $OPTS -F"
        echo ""
        echo "  客户端命令: $CLIENT_CMD"
        
        # 执行测试，支持最多3次重试
        MAX_RETRIES=3
        RETRY_COUNT=0
        TEST_SUCCESS=0
        
        while [ $RETRY_COUNT -lt $MAX_RETRIES ] && [ $TEST_SUCCESS -eq 0 ]; do
            TEMP_OUTPUT=$(mktemp)
            TEST_EXIT_CODE=0
            
            if [ $RETRY_COUNT -gt 0 ]; then
                echo "  重试 $RETRY_COUNT/$((MAX_RETRIES-1))..."
                sleep 2
            fi
            
            eval "$CLIENT_CMD" > "$TEMP_OUTPUT" 2>&1 || TEST_EXIT_CODE=$?
            
            if [ $TEST_EXIT_CODE -eq 0 ]; then
                TEST_SUCCESS=1
                
                if [ $IS_LATENCY_TEST -eq 1 ]; then
                    # 延时测试：提取延时结果 (usec)
                    LAT=$(grep -oP '\d+\.\d+' "$TEMP_OUTPUT" | tail -1 2>/dev/null || echo "N/A")
                    if [ "$LAT" != "N/A" ]; then
                        echo "✅ 成功 (延时: ${LAT} usec)"
                        RESULT="成功 (延时: ${LAT} usec)"
                        # 累加延时用于计算平均值
                        total_lat=$(awk "BEGIN {printf \"%.2f\", $total_lat + $LAT}")
                        valid_samples=$((valid_samples + 1))
                        # 区分同轨/跨轨统计
                        if [ "$TEST_TYPE" = "同轨" ]; then
                            total_lat_same=$(awk "BEGIN {printf \"%.2f\", $total_lat_same + $LAT}")
                            valid_samples_same=$((valid_samples_same + 1))
                        else
                            total_lat_cross=$(awk "BEGIN {printf \"%.2f\", $total_lat_cross + $LAT}")
                            valid_samples_cross=$((valid_samples_cross + 1))
                        fi
                    else
                        echo "✅ 成功"
                        RESULT="成功"
                    fi
                else
                    # 带宽测试：提取带宽结果 (Gb/s)
                    BW=$(grep -oP '\d+\.\d+' "$TEMP_OUTPUT" | tail -1 2>/dev/null || echo "N/A")
                    if [ "$BW" != "N/A" ]; then
                        BW_GBPS=$(awk "BEGIN {printf \"%.2f\", $BW}")
                        echo "✅ 成功 (${BW_GBPS} Gb/s)"
                        RESULT="成功 (${BW_GBPS} Gb/s)"
                        # 累加带宽用于计算平均值
                        total_bw=$(awk "BEGIN {printf \"%.2f\", $total_bw + $BW}")
                        valid_samples=$((valid_samples + 1))
                        # 区分同轨/跨轨统计
                        if [ "$TEST_TYPE" = "同轨" ]; then
                            total_bw_same=$(awk "BEGIN {printf \"%.2f\", $total_bw_same + $BW}")
                            valid_samples_same=$((valid_samples_same + 1))
                        else
                            total_bw_cross=$(awk "BEGIN {printf \"%.2f\", $total_bw_cross + $BW}")
                            valid_samples_cross=$((valid_samples_cross + 1))
                        fi
                    else
                        echo "✅ 成功 (${BW} Gb/s)"
                        RESULT="成功 (${BW} Gb/s)"
                    fi
                fi
                success_count=$((success_count + 1))
                # 区分同轨/跨轨成功计数
                if [ "$TEST_TYPE" = "同轨" ]; then
                    success_count_same=$((success_count_same + 1))
                else
                    success_count_cross=$((success_count_cross + 1))
                fi
            else
                RETRY_COUNT=$((RETRY_COUNT + 1))
                
                # 如果是最后一次重试仍然失败
                if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
                    # 测试失败，提取原始错误信息
                    if [ $TEST_EXIT_CODE -eq 124 ]; then
                        ERROR_MSG="超时 (30秒)"
                    else
                        # 提取最后几行非空错误信息
                        ERROR_MSG=$(grep -v "^$" "$TEMP_OUTPUT" | tail -3 | tr '\n' ' ' | cut -c1-200)
                        if [ -z "$ERROR_MSG" ]; then
                            ERROR_MSG="退出码: $TEST_EXIT_CODE"
                        fi
                    fi
                    
                    echo "❌ 失败 (重试 $((MAX_RETRIES)) 次后仍失败)"
                    echo "   错误: $ERROR_MSG"
                    RESULT="失败: $ERROR_MSG"
                    fail_count=$((fail_count + 1))
                    # 区分同轨/跨轨失败计数
                    if [ "$TEST_TYPE" = "同轨" ]; then
                        fail_count_same=$((fail_count_same + 1))
                    else
                        fail_count_cross=$((fail_count_cross + 1))
                    fi
                fi
            fi
            
            # 删除临时文件
            rm -f "$TEMP_OUTPUT"
        done
        
        # 记录到汇总文件
        printf "%-12s %-10s %-18s %-18s %-15s %s\n" \
            "$SRC_RDMA" "$SRC_NETDEV" "$SRC_IP" "$DEST_IP" "$TEST_TYPE" "$RESULT" >> "$SUMMARY_FILE"
        
        # 等待 1 秒，让服务端准备好下一次测试
        sleep 1
    done
    
    echo ""
done

# 输出测试统计
echo "========================================"
echo "测试完成统计"
echo "========================================"
echo "总测试数: $total_tests"
echo "成功: $success_count"
echo "失败: $fail_count"
echo "成功率: $(awk "BEGIN {printf \"%.2f\", ($success_count/$total_tests)*100}")%"
echo ""

# 计算并输出总体平均值
if [ $valid_samples -gt 0 ]; then
    if [ $IS_LATENCY_TEST -eq 1 ]; then
        avg_lat=$(awk "BEGIN {printf \"%.2f\", $total_lat / $valid_samples}")
        echo "总体平均延时: ${avg_lat} usec (基于 $valid_samples 个有效样本)"
    else
        avg_bw=$(awk "BEGIN {printf \"%.2f\", $total_bw / $valid_samples}")
        echo "总体平均带宽: ${avg_bw} Gb/s (基于 $valid_samples 个有效样本)"
    fi
fi

# 输出同轨统计
echo ""
echo "--- 同轨测试统计 ---"
total_same=$((success_count_same + fail_count_same))
if [ $total_same -gt 0 ]; then
    echo "测试数: $total_same (成功: $success_count_same, 失败: $fail_count_same)"
    if [ $valid_samples_same -gt 0 ]; then
        if [ $IS_LATENCY_TEST -eq 1 ]; then
            avg_lat_same=$(awk "BEGIN {printf \"%.2f\", $total_lat_same / $valid_samples_same}")
            echo "同轨平均延时: ${avg_lat_same} usec (基于 $valid_samples_same 个有效样本)"
        else
            avg_bw_same=$(awk "BEGIN {printf \"%.2f\", $total_bw_same / $valid_samples_same}")
            echo "同轨平均带宽: ${avg_bw_same} Gb/s (基于 $valid_samples_same 个有效样本)"
        fi
    fi
else
    echo "无同轨测试"
fi

# 输出跨轨统计
echo ""
echo "--- 跨轨测试统计 ---"
total_cross=$((success_count_cross + fail_count_cross))
if [ $total_cross -gt 0 ]; then
    echo "测试数: $total_cross (成功: $success_count_cross, 失败: $fail_count_cross)"
    if [ $valid_samples_cross -gt 0 ]; then
        if [ $IS_LATENCY_TEST -eq 1 ]; then
            avg_lat_cross=$(awk "BEGIN {printf \"%.2f\", $total_lat_cross / $valid_samples_cross}")
            echo "跨轨平均延时: ${avg_lat_cross} usec (基于 $valid_samples_cross 个有效样本)"
        else
            avg_bw_cross=$(awk "BEGIN {printf \"%.2f\", $total_bw_cross / $valid_samples_cross}")
            echo "跨轨平均带宽: ${avg_bw_cross} Gb/s (基于 $valid_samples_cross 个有效样本)"
        fi
    fi
else
    echo "无跨轨测试"
fi

echo ""
echo "结果已保存到: $SUMMARY_FILE"

# 追加统计到汇总文件
echo "" >> "$SUMMARY_FILE"
echo "=======================================" >> "$SUMMARY_FILE"
echo "测试统计" >> "$SUMMARY_FILE"
echo "=======================================" >> "$SUMMARY_FILE"
echo "总测试数: $total_tests" >> "$SUMMARY_FILE"
echo "成功: $success_count" >> "$SUMMARY_FILE"
echo "失败: $fail_count" >> "$SUMMARY_FILE"
echo "成功率: $(awk "BEGIN {printf \"%.2f\", ($success_count/$total_tests)*100}")%" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

# 追加总体平均值到汇总文件
if [ $valid_samples -gt 0 ]; then
    if [ $IS_LATENCY_TEST -eq 1 ]; then
        avg_lat=$(awk "BEGIN {printf \"%.2f\", $total_lat / $valid_samples}")
        echo "总体平均延时: ${avg_lat} usec (基于 $valid_samples 个有效样本)" >> "$SUMMARY_FILE"
    else
        avg_bw=$(awk "BEGIN {printf \"%.2f\", $total_bw / $valid_samples}")
        echo "总体平均带宽: ${avg_bw} Gb/s (基于 $valid_samples 个有效样本)" >> "$SUMMARY_FILE"
    fi
fi

# 追加同轨统计到汇总文件
echo "" >> "$SUMMARY_FILE"
echo "--- 同轨测试统计 ---" >> "$SUMMARY_FILE"
if [ $total_same -gt 0 ]; then
    echo "测试数: $total_same (成功: $success_count_same, 失败: $fail_count_same)" >> "$SUMMARY_FILE"
    if [ $valid_samples_same -gt 0 ]; then
        if [ $IS_LATENCY_TEST -eq 1 ]; then
            avg_lat_same=$(awk "BEGIN {printf \"%.2f\", $total_lat_same / $valid_samples_same}")
            echo "同轨平均延时: ${avg_lat_same} usec (基于 $valid_samples_same 个有效样本)" >> "$SUMMARY_FILE"
        else
            avg_bw_same=$(awk "BEGIN {printf \"%.2f\", $total_bw_same / $valid_samples_same}")
            echo "同轨平均带宽: ${avg_bw_same} Gb/s (基于 $valid_samples_same 个有效样本)" >> "$SUMMARY_FILE"
        fi
    fi
else
    echo "无同轨测试" >> "$SUMMARY_FILE"
fi

# 追加跨轨统计到汇总文件
echo "" >> "$SUMMARY_FILE"
echo "--- 跨轨测试统计 ---" >> "$SUMMARY_FILE"
if [ $total_cross -gt 0 ]; then
    echo "测试数: $total_cross (成功: $success_count_cross, 失败: $fail_count_cross)" >> "$SUMMARY_FILE"
    if [ $valid_samples_cross -gt 0 ]; then
        if [ $IS_LATENCY_TEST -eq 1 ]; then
            avg_lat_cross=$(awk "BEGIN {printf \"%.2f\", $total_lat_cross / $valid_samples_cross}")
            echo "跨轨平均延时: ${avg_lat_cross} usec (基于 $valid_samples_cross 个有效样本)" >> "$SUMMARY_FILE"
        else
            avg_bw_cross=$(awk "BEGIN {printf \"%.2f\", $total_bw_cross / $valid_samples_cross}")
            echo "跨轨平均带宽: ${avg_bw_cross} Gb/s (基于 $valid_samples_cross 个有效样本)" >> "$SUMMARY_FILE"
        fi
    fi
else
    echo "无跨轨测试" >> "$SUMMARY_FILE"
fi
