package dra

// import (
// 	"fmt"
// 	"strings"
// 	"sync"

// 	"github.com/vishvananda/netlink"
// 	"k8s.io/klog/v2"
// )
// // 创建网络监控器
// monitor := NewNetworkMonitor()

// // 启动监控
// err := monitor.Start()
// if err != nil {
// 	// 处理错误
// 	panic(err)
// }

// // 获取更新通道
// updates := monitor.Updates()

// // 在您的代码中监听更新
// go func() {
// 	for event := range updates {
// 		// 处理网卡变化事件
// 		fmt.Printf("网卡 %s 被%s\n", event.Interface,
// 			event.Type == "add" ? "添加" : "移除")
// 	}
// }()

// // 当不再需要监控时停止
// // monitor.Stop()// 创建网络监控器
// monitor := NewNetworkMonitor()

// // 启动监控
// err := monitor.Start()
// if err != nil {
// 	// 处理错误
// 	panic(err)
// }

// // 获取更新通道
// updates := monitor.Updates()

// // 在您的代码中监听更新
// go func() {
// 	for event := range updates {
// 		// 处理网卡变化事件
// 		fmt.Printf("网卡 %s 被%s\n", event.Interface,
// 			event.Type == "add" ? "添加" : "移除")
// 	}
// }()

// // 当不再需要监控时停止
// // monitor.Stop()package dra

// // NetworkChangeEvent represents a network interface change event
// type NetworkChangeEvent struct {
// 	Type      string // "add" or "remove"
// 	Interface string
// }

// // NetworkMonitor monitors physical network interface changes
// type NetworkMonitor struct {
// 	updates    chan NetworkChangeEvent
// 	stop       chan struct{}
// 	wg         sync.WaitGroup
// 	interfaces map[string]bool
// 	mu         sync.RWMutex
// }

// // NewNetworkMonitor creates a new NetworkMonitor instance
// func NewNetworkMonitor() *NetworkMonitor {
// 	return &NetworkMonitor{
// 		updates:    make(chan NetworkChangeEvent, 10),
// 		stop:       make(chan struct{}),
// 		interfaces: make(map[string]bool),
// 	}
// }

// // isPhysicalInterface checks if the interface is a physical network interface
// func isPhysicalInterface(link netlink.Link) bool {
// 	name := link.Attrs().Name

// 	// Skip loopback
// 	if name == "lo" {
// 		return false
// 	}

// 	// Skip virtual interfaces (common prefixes)
// 	skipPrefixes := []string{
// 		"veth", // Virtual ethernet
// 		"vxlan", // VXLAN interfaces
// 		"docker", // Docker interfaces
// 		"cni",  // CNI interfaces
// 		"flannel", // Flannel interfaces
// 		"calico", // Calico interfaces
// 		"virbr", // Virtual bridge
// 		"vmnet", // VMware interfaces
// 		"br-",   // Bridge interfaces
// 		"tun",   // TUN interfaces
// 		"tap",   // TAP interfaces
// 	}

// 	for _, prefix := range skipPrefixes {
// 		if strings.HasPrefix(name, prefix) {
// 			return false
// 		}
// 	}

// 	// Check if it's a virtual interface by type
// 	switch link.Type() {
// 	case "veth", "bridge", "vxlan", "vcan", "dummy", "ifb", "macvlan", "macvtap", "ipvlan", "vlan":
// 		return false
// 	}

// 	return true
// }

// // Start begins monitoring network interface changes
// func (nm *NetworkMonitor) Start() error {
// 	// Get initial interfaces
// 	links, err := netlink.LinkList()
// 	if err != nil {
// 		return fmt.Errorf("failed to list interfaces: %v", err)
// 	}

// 	nm.mu.Lock()
// 	for _, link := range links {
// 		if isPhysicalInterface(link) {
// 			nm.interfaces[link.Attrs().Name] = true
// 		}
// 	}
// 	nm.mu.Unlock()

// 	nm.wg.Add(1)
// 	go nm.monitor()

// 	return nil
// }

// // Stop stops the network monitoring
// func (nm *NetworkMonitor) Stop() {
// 	close(nm.stop)
// 	nm.wg.Wait()
// 	close(nm.updates)
// }

// // Updates returns the channel for network interface updates
// func (nm *NetworkMonitor) Updates() <-chan NetworkChangeEvent {
// 	return nm.updates
// }
