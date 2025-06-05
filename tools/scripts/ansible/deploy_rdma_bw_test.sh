#!/bin/bash
# 快速部署 RDMA 带宽测试脚本到目标节点

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 目标节点
NODE1="10.100.8.11"  # GPU 266
NODE2="10.100.8.12"  # GPU 267

echo "=== 部署 RDMA 带宽测试脚本 ==="
echo ""

# 检查脚本是否存在
if [ ! -f "$SCRIPT_DIR/rdma_bw_test_server.sh" ] || [ ! -f "$SCRIPT_DIR/rdma_bw_test_client.sh" ]; then
    echo "错误: 找不到测试脚本"
    exit 1
fi

echo "部署到节点:"
echo "  - $NODE1 (GPU 266)"
echo "  - $NODE2 (GPU 267)"
echo ""

# 部署到节点 1
echo "正在部署到 $NODE1 ..."
scp "$SCRIPT_DIR/rdma_bw_test_server.sh" "$SCRIPT_DIR/rdma_bw_test_client.sh" "root@$NODE1:/root/"
ssh "root@$NODE1" "chmod +x /root/rdma_bw_test_*.sh"
echo "✅ $NODE1 部署完成"

# 部署到节点 2
echo "正在部署到 $NODE2 ..."
scp "$SCRIPT_DIR/rdma_bw_test_server.sh" "$SCRIPT_DIR/rdma_bw_test_client.sh" "root@$NODE2:/root/"
ssh "root@$NODE2" "chmod +x /root/rdma_bw_test_*.sh"
echo "✅ $NODE2 部署完成"

echo ""
echo "=== 部署完成 ==="
echo ""
echo "开始测试:"
echo ""
echo "场景 1: GPU 266 -> GPU 267"
echo "  在 $NODE2 上: ./rdma_bw_test_server.sh"
echo "  在 $NODE1 上: ./rdma_bw_test_client.sh 12"
echo ""
echo "场景 2: GPU 267 -> GPU 266"
echo "  在 $NODE1 上: ./rdma_bw_test_server.sh"
echo "  在 $NODE2 上: ./rdma_bw_test_client.sh 11"
