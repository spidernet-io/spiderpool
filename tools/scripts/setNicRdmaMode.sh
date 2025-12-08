#!/bin/bash

# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# Script to automatically set RDMA mode for all Mellanox/NVIDIA ConnectX cards
# 
# Scenario 1: Configure all NICs with the same mode (no GPU topology filtering)
# Usage:   RDMA_MODE="roce"    ./setNicRdmaMode.sh
# Usage:   RDMA_MODE="infiniband"   ./setNicRdmaMode.sh
#
# Scenario 2: Configure GPU NICs and other NICs with different modes
# Usage:   GPU_RDMA_MODE="infiniband" OTHER_RDMA_MODE="roce" ./setNicRdmaMode.sh
# Usage:   GPU_RDMA_MODE="infiniband" ./setNicRdmaMode.sh  # Only configure GPU NICs
#
# Query current configuration:
# Usage:    ./setNicRdmaMode.sh q

#set -e
#set -o xtrace
set -o pipefail
set -o errexit

# Function for logging
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Initialize variables
SUCCESS="true"
NEED_REBOOT="false"
LINK_TYPE=""
CHANGED_DEVICES=""

# Function to convert RDMA mode to mlxconfig format
convert_mode_to_link_type() {
    local mode="$1"
    if [ "$mode" = "roce" ]; then
        echo "ETH"
    elif [ "$mode" = "infiniband" ]; then
        echo "IB"
    else
        echo ""
    fi
}

# Validate RDMA_MODE environment variables
if [ "$1" != "q" ] ; then
    # Determine configuration scenario
    if [ -n "$GPU_RDMA_MODE" ] || [ -n "$OTHER_RDMA_MODE" ]; then
        # Scenario 2: Separate configuration for GPU and other NICs
        log "Configuration mode: Separate GPU and other NICs"
        
        if [ -n "$GPU_RDMA_MODE" ]; then
            if [ "$GPU_RDMA_MODE" != "roce" ] && [ "$GPU_RDMA_MODE" != "infiniband" ]; then
                log "ERROR: Invalid GPU_RDMA_MODE value. Must be 'roce' or 'infiniband'"
                exit 1
            fi
            GPU_LINK_TYPE=$(convert_mode_to_link_type "$GPU_RDMA_MODE")
            log "GPU NICs will be configured to: $GPU_RDMA_MODE mode"
        fi
        
        if [ -n "$OTHER_RDMA_MODE" ]; then
            if [ "$OTHER_RDMA_MODE" != "roce" ] && [ "$OTHER_RDMA_MODE" != "infiniband" ]; then
                log "ERROR: Invalid OTHER_RDMA_MODE value. Must be 'roce' or 'infiniband'"
                exit 1
            fi
            OTHER_LINK_TYPE=$(convert_mode_to_link_type "$OTHER_RDMA_MODE")
            log "Other NICs will be configured to: $OTHER_RDMA_MODE mode"
        fi
        
        if [ -z "$GPU_RDMA_MODE" ] && [ -z "$OTHER_RDMA_MODE" ]; then
            log "ERROR: At least one of GPU_RDMA_MODE or OTHER_RDMA_MODE must be set"
            exit 1
        fi
        
        SEPARATE_CONFIG="true"
    elif [ -n "$RDMA_MODE" ]; then
        # Scenario 1: Unified configuration for all NICs
        log "Configuration mode: Unified mode for all NICs"
        
        if [ "$RDMA_MODE" != "roce" ] && [ "$RDMA_MODE" != "infiniband" ]; then
            log "ERROR: Invalid RDMA_MODE value. Must be 'roce' or 'infiniband'"
            exit 1
        fi
        
        LINK_TYPE=$(convert_mode_to_link_type "$RDMA_MODE")
        log "All NICs will be configured to: $RDMA_MODE mode"
        SEPARATE_CONFIG="false"
    else
        log "ERROR: No RDMA mode specified"
        log "Scenario 1 - Configured All RDMA Nics: RDMA_MODE='roce' or RDMA_MODE='infiniband'"
        log "Scenario 2 - Configured Separate RDMA Nics: GPU_RDMA_MODE='infiniband' OTHER_RDMA_MODE='roce'"
        exit 1
    fi
