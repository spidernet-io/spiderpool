
#!/bin/bash

# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# Script to automatically set RDMA mode for all Mellanox/NVIDIA ConnectX cards
# Usage:   RDMA_CIDR="172.16.0.0/16"    ./setRdmaRule.sh

TABLE="main"
# must be bigger than 1000 used by spiderpool
PRIORITY=2000
RDMA_CIDR=${RDMA_CIDR:-"172.16.0.0/16"}

# 创建systemd服务文件
sudo tee /etc/systemd/system/rdma-routing.service > /dev/null <<EOF
[Unit]
Description=Custom IP Routing Rules
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/bash -c 'ip rule add not to ${RDMA_CIDR} table ${TABLE} priority ${PRIORITY}'
ExecStop=/bin/bash -c 'ip rule del not to ${RDMA_CIDR} table ${TABLE} priority ${PRIORITY} 2>/dev/null || true'

[Install]
WantedBy=multi-user.target
EOF

# 重新加载systemd配置
systemctl daemon-reload

# 启用并启动服务
systemctl enable rdma-routing.service
systemctl start rdma-routing.service

# 验证服务状态
echo "服务状态："
systemctl status rdma-routing.service

echo -e "\n当前路由规则："
ip rule list

