#!/bin/bash
# 脚本名称: config_ib_ips.sh
# 功能: 批量配置 ib0-ib7 网卡的 IP 地址
# 用法: ./config_ib_ips.sh <ip1> <ip2> ... <ip8>

# 默认配置
DEFAULT_NETMASK=20
START_IB_INDEX=0

# 帮助函数
usage() {
    echo "用法: $0 <ip1> <ip2> ... <ip8>"
    echo "示例: $0 10.10.1.9 10.10.17.9 ... 10.10.113.9"
    echo ""
    echo "环境变量 (可选):"
    echo "  NETMASK: 子网掩码前缀长度 (默认: $DEFAULT_NETMASK)"
    echo "  START_DEV: 起始网卡编号 (默认: $START_IB_INDEX)"
    exit 1
}

# 获取掩码
NETMASK=${NETMASK:-$DEFAULT_NETMASK}
START_IDX=${START_DEV:-$START_IB_INDEX}

# 检查参数数量
if [ "$#" -ne 8 ]; then
    echo "错误: 需要输入 8 个 IP 地址，实际输入了 $# 个"
    usage
fi

# 将输入参数转为数组
IPS=("$@")
COUNT=${#IPS[@]}

echo "=== 开始配置 IB 网卡 (掩码: /$NETMASK) ==="

# 循环配置每个网卡
for (( i=0; i<COUNT; i++ )); do
    # 计算网卡名称，例如 ib0, ib1...
    DEV_IDX=$((START_IDX + i))
    DEV="ib${DEV_IDX}"
    IP="${IPS[$i]}"
    
    echo -n "正在配置 $DEV -> $IP ... "
    
    # 1. 检查网卡是否存在
    if ! ip link show "$DEV" >/dev/null 2>&1; then
        echo "[跳过] 网卡 $DEV 不存在"
        continue
    fi
    
    # 2. 启动网卡
    if ! ip link set "$DEV" up; then
        echo "[失败] 无法启动网卡"
        continue
    fi
    
    # 3. 清除旧 IP (避免累积多个 IP)
    ip addr flush dev "$DEV"
    
    # 4. 配置新 IP
    if ip addr add "$IP/$NETMASK" dev "$DEV"; then
        echo "[成功]"
    else
        echo "[失败] 无法设置 IP"
    fi
done

echo ""
echo "=== 配置结果概览 ==="
ip -br addr show | grep "^ib" | awk '{printf "%-8s %-30s %s\n", $1, $3, $2}'