else 
    echo "query the configuration"
fi 


# Check for required tools
for cmd in mst mlxconfig lspci grep ethtool; do
    if ! command_exists $cmd; then
        log "ERROR: Required command '$cmd' not found"
        exit 1
    fi
done

# Check for additional dependencies if separate configuration is enabled
if [ "$SEPARATE_CONFIG" = "true" ]; then
    log "GPU topology detection enabled - checking additional dependencies..."
    for cmd in nvidia-smi rdma; do
        if ! command_exists $cmd; then
            log "ERROR: GPU topology detection requires '$cmd' command"
            exit 1
        fi
    done
fi

# Start MST service
if systemctl start mst 2>/dev/null || mst start ; then
    log "MST service started successfully"
else
    log "ERROR: Failed to start MST service"
    exit 1
fi

# Wait for MST service to be ready
sleep 2

# Function to get PCI address from RDMA device name
# Input: RDMA device name (e.g., mlx5_0)
# Output: PCI address (e.g., 0000:17:00.0)
get_pci_from_rdma_dev() {
    local rdma_dev="$1"
    local pci_addr
    local device_path
    
    # Method 1: Direct sysfs lookup (most reliable)
    # /sys/class/infiniband/mlx5_0/device -> /sys/class/devices/pci0000:00/0000:00:01.0/0000:17:00.0
    if [ -e "/sys/class/infiniband/$rdma_dev/device" ]; then
        device_path=$(realpath "/sys/class/infiniband/$rdma_dev/device")
        pci_addr=$(basename "$device_path")
        echo "$pci_addr"
        return 0
    fi
        
    return 1
}

