#!/bin/bash

# Script to automatically set RDMA mode for all Mellanox/NVIDIA ConnectX cards
# Usage:   RDMA_MODE="roce"    ./auto-set-mode.sh
# Usage:   RDMA_MODE="infiniband"   ./auto-set-mode.sh
# Usage:    ./auto-set-mode.sh q    #q uery the configuration

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


# Validate RDMA_MODE environment variable
if [ "$1" != "q" ] ; then
    if [ -z "$RDMA_MODE" ]; then
        log "ERROR: RDMA_MODE environment variable is not set"
        log "Usage: RDMA_MODE='roce' or RDMA_MODE='infiniband' $0"
        exit 1
    fi
    if [ "$RDMA_MODE" != "roce" ] && [ "$RDMA_MODE" != "infiniband" ]; then
        log "ERROR: Invalid RDMA_MODE value. Must be 'roce' or 'infiniband'"
        exit 1
    fi

    # Convert RDMA_MODE to mlxconfig format
    if [ "$RDMA_MODE" = "roce" ]; then
        LINK_TYPE="ETH"
    else
        LINK_TYPE="IB"
    fi
else 
    echo "query the configuration"
fi 


# Check for required tools
for cmd in mst mlxconfig lspci grep; do
    if ! command_exists $cmd; then
        log "ERROR: Required command '$cmd' not found"
        exit 1
    fi
done

# Start MST service
if systemctl start mst 2>/dev/null || mst start ; then
    log "MST service started successfully"
else
    log "ERROR: Failed to start MST service"
    exit 1
fi

# Wait for MST service to be ready
sleep 2

# Get list of Mellanox/NVIDIA devices
log "Scanning for Mellanox/NVIDIA devices..."
DEVICES=$(lspci -D | grep -i "Mellanox\|NVIDIA" | grep -i "ConnectX" | cut -d' ' -f1)

if [ -z "$DEVICES" ]; then
    log "ERROR: No Mellanox/NVIDIA ConnectX devices found"
    exit 1
fi

# Process each device
for dev in $DEVICES; do
    echo "-------- show firmware configuration for device $dev --------"
    mlxconfig -d $dev q | grep "LINK_TYPE" 
done

if [ "$1" == "q" ] ; then
    ALL_RDMA_DEV=$( rdma link | awk '{print $2}' | awk -F'/' '{print $1}' )
    for dev in $ALL_RDMA_DEV; do
        echo "-------- show configuration in use for device $dev --------"
        ibstat $dev | grep "Link layer"
    done

	echo "finish querying the configuration"
	exit 0
fi

# Process each device
for dev in $DEVICES; do
    log "---------------------- Configuring device $dev..."
    
    # Check if device supports mode switching
    if ! mlxconfig -d $dev q &>/dev/null; then
        log "WARNING: Device $dev does not support configuration or cannot be accessed"
        SUCCESS="false"
        continue
    fi
    
    # Get current configuration to check available parameters
    CONFIG=$(mlxconfig -d $dev q)
    
    # Set LINK_TYPE for available ports
    if echo "$CONFIG" | grep -q "LINK_TYPE_P1"; then
        CURRENT_MODE_P1=$(echo "$CONFIG" | grep "LINK_TYPE_P1" | awk '{print $NF}' )
        if [[ ! "$CURRENT_MODE_P1" =~ "$LINK_TYPE" ]]; then
            log "Port 1 current mode: $CURRENT_MODE_P1, target mode: $LINK_TYPE"
            if ! mlxconfig -d $dev -y set LINK_TYPE_P1=$LINK_TYPE; then
                log "ERROR: Failed to set mode for device $dev port 1"
                SUCCESS="false"
                continue
            else
                echo "> mlxconfig -d $dev -y set LINK_TYPE_P1=$LINK_TYPE"
            fi
            log "Configured port 1 of device $dev to $RDMA_MODE mode"
            NEED_REBOOT="true"
        else
            log "Port 1 of device $dev already in $RDMA_MODE mode"
        fi
    fi
    
    if echo "$CONFIG" | grep -q "LINK_TYPE_P2"; then
        CURRENT_MODE_P2=$(echo "$CONFIG" | grep "LINK_TYPE_P2" | awk '{print $NF}')
        if [[ ! "$CURRENT_MODE_P2" =~ "$LINK_TYPE" ]]; then
            log "Port 2 current mode: $CURRENT_MODE_P2, target mode: $LINK_TYPE"
            if ! mlxconfig -d $dev -y set LINK_TYPE_P2=$LINK_TYPE; then
                log "ERROR: Failed to set mode for device $dev port 2"
                SUCCESS="false"
                continue
            else
                echo "> mlxconfig -d $dev -y set LINK_TYPE_P2=$LINK_TYPE"
            fi
            log "Configured port 2 of device $dev to $RDMA_MODE mode"
            NEED_REBOOT="true"
        else
            log "Port 2 of device $dev already in $RDMA_MODE mode"
        fi
    fi
    
    log "Successfully checked/configured device $dev"
done

echo ""
# Process each device
for dev in $DEVICES; do
    echo "-------- show firmware configuration for device $dev --------"
    mlxconfig -d $dev q | grep "LINK_TYPE" 
done
echo ""

if [ "$SUCCESS" = "false" ]; then
    log "WARNING: Some devices could not be configured. Please check the logs above."
    exit 1
fi

if [ "$NEED_REBOOT" = "true" ]; then
    log "Configuration changes were made. A reboot is required to apply changes."
else
    log "All devices are already in $RDMA_MODE mode. No changes needed."
fi
