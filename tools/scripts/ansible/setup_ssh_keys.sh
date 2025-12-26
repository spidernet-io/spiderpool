#!/bin/bash
# SSH 免密登录批量检查和配置脚本
# 用法: ./setup_ssh_keys.sh [inventory_file] [check|setup]

set -e

INVENTORY_FILE="${1:-inventory.ini}"
ACTION="${2:-check}"
SSH_USER="${3:-toor}"
SSH_PASS="${4:-root@123}"
SSH_KEY="$HOME/.ssh/id_rsa"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_ok() { echo -e "${GREEN}[OK]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_info() { echo -e "[INFO] $1"; }

# 从 inventory 文件提取主机列表
get_hosts() {
    if [ ! -f "$INVENTORY_FILE" ]; then
        echo "错误: 找不到 inventory 文件: $INVENTORY_FILE" >&2
        exit 1
    fi
    # 只提取 IP 地址（跳过注释、组名和变量定义）
    grep -oE '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' "$INVENTORY_FILE" | \
        sort -u
}

# 检查本地 SSH 密钥
check_local_key() {
    log_info "检查本地 SSH 密钥..."
    if [ -f "$SSH_KEY" ]; then
        log_ok "本地密钥存在: $SSH_KEY"
        return 0
    else
        log_warn "本地密钥不存在: $SSH_KEY"
        return 1
    fi
}

# 生成本地 SSH 密钥
generate_local_key() {
    log_info "生成本地 SSH 密钥..."
    if [ -f "$SSH_KEY" ]; then
        log_ok "密钥已存在，跳过生成"
        return 0
    fi
    ssh-keygen -t rsa -b 4096 -f "$SSH_KEY" -N "" -q
    log_ok "密钥生成完成: $SSH_KEY"
}

# 检查单个主机的免密状态
check_host() {
    local host="$1"
    local timeout=5
    
    # 尝试免密登录
    if ssh -o BatchMode=yes -o ConnectTimeout=$timeout -o StrictHostKeyChecking=no "${SSH_USER}@${host}" "exit" 2>/dev/null; then
        log_ok "$host - 免密登录正常 (用户: $SSH_USER)"
        return 0
    else
        log_fail "$host - 免密登录失败 (用户: $SSH_USER)"
        return 1
    fi
}

# 配置单个主机的免密登录
setup_host() {
    local host="$1"
    local timeout=10
    
    # 先检查是否已经可以免密登录
    if ssh -o BatchMode=yes -o ConnectTimeout=$timeout -o StrictHostKeyChecking=no "${SSH_USER}@${host}" "exit" 2>/dev/null; then
        log_ok "$host - 已配置免密登录 (用户: $SSH_USER)"
        return 0
    fi
    
    log_info "$host - 配置免密登录 (用户: $SSH_USER)..."
    
    # 使用 sshpass + ssh-copy-id 复制公钥
    if command -v sshpass &>/dev/null; then
        if sshpass -p "$SSH_PASS" ssh-copy-id -o StrictHostKeyChecking=no -o ConnectTimeout=$timeout "${SSH_USER}@${host}" 2>/dev/null; then
            log_ok "$host - 免密登录配置成功"
            return 0
        else
            log_fail "$host - 免密登录配置失败"
            return 1
        fi
    else
        log_warn "sshpass 未安装，请先安装: apt install sshpass 或 yum install sshpass"
        log_info "尝试手动方式 (需要输入密码: $SSH_PASS)..."
        if ssh-copy-id -o StrictHostKeyChecking=no -o ConnectTimeout=$timeout "${SSH_USER}@${host}"; then
            log_ok "$host - 免密登录配置成功"
            return 0
        else
            log_fail "$host - 免密登录配置失败"
            return 1
        fi
    fi
}

# 批量检查
batch_check() {
    log_info "========== 批量检查 SSH 免密登录 =========="
    local hosts
    hosts=$(get_hosts)
    local total=0
    local success=0
    local failed=0
    local failed_hosts=""
    
    check_local_key
    echo ""
    
    for host in $hosts; do
        total=$((total + 1))
        if check_host "$host"; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
            failed_hosts="$failed_hosts $host"
        fi
    done
    
    echo ""
    log_info "========== 检查结果 =========="
    log_info "总计: $total 台主机"
    log_ok "成功: $success 台"
    [ $failed -gt 0 ] && log_fail "失败: $failed 台"
    
    if [ $failed -gt 0 ]; then
        echo ""
        log_warn "失败的主机列表:"
        for h in $failed_hosts; do
            log_warn "  - $h"
        done
    fi
    
    return $failed
}

# 批量配置
batch_setup() {
    log_info "========== 批量配置 SSH 免密登录 =========="
    local hosts
    hosts=$(get_hosts)
    local total=0
    local success=0
    local failed=0
    local failed_hosts=""
    
    # 确保本地密钥存在
    generate_local_key
    echo ""
    
    for host in $hosts; do
        total=$((total + 1))
        if setup_host "$host"; then
            success=$((success + 1))
        else
            failed=$((failed + 1))
            failed_hosts="$failed_hosts $host"
        fi
    done
    
    echo ""
    log_info "========== 配置结果 =========="
    log_info "总计: $total 台主机"
    log_ok "成功: $success 台"
    [ $failed -gt 0 ] && log_fail "失败: $failed 台"
    
    if [ $failed -gt 0 ]; then
        echo ""
        log_warn "失败的主机列表:"
        for h in $failed_hosts; do
            log_warn "  - $h"
        done
        echo ""
        log_warn "请手动执行: ssh-copy-id ${SSH_USER}@<host>"
    fi
    
    return $failed
}

# 配置节点间互相免密 (mesh)
setup_mesh() {
    log_info "========== 配置节点间互相免密登录 =========="
    local hosts
    hosts=$(get_hosts)
    local host_array=($hosts)
    local total=${#host_array[@]}
    local success=0
    local failed=0
    local failed_pairs=""
    
    # 检查 sshpass
    if ! command -v sshpass &>/dev/null; then
        log_fail "sshpass 未安装，请先安装: apt install sshpass 或 yum install sshpass"
        return 1
    fi
    
    log_info "共 $total 台主机，需要配置 $((total * (total - 1))) 对免密关系"
    echo ""
    
    for src_host in "${host_array[@]}"; do
        log_info "配置 $src_host 到其他节点的免密..."
        
        # 先在源节点生成密钥（如果不存在）
        sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "${SSH_USER}@${src_host}" \
            "[ -f ~/.ssh/id_rsa ] || ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N '' -q" 2>/dev/null
        
        # 获取源节点的公钥
        local src_pubkey
        src_pubkey=$(sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "${SSH_USER}@${src_host}" "cat ~/.ssh/id_rsa.pub" 2>/dev/null)
        
        if [ -z "$src_pubkey" ]; then
            log_fail "$src_host - 无法获取公钥"
            failed=$((failed + 1))
            continue
        fi
        
        for dst_host in "${host_array[@]}"; do
            [ "$src_host" = "$dst_host" ] && continue
            
            # 检查是否已经可以免密
            if sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "${SSH_USER}@${src_host}" \
                "ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${SSH_USER}@${dst_host} exit" 2>/dev/null; then
                log_ok "  $src_host -> $dst_host (已配置)"
                success=$((success + 1))
            else
                # 将源节点公钥添加到目标节点
                if sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "${SSH_USER}@${dst_host}" \
                    "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '$src_pubkey' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && sort -u ~/.ssh/authorized_keys -o ~/.ssh/authorized_keys" 2>/dev/null; then
                    log_ok "  $src_host -> $dst_host (新配置)"
                    success=$((success + 1))
                else
                    log_fail "  $src_host -> $dst_host (失败)"
                    failed=$((failed + 1))
                    failed_pairs="$failed_pairs\n  $src_host -> $dst_host"
                fi
            fi
        done
    done
    
    echo ""
    log_info "========== 配置结果 =========="
    log_info "总计: $((total * (total - 1))) 对免密关系"
    log_ok "成功: $success 对"
    [ $failed -gt 0 ] && log_fail "失败: $failed 对"
    
    if [ $failed -gt 0 ]; then
        echo ""
        log_warn "失败的配对:"
        echo -e "$failed_pairs"
    fi
    
    return $failed
}

# 检查节点间互相免密状态
check_mesh() {
    log_info "========== 检查节点间互相免密登录 =========="
    local hosts
    hosts=$(get_hosts)
    local host_array=($hosts)
    local total=${#host_array[@]}
    local success=0
    local failed=0
    local failed_pairs=""
    
    # 检查 sshpass
    if ! command -v sshpass &>/dev/null; then
        log_fail "sshpass 未安装，请先安装: apt install sshpass 或 yum install sshpass"
        return 1
    fi
    
    log_info "共 $total 台主机，检查 $((total * (total - 1))) 对免密关系"
    echo ""
    
    for src_host in "${host_array[@]}"; do
        for dst_host in "${host_array[@]}"; do
            [ "$src_host" = "$dst_host" ] && continue
            
            # 从控制节点 SSH 到源节点，再从源节点 SSH 到目标节点
            if sshpass -p "$SSH_PASS" ssh -o StrictHostKeyChecking=no "${SSH_USER}@${src_host}" \
                "ssh -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=no ${SSH_USER}@${dst_host} exit" 2>/dev/null; then
                log_ok "$src_host -> $dst_host"
                success=$((success + 1))
            else
                log_fail "$src_host -> $dst_host"
                failed=$((failed + 1))
                failed_pairs="$failed_pairs\n  $src_host -> $dst_host"
            fi
        done
    done
    
    echo ""
    log_info "========== 检查结果 =========="
    log_info "总计: $((total * (total - 1))) 对免密关系"
    log_ok "成功: $success 对"
    [ $failed -gt 0 ] && log_fail "失败: $failed 对"
    
    if [ $failed -gt 0 ]; then
        echo ""
        log_warn "失败的配对:"
        echo -e "$failed_pairs"
    fi
    
    return $failed
}

# 显示帮助
show_help() {
    echo "SSH 免密登录批量检查和配置脚本"
    echo ""
    echo "用法: $0 [inventory_file] [action] [user] [password]"
    echo ""
    echo "参数:"
    echo "  inventory_file  Ansible inventory 文件路径 (默认: inventory.ini)"
    echo "  action          操作类型:"
    echo "                    check      - 检查控制节点到各主机的免密状态 (默认)"
    echo "                    setup      - 配置控制节点到各主机的免密登录"
    echo "                    check-mesh - 检查所有节点间的互相免密状态"
    echo "                    setup-mesh - 配置所有节点间的互相免密登录"
    echo "                    help       - 显示帮助"
    echo "  user            SSH 用户名 (默认: toor)"
    echo "  password        SSH 密码 (默认: root@123)"
    echo ""
    echo "示例:"
    echo "  $0 inventory.ini check              # 检查控制节点免密状态"
    echo "  $0 inventory.ini setup              # 配置控制节点免密"
    echo "  $0 inventory.ini check-mesh         # 检查节点间互相免密"
    echo "  $0 inventory.ini setup-mesh         # 配置节点间互相免密 (MPI需要)"
}

# 主函数
main() {
    case "$ACTION" in
        check)
            batch_check
            ;;
        setup)
            batch_setup
            ;;
        check-mesh)
            check_mesh
            ;;
        setup-mesh)
            setup_mesh
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo "未知操作: $ACTION"
            show_help
            exit 1
            ;;
    esac
}

main