# Function to get RDMA device name from PCI address
# Input: PCI address (e.g., 0000:17:00.0)
# Output: RDMA device name (e.g., mlx5_0)
get_rdma_dev_from_pci() {
    local pci_addr="$1"
    local short_pci
    local dev_pci
    local device_link
    
    # Remove domain prefix if present (0000:05:00.0 -> 05:00.0)
    short_pci=$(echo "$pci_addr" | sed 's/^[0-9]*://')
    
    # Find RDMA device by PCI address
    for rdma_dev in /sys/class/infiniband/*; do
        if [ -e "$rdma_dev/device" ]; then
            device_link=$(readlink -f "$rdma_dev/device")
            dev_pci=$(basename "$device_link")
            if [ "$dev_pci" = "$short_pci" ] || [ "$dev_pci" = "$pci_addr" ]; then
                basename "$rdma_dev"
                return 0
            fi
        fi
    done
    return 1
}

# Function to display device mapping information
# Input: space-separated list of PCI addresses
display_device_mapping() {
    local devices="$1"
    local rdma_dev
    local nic_name
    
    for pci in $devices; do
        rdma_dev=$(get_rdma_dev_from_pci "$pci")
        if [ -n "$rdma_dev" ]; then
            # Try to get network interface name from RDMA device (only available in RoCE/Ethernet mode)
            nic_name=$(rdma link 2>/dev/null | grep "^link $rdma_dev/" | grep -o 'netdev [^ ]*' | awk '{print $2}' | head -n1)
            if [ -n "$nic_name" ]; then
                log "  $pci -> $rdma_dev -> $nic_name" >&2
            else
                # InfiniBand mode - no netdev available
                log "  $pci -> $rdma_dev (IB mode)" >&2
            fi
        else
            log "  $pci (RDMA device not found)" >&2
        fi
    done
}

# Function to filter devices based on GPU topology
filter_devices_by_gpu_topology() {        
    # Get NICs with PIX/PXB topology from nvidia-smi
    local gpu_nics
    gpu_nics=$(nvidia-smi topo -m 2>/dev/null | grep -E '^NIC[0-9]+' | grep -E '(PIX|PXB)' | awk '{print $1}')
    
    if [ -z "$gpu_nics" ]; then
        log "WARNING: No NICs with PIX/PXB GPU topology found" >&2
        return 1
    fi
    
    # Get the mapping of NIC names to RDMA devices from nvidia-smi output
    local nic_legend
    nic_legend=$(nvidia-smi topo -m 2>/dev/null | grep -A 100 "NIC Legend:" | grep "NIC[0-9]*:")
    
    if [ -z "$nic_legend" ]; then
        log "WARNING: Could not parse NIC legend from nvidia-smi topo output" >&2
        return 1
    fi
    
    log "Found NICs with PIX/PXB GPU topology:" >&2
    
    # Build filtered device list by converting RDMA device names to PCI addresses
    local filtered_devices=""
    local rdma_dev
    local pci_addr
    
    for nic in $gpu_nics; do
        # Get RDMA device name from NIC legend (e.g., NIC0 -> mlx5_0)
        rdma_dev=$(echo "$nic_legend" | grep "$nic:" | awk '{print $2}')
        
        if [ -z "$rdma_dev" ]; then
            log "WARNING: Could not find RDMA device for $nic" >&2
            continue
        fi
        
        # Get PCI address from RDMA device (mlx5_0 -> rdma link -> netdev -> ethtool -> PCI)
        pci_addr=$(get_pci_from_rdma_dev "$rdma_dev")
        
        if [ -z "$pci_addr" ]; then
            log "WARNING: Could not find PCI address for RDMA device $rdma_dev" >&2
            continue
        fi
        
        # Normalize PCI address format (add domain if missing)
        if [[ ! "$pci_addr" =~ ^[0-9a-f]{4}: ]]; then
            pci_addr="0000:$pci_addr"
        fi
        
        filtered_devices="$filtered_devices $pci_addr"
        log "  $nic -> $rdma_dev -> $pci_addr" >&2
    done
    
    if [ -z "$filtered_devices" ]; then
        log "WARNING: No devices matched GPU topology filter" >&2
        return 1
    fi
    
    echo "$filtered_devices"
}

# Get PF list of Mellanox/NVIDIA devices
ALL_DEVICES=$(lspci -D | grep -i "Mellanox\|NVIDIA" | grep -i "ConnectX" | grep -v "Virtual Function" | cut -d' ' -f1)

if [ -z "$ALL_DEVICES" ]; then
    log "ERROR: No Mellanox/NVIDIA ConnectX devices found"
    exit 1
fi

# Separate devices into GPU and other NICs if needed
if [ "$SEPARATE_CONFIG" = "true" ]; then
    # Always detect GPU devices in separate config mode to properly identify "other" NICs
    log "Detecting GPU NICs with PIX/PXB topology..."
    GPU_DEVICES=$(filter_devices_by_gpu_topology)
    if [ -z "$GPU_DEVICES" ]; then
        log "WARNING: No GPU NICs detected with PIX/PXB topology"
        GPU_DEVICES=""
    fi
    
    # Identify other (non-GPU) NICs
    if [ -n "$OTHER_RDMA_MODE" ]; then
        log "Identifying other (non-GPU) NICs..."
        OTHER_DEVICES=""
        for dev in $ALL_DEVICES; do
            is_gpu="false"
            for gpu_dev in $GPU_DEVICES; do
                if [ "$dev" = "$gpu_dev" ]; then
                    is_gpu="true"
                    break
                fi
            done
            if [ "$is_gpu" = "false" ]; then
                OTHER_DEVICES="$OTHER_DEVICES $dev"
            fi
        done
        
        if [ -z "$OTHER_DEVICES" ]; then
            log "WARNING: No other (non-GPU) NICs found"
            OTHER_DEVICES=""
        else
            log "Found $(echo $OTHER_DEVICES | wc -w) other (non-GPU) NICs:"
            display_device_mapping "$OTHER_DEVICES"
        fi
    fi
else
    # Unified configuration - all devices use the same mode
    log "Configuring all $(echo $ALL_DEVICES | wc -w) Mellanox/NVIDIA NICs"
    DEVICES="$ALL_DEVICES"
fi

# Function to configure a device with specified mode
configure_device() {
    local dev="$1"
    local target_link_type="$2"
    local mode_name="$3"
    
    log "---------------------- Configuring PF device $dev to $mode_name mode..."
    
    # Check if device supports mode switching
    if ! mlxconfig -d $dev q &>/dev/null; then
        log "WARNING: Device $dev does not support configuration or cannot be accessed"
        SUCCESS="false"
        return 1
    fi
    
    # Get current configuration to check available parameters
    local config
    config=$(mlxconfig -d $dev q)
    
    # Set LINK_TYPE for available ports
    if echo "$config" | grep -q "LINK_TYPE_P1"; then
        local current_mode_p1
        current_mode_p1=$(echo "$config" | grep "LINK_TYPE_P1" | awk '{print $NF}')
        if [[ ! "$current_mode_p1" =~ "$target_link_type" ]]; then
            log "Port 1 current mode: $current_mode_p1, target mode: $target_link_type"
            if ! mlxconfig -d $dev -y set LINK_TYPE_P1=$target_link_type; then
                log "ERROR: Failed to set mode for device $dev port 1"
                SUCCESS="false"
                return 1
            else
                echo "> mlxconfig -d $dev -y set LINK_TYPE_P1=$target_link_type"
            fi
            log "Configured port 1 of device $dev to $mode_name mode"
            NEED_REBOOT="true"
            # Add device to changed list for firmware reset
            if [[ ! "$CHANGED_DEVICES" =~ "$dev" ]]; then
                CHANGED_DEVICES="$CHANGED_DEVICES $dev"
            fi
        else
            log "Port 1 of device $dev already in $mode_name mode"
        fi
    fi
    
    if echo "$config" | grep -q "LINK_TYPE_P2"; then
        local current_mode_p2
        current_mode_p2=$(echo "$config" | grep "LINK_TYPE_P2" | awk '{print $NF}')
        if [[ ! "$current_mode_p2" =~ "$target_link_type" ]]; then
            log "Port 2 current mode: $current_mode_p2, target mode: $target_link_type"
            if ! mlxconfig -d $dev -y set LINK_TYPE_P2=$target_link_type; then
                log "ERROR: Failed to set mode for device $dev port 2"
                SUCCESS="false"
                return 1
            else
                echo "> mlxconfig -d $dev -y set LINK_TYPE_P2=$target_link_type"
            fi
            log "Configured port 2 of device $dev to $mode_name mode"
            NEED_REBOOT="true"
            # Add device to changed list for firmware reset
            if [[ ! "$CHANGED_DEVICES" =~ "$dev" ]]; then
                CHANGED_DEVICES="$CHANGED_DEVICES $dev"
            fi
        else
            log "Port 2 of device $dev already in $mode_name mode"
        fi
    fi
    
    log "Successfully checked/configured device $dev"
    return 0
}

# Display current firmware configuration
if [ "$SEPARATE_CONFIG" = "true" ]; then
    if [ -n "$GPU_DEVICES" ]; then
        echo ""
        echo "======== GPU NICs (PIX/PXB topology) ========"
        for dev in $GPU_DEVICES; do
            echo "-------- show firmware configuration for device $dev --------"
            mlxconfig -d $dev q | grep "LINK_TYPE" 
        done
    fi
    
    if [ -n "$OTHER_DEVICES" ]; then
        echo ""
        echo "======== Other NICs (non-GPU) ========"
        for dev in $OTHER_DEVICES; do
            echo "-------- show firmware configuration for device $dev --------"
            mlxconfig -d $dev q | grep "LINK_TYPE" 
        done
    fi
else
    echo ""
    echo "======== All NICs ========"
    for dev in $DEVICES; do
        echo "-------- show firmware configuration for device $dev --------"
        mlxconfig -d $dev q | grep "LINK_TYPE" 
    done
fi

if [ "$1" == "q" ] ; then
    ALL_RDMA_DEV=$( rdma link | awk '{print $2}' | awk -F'/' '{print $1}' )
    echo ""
    echo "======== RDMA Link Layer Status ========"
    for dev in $ALL_RDMA_DEV; do
        echo "-------- show configuration in use for device $dev --------"
        ibstat $dev | grep "Link layer"
    done

	echo ""
	echo "finish querying the configuration"
	exit 0
fi

# Configure devices based on scenario
if [ "$SEPARATE_CONFIG" = "true" ]; then
    # Scenario 2: Separate configuration for GPU and other NICs
    if [ -n "$GPU_RDMA_MODE" ] && [ -n "$GPU_DEVICES" ]; then
        log "========================================"
        log "Configuring GPU NICs to $GPU_RDMA_MODE mode"
        log "========================================"
        for dev in $GPU_DEVICES; do
            configure_device "$dev" "$GPU_LINK_TYPE" "$GPU_RDMA_MODE"
        done
    fi
    
    if [ -n "$OTHER_RDMA_MODE" ] && [ -n "$OTHER_DEVICES" ]; then
        log "========================================"
        log "Configuring other NICs to $OTHER_RDMA_MODE mode"
        log "========================================"
        for dev in $OTHER_DEVICES; do
            configure_device "$dev" "$OTHER_LINK_TYPE" "$OTHER_RDMA_MODE"
        done
    fi
else
    # Scenario 1: Unified configuration for all NICs
    log "========================================"
    log "Configuring all NICs to $RDMA_MODE mode"
    log "========================================"
    for dev in $DEVICES; do
        configure_device "$dev" "$LINK_TYPE" "$RDMA_MODE"
    done
fi

# Perform firmware reset for changed devices
if [ -n "$CHANGED_DEVICES" ]; then
    log "Performing firmware reset for changed devices..."
    for dev in $CHANGED_DEVICES; do
        log "Resetting firmware for device $dev..."
        if mlxfwreset -d $dev reset -y 2>&1; then
            log "Firmware reset completed for device $dev"
        else
            log "WARNING: Firmware reset failed for device $dev. Cold reboot (power cycle) may be required."
        fi
    done
    # Wait for devices to recover
    log "Waiting for devices to recover after firmware reset..."
    sleep 30
fi

# Display final firmware configuration
echo ""
echo "======== Final Firmware Configuration ========"
if [ "$SEPARATE_CONFIG" = "true" ]; then
    if [ -n "$GPU_DEVICES" ]; then
        echo ""
        echo "--- GPU NICs ---"
        for dev in $GPU_DEVICES; do
            echo "Device $dev:"
            mlxconfig -d $dev q | grep "LINK_TYPE" 
        done
    fi
    
    if [ -n "$OTHER_DEVICES" ]; then
        echo ""
        echo "--- Other NICs ---"
        for dev in $OTHER_DEVICES; do
            echo "Device $dev:"
            mlxconfig -d $dev q | grep "LINK_TYPE" 
        done
    fi
else
    for dev in $DEVICES; do
        echo "Device $dev:"
        mlxconfig -d $dev q | grep "LINK_TYPE" 
    done
fi
echo ""

if [ "$SUCCESS" = "false" ]; then
    log "WARNING: Some devices could not be configured. Please check the logs above."
    exit 1
fi

if [ "$NEED_REBOOT" = "true" ]; then
    log "Configuration changes were made and firmware reset was attempted."
    log ""
    log "=== IMPORTANT ==="
    log "Please verify the NIC mode has been switched correctly by running:"
    log "    $0 q"
    log ""
    log "If the NIC mode is still not switched after firmware reset:"
    log "    1. A COLD REBOOT (power cycle) is required - normal reboot may not work"
    log "    2. Completely power off the server (not just reboot)"
    log "    3. Wait a few seconds, then power on"
    log "    4. Verify the mode again after boot"
    log "================="
else
    log "All devices are already in expected mode. No changes needed."
fi
