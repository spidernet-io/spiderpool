// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/exec"
	testexec "k8s.io/utils/exec/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/podownercache"
)

// Label(K00002)

type FakeCache struct {
	IPToPodMap map[string]podownercache.Pod
}

func (f *FakeCache) GetPodByIP(ip string) *podownercache.Pod {
	if val, ok := f.IPToPodMap[ip]; ok {
		return &val
	}
	return nil
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	meter := noop.NewMeterProvider().Meter("test")

	err := Register(ctx, meter, &FakeCache{IPToPodMap: map[string]podownercache.Pod{}})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReRegisterMetrics(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("test")

	log := logutils.Logger.Named("rdma-metrics-exporter")
	e := &exporter{
		observableMap:         make(map[string]metric.Int64ObservableCounter),
		meter:                 meter,
		log:                   log,
		ch:                    make(chan struct{}, 10),
		waitToRegisterMetrics: make(map[string]struct{}),
	}
	err := e.registerMetrics(meter)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = e.reRegisterMetrics()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	log.Info("rdma metrics registered")
}

func TestCamelToSnake(t *testing.T) {
	tests := map[string]string{
		"camelCase":  "camel_case",
		"PascalCase": "pascal_case",
		"snake_case": "snake_case",
		"HTTPServer": "http_server",
		"noChange":   "no_change",
	}

	for input, expected := range tests {
		output := camelToSnake(input)
		if output != expected {
			t.Errorf("Expected %s but got %s", expected, output)
		}
	}
}

func TestGetDefaultIP(t *testing.T) {
	tests := []struct {
		name          string
		commandScript []testexec.FakeCommandAction
		expectedIP    string
		expectError   bool
	}{
		{
			name: "IPv4",
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					if cmd == "ip" && len(args) > 0 && args[0] == "route" && args[1] == "get" && args[2] == "1.0.0.0" {
						fakeCmd.CombinedOutputScript = []testexec.FakeAction{
							func() ([]byte, []byte, error) {
								return []byte("1.0.0.0 via 10.6.0.1 dev ens160 src 10.6.1.21 uid 1000\n    cache"), nil, nil
							},
						}
					}
					return fakeCmd
				},
			},
			expectedIP:  "10.6.1.21",
			expectError: false,
		},
		{
			name: "IPv6",
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("xxx"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("2001:4860:4860::8888 from :: via fd00::1 dev ens160 proto static src fd00::21 metric 1024 pref medium"), nil, nil
						},
					}
					return fakeCmd
				},
			},
			expectedIP:  "fd00::21",
			expectError: false,
		},
		{
			name: "IPv4 and IPv6",
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("1.0.0.0 via 10.6.0.1 dev ens160 src 10.6.1.21 uid 1000\n    cache"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("2001:4860:4860::8888 from :: via fd00::1 dev ens160 proto static src fd00::21 metric 1024 pref medium"), nil, nil
						},
					}
					return fakeCmd
				},
			},
			expectedIP:  "10.6.1.21",
			expectError: false,
		},
		{
			name: "Neither IPv4 nor IPv6",
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte(""), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte(""), nil, nil
						},
					}
					return fakeCmd
				},
			},
			expectedIP:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeExec := &testexec.FakeExec{
				CommandScript: tt.commandScript,
			}

			ip, err := getDefaultIP(fakeExec)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if ip != tt.expectedIP {
					t.Errorf("Expected IP %s but got %s", tt.expectedIP, ip)
				}
			}
		})
	}
}

func TestListNodeNetNS(t *testing.T) {
	tests := []struct {
		name                        string
		mode                        string
		rdmaSystemGetNetnsModeError bool
		dirEntries                  []os.DirEntry
		readDirErr                  error
		expected                    []string
		expectError                 bool
	}{
		{
			name:                        "read system rdma mode error",
			rdmaSystemGetNetnsModeError: true,
			expectError:                 true,
		},
		{
			name:        "exclusive mode with entries",
			mode:        "exclusive",
			dirEntries:  []os.DirEntry{mockDirEntry("netns1"), mockDirEntry("netnsimpl")},
			expected:    []string{"netns1", "netnsimpl"},
			expectError: false,
		},
		{
			name:        "non-exclusive mode",
			mode:        "non-exclusive",
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "readDir error",
			mode:        "exclusive",
			dirEntries:  []os.DirEntry{mockDirEntry("netns1"), mockDirEntry("netnsimpl")},
			readDirErr:  errors.New("mock error"),
			expectError: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			rdmaSystemGetNetnsMode = func() (string, error) {
				if tt.rdmaSystemGetNetnsModeError {
					return "", errors.New("mock error")
				}
				return tt.mode, nil
			}

			// patch os.ReadDir
			readDir = func(name string) ([]os.DirEntry, error) {
				t.Logf("test name: %s ,tt.readdiderr: %v \n", tt.name, tt.readDirErr)
				return tt.dirEntries, tt.readDirErr
			}

			result, err := listNodeNetNS()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			}

			if !reflect.DeepEqual(tt.expected, result) {
				t.Errorf("Expected %v but got %v", tt.expected, result)
			}
			time.Sleep(100 * time.Millisecond)
			runtime.GC()
		})
	}
}

// Helper function to create a mock DirEntry
func mockDirEntry(name string) os.DirEntry {
	return &mockDirEntryStruct{name: name}
}

type mockDirEntryStruct struct {
	name string
}

func (m *mockDirEntryStruct) Name() string               { return m.name }
func (m *mockDirEntryStruct) IsDir() bool                { return false }
func (m *mockDirEntryStruct) Type() os.FileMode          { return 0 }
func (m *mockDirEntryStruct) Info() (os.FileInfo, error) { return nil, nil }

func TestGetIPToPodMap(t *testing.T) {
	tests := []struct {
		name        string
		pods        []client.Object
		expectedMap map[string]types.NamespacedName
		expectError bool
	}{
		{
			name: "Single Pod with IP",
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1", Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{IP: "192.168.1.1"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod2", Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{IP: "192.168.1.2"},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod3", Namespace: "test",
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{IP: "192.168.1.3"},
							{IP: ""},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod4", Namespace: "test",
					},
					Spec: corev1.PodSpec{
						HostNetwork: true,
					},
					Status: corev1.PodStatus{
						PodIPs: []corev1.PodIP{
							{IP: "10.6.1.21"},
						},
					},
				},
			},
			expectedMap: map[string]types.NamespacedName{
				"192.168.1.1": {Namespace: "default", Name: "pod1"},
				"192.168.1.2": {Namespace: "default", Name: "pod2"},
				"192.168.1.3": {Namespace: "test", Name: "pod3"},
			},
			expectError: false,
		},
		{
			name: "Pod with no IP",
			pods: []client.Object{
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod3", Namespace: "default"},
					Status:     corev1.PodStatus{},
				},
			},
			expectedMap: map[string]types.NamespacedName{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create a fake client with the provided pods
			scheme := kruntime.NewScheme()
			err := corev1.AddToScheme(scheme)
			if err != nil {
				t.Fatalf("Failed to add corev1 to scheme: %v", err)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.pods...).Build()

			// call the function
			result, err := getIPToPodMap(context.Background(), cli)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(tt.expectedMap, result) {
					t.Errorf("Expected %v but got %v", tt.expectedMap, result)
				}
			}
		})
	}
}

func TestGetIPToPodMap_WithMockClient(t *testing.T) {
	ctx := context.Background()
	scheme := kruntime.NewScheme()
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	_, err := getIPToPodMap(ctx, cli)
	if err == nil {
		t.Errorf("expected an error, got nil")
	}
}

func TestGetPodAttributes(t *testing.T) {
	tests := []struct {
		name              string
		input             types.NamespacedName
		expectedNamespace *attribute.KeyValue
		expectedName      *attribute.KeyValue
	}{
		{
			name: "Both namespace and name are set",
			input: types.NamespacedName{
				Namespace: "default",
				Name:      "pod1",
			},
			expectedNamespace: &attribute.KeyValue{Key: "pod_namespace", Value: attribute.StringValue("default")},
			expectedName:      &attribute.KeyValue{Key: "pod_name", Value: attribute.StringValue("pod1")},
		},
		{
			name: "Only namespace is set",
			input: types.NamespacedName{
				Namespace: "default",
			},
			expectedNamespace: &attribute.KeyValue{Key: "pod_namespace", Value: attribute.StringValue("default")},
			expectedName:      nil,
		},
		{
			name: "Only name is set",
			input: types.NamespacedName{
				Name: "pod1",
			},
			expectedNamespace: nil,
			expectedName:      &attribute.KeyValue{Key: "pod_name", Value: attribute.StringValue("pod1")},
		},
		{
			name:              "Neither namespace nor name is set",
			input:             types.NamespacedName{},
			expectedNamespace: nil,
			expectedName:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attributeNamespace, attributeName := getPodAttributes(tt.input)
			if attributeNamespace != nil && tt.expectedNamespace != nil {
				if *attributeNamespace != *tt.expectedNamespace {
					t.Errorf("Expected namespace %v, but got %v", *tt.expectedNamespace, *attributeNamespace)
				}
			} else if attributeNamespace != tt.expectedNamespace {
				t.Errorf("Expected namespace %v, but got %v", tt.expectedNamespace, attributeNamespace)
			}

			if attributeName != nil && tt.expectedName != nil {
				if *attributeName != *tt.expectedName {
					t.Errorf("Expected name %v, but got %v", *tt.expectedName, *attributeName)
				}
			} else if attributeName != tt.expectedName {
				t.Errorf("Expected name %v, but got %v", tt.expectedName, attributeName)
			}
		})
	}
}

func TestGetIdentifyAttributes(t *testing.T) {
	tests := []struct {
		name     string
		stats    map[string]interface{}
		expCount int
	}{
		{
			name: "Valid port and ifname",
			stats: map[string]interface{}{
				"port":             float64(1),
				"ifname":           "eth0",
				"net_dev_name":     "net1",
				"is_root":          true,
				"node_guid":        "1d:c9:d1:fe:ff:ac:36:ae",
				"sys_image_guid":   "b6:65:05:0c:9c:5c:f6:08",
				"rdma_parent_name": "ib1",
			},
			expCount: 7,
		},
		{
			name: "Missing port",
			stats: map[string]interface{}{
				"ifname": "eth1",
			},

			expCount: 1,
		},
		{
			name: "Missing ifname",
			stats: map[string]interface{}{
				"port": float64(2),
			},
			expCount: 1,
		},
		{
			name:     "Empty stats",
			stats:    map[string]interface{}{},
			expCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := getIdentifyAttributes(tt.stats)
			if len(list) != tt.expCount {
				t.Errorf("Expected %d attributes, but got %d", tt.expCount, len(list))
			}
		})
	}
}

func TestObserve(t *testing.T) {
	tests := []struct {
		name       string
		counter    metric.Int64ObservableCounter
		value      int
		attributes []*attribute.KeyValue
		expected   []attribute.KeyValue
	}{
		{
			name:     "No attributes",
			counter:  mustNewInt64ObservableCounter(noop.NewMeterProvider().Meter("test").Int64ObservableCounter("test_counter")),
			value:    10,
			expected: []attribute.KeyValue{},
		},
		{
			name:    "Single attribute",
			counter: mustNewInt64ObservableCounter(noop.NewMeterProvider().Meter("test").Int64ObservableCounter("test_counter")),
			value:   20,
			attributes: []*attribute.KeyValue{
				ptr(attribute.String("key1", "value1")),
			},
			expected: []attribute.KeyValue{
				attribute.String("key1", "value1"),
			},
		},
		{
			name:    "Multiple attributes",
			counter: mustNewInt64ObservableCounter(noop.NewMeterProvider().Meter("test").Int64ObservableCounter("test_counter")),
			value:   30,
			attributes: []*attribute.KeyValue{
				ptr(attribute.String("key1", "value1")),
				ptr(attribute.Int("key2", 2)),
			},
			expected: []attribute.KeyValue{
				attribute.String("key1", "value1"),
				attribute.Int("key2", 2),
			},
		},
		{
			name:    "Nil attribute",
			counter: mustNewInt64ObservableCounter(noop.NewMeterProvider().Meter("test").Int64ObservableCounter("test_counter")),
			value:   40,
			attributes: []*attribute.KeyValue{
				nil,
				ptr(attribute.String("key1", "value1")),
			},
			expected: []attribute.KeyValue{
				attribute.String("key1", "value1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observe(noop.Observer{}, tt.counter, tt.value, tt.attributes...)
		})
	}
}

func mustNewInt64ObservableCounter(counter metric.Int64ObservableCounter, err error) metric.Int64ObservableCounter {
	if err != nil {
		panic(err)
	}
	return counter
}

func ptr(kv attribute.KeyValue) *attribute.KeyValue {
	return &kv
}

func TestGetRDMAStats(t *testing.T) {
	tests := []struct {
		name          string
		nsID          string
		mockNetnsCli  func(netnsID string, toRun func() error) error
		nl            NetlinkImpl
		commandScript []testexec.FakeCommandAction
		expectedIP    string
		expectedStats string
		expectedErr   bool
		ethtoolImpl   EthtoolImpl
	}{
		{
			name: "get net get from path should return error",
			nsID: "test",
			mockNetnsCli: func(netnsID string, toRun func() error) error {
				return errors.New("mock error")
			},
			nl: NetlinkImpl{
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					return nil, nil
				},
				LinkList: nil,
			},
			expectedErr: true,
		},
		{
			name: "get pod rdma stats error",
			nsID: "test",
			mockNetnsCli: func(netnsID string, toRun func() error) error {
				return toRun()
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return nil, nil, errors.New("mock error")
						},
					}
					return fakeCmd
				},
			},

			expectedErr: true,
		},

		{
			name: "get pod rdma stats error with unmarshal json",
			nsID: "test",
			mockNetnsCli: func(netnsID string, toRun func() error) error {
				return toRun()
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("{\"rxWriteRequests\": 100}"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("src 1.1.1.1"), nil, nil
						},
					}
					return fakeCmd
				},
			},

			expectedErr: true,
		},

		{
			name: "get pod RDMA stats success",
			nsID: "test",
			ethtoolImpl: EthtoolImpl{Stats: func(netIfName string) (map[string]uint64, error) {
				return map[string]uint64{
					"rx_vport_rdma_unicast_bytes": 100,
					"tx_vport_rdma_unicast_bytes": 100,
				}, nil
			}},
			nl: NetlinkImpl{
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					rdmaList := []*netlink.RdmaLink{
						{Attrs: netlink.RdmaLinkAttrs{
							Name:     "mlx5_34",
							NodeGuid: "1d:c9:d1:fe:ff:ac:36:ae",
						}},
						{Attrs: netlink.RdmaLinkAttrs{
							Name:         "mlx5_6",
							NodeGuid:     "b6:65:05:0c:9c:5c:f6:08",
							SysImageGuid: "b6:65:05:0c:9c:5c:f6:00",
						}},
					}
					return rdmaList, nil
				},
				LinkList: func() ([]netlink.Link, error) {
					linkList := []netlink.Link{
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: ""}},
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "mock-empty-addr", HardwareAddr: nil}},
						&netlink.Device{LinkAttrs: netlink.LinkAttrs{
							Name: "enp5s0f0v6",
							HardwareAddr: func() net.HardwareAddr {
								mac, _ := net.ParseMAC("ae:36:ac:d1:c9:1d")
								return mac
							}(),
						}},
						&netlink.IPoIB{LinkAttrs: netlink.LinkAttrs{
							Name: "ibp13s0v7",
							HardwareAddr: func() net.HardwareAddr {
								mac, _ := net.ParseMAC("00:00:00:68:fe:80:00:00:00:00:00:00:08:f6:5c:9c:0c:05:65:b6")
								return mac
							}(),
						}},
					}
					return linkList, nil
				},
			},
			mockNetnsCli: func(netnsID string, toRun func() error) error {
				return toRun()
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("[{\"rxWriteRequests\": 100,\"ifname\": \"mlx5_34\"}]"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("src 192.168.1.1"), nil, nil
						},
					}
					return fakeCmd
				},
			},
			expectedIP:    "192.168.1.1",
			expectedErr:   false,
			expectedStats: "[{\"ifname\":\"mlx5_34\",\"is_root\":false,\"net_dev_name\":\"enp5s0f0v6\",\"node_guid\":\"1d:c9:d1:fe:ff:ac:36:ae\",\"rx_vport_rdma_unicast_bytes\":100,\"rx_write_requests\":100,\"sys_image_guid\":\"\",\"tx_vport_rdma_unicast_bytes\":100}]",
		},

		{
			name: "get default ip return error",
			nsID: "test",
			nl: NetlinkImpl{
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					return nil, nil
				},
				LinkList: func() ([]netlink.Link, error) {
					return nil, nil
				},
			},
			mockNetnsCli: func(netnsID string, toRun func() error) error {
				return toRun()
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("[{\"rxWriteRequests\": 100}]"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return nil, nil, errors.New("mock error")
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return nil, nil, errors.New("mock error")
						},
					}
					return fakeCmd
				},
			},
			expectedIP:    "192.168.1.1",
			expectedErr:   true,
			expectedStats: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeExec := &testexec.FakeExec{CommandScript: tt.commandScript}
			guidMapNetDeviceName := map[string]string{
				"b6:65:05:0c:9c:5c:f6:00": "ib1",
			}
			ip, stats, err := getRDMAStats(tt.nsID, tt.mockNetnsCli, guidMapNetDeviceName, tt.nl, fakeExec, tt.ethtoolImpl)
			if (err != nil) != tt.expectedErr {
				t.Errorf("expected error: %v, but got: %v", tt.expectedErr, err)
			}
			if tt.expectedErr {
				return
			}
			if ip != tt.expectedIP {
				t.Errorf("expected IP %v, but got %v", tt.expectedIP, ip)
			}

			got, err := json.Marshal(stats)
			if err != nil {
				t.Fatal(err)
			}
			if !compareMaps(string(got), tt.expectedStats) {
				raw, _ := json.Marshal(stats)
				t.Errorf("expected stats %v, but got %v", tt.expectedStats, string(raw))
			}
		})
	}
}

// compareMaps compares two slices of maps for equality
func compareMaps(got, exp string) bool {
	expList := make([]map[string]interface{}, 0)
	err := json.Unmarshal([]byte(exp), &expList)
	if err != nil {
		panic(err)
	}

	gotList := make([]map[string]interface{}, 0)
	err = json.Unmarshal([]byte(got), &gotList)
	if err != nil {
		panic(err)
	}

	if len(gotList) != len(expList) {
		return false
	}
	for i := range gotList {
		for key, value := range gotList[i] {
			if expList[i][key] != value {
				return false
			}
		}
		for key, value := range expList[i] {
			if gotList[i][key] != value {
				return false
			}
		}
	}
	return true
}

func TestProcessStats(t *testing.T) {
	stats := map[string]interface{}{
		"port":              float64(1),
		"ifname":            "eth0",
		"rx_write_requests": float64(100),
		"some_uint64":       uint64(100),
	}
	var attributes []*attribute.KeyValue

	processStats(stats, noop.Observer{}, func(s string) (metric.Int64ObservableCounter, bool) {
		return mustNewInt64ObservableCounter(noop.NewMeterProvider().Meter("test").Int64ObservableCounter("test_counter")), true
	}, attributes...)

	processStats(stats, noop.Observer{}, func(s string) (metric.Int64ObservableCounter, bool) {
		return nil, false
	}, attributes...)
}

func TestProcessNetNS(t *testing.T) {
	tests := []struct {
		name          string
		netnsID       string
		ipPodMap      map[string]types.NamespacedName
		commandScript []testexec.FakeCommandAction
		getObservable GetObservable
		expectError   bool
	}{
		{
			name:    "Empty netnsID and ipPodMap",
			netnsID: "",
			ipPodMap: map[string]types.NamespacedName{
				"192.168.1.1": {Namespace: "default", Name: "demo"},
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("[{\"rxWriteRequests\": 100}]"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("src 192.168.1.1"), nil, nil
						},
					}
					return fakeCmd
				},
			},
			getObservable: func(s string) (metric.Int64ObservableCounter, bool) {
				return nil, true
			},
			expectError: false,
		},

		{
			name:    "processNetNS call getRDMAStats get error",
			netnsID: "",
			ipPodMap: map[string]types.NamespacedName{
				"192.168.1.1": {Namespace: "default", Name: "demo"},
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return nil, nil, errors.New("mock error")
						},
					}
					return fakeCmd
				},
			},
			getObservable: func(s string) (metric.Int64ObservableCounter, bool) {
				return nil, true
			},
			expectError: true,
		},

		{
			name:    "processNetNS call getRDMAStats get empty stats",
			netnsID: "",
			ipPodMap: map[string]types.NamespacedName{
				"192.168.1.1": {Namespace: "default", Name: "demo"},
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("[]"), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("src 192.168.1.1"), nil, nil
						},
					}
					return fakeCmd
				},
			},
			getObservable: func(s string) (metric.Int64ObservableCounter, bool) {
				return nil, true
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meter := noop.NewMeterProvider().Meter("test")

			fakeExec := &testexec.FakeExec{
				CommandScript: tt.commandScript,
			}

			e := &exporter{
				cache: &FakeCache{},
				meter: meter,
				netlinkImpl: NetlinkImpl{
					RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
						return nil, nil
					},
					LinkList: func() ([]netlink.Link, error) {
						return nil, nil
					},
				},
				netns: func(netnsID string, toRun func() error) error {
					return toRun()
				},
				exec: fakeExec,
				log:  logutils.Logger.Named("rdma-metrics-exporter"),
			}
			observer := noop.Observer{}
			guidMapNetDeviceName := map[string]string{
				"b6:65:05:0c:9c:5c:f6:00": "ib1",
			}
			err := e.processNetNS(tt.netnsID, guidMapNetDeviceName, observer, tt.getObservable)
			if (err != nil) != tt.expectError {
				t.Errorf("Expected error: %v, but got: %v", tt.expectError, err)
			}
		})
	}
}

func TestUpdateUnregisteredMetrics(t *testing.T) {
	var commandScript []testexec.FakeCommandAction

	meter := noop.NewMeterProvider().Meter("test")

	fakeExec := &testexec.FakeExec{
		CommandScript: commandScript,
	}

	e := &exporter{
		observableMap: make(map[string]metric.Int64ObservableCounter),
		ch:            make(chan struct{}, 10),
		meter:         meter,
		netns: func(netnsID string, toRun func() error) error {
			return toRun()
		},
		exec: fakeExec,
		log:  logutils.Logger.Named("rdma-metrics-exporter"),
	}
	ctx, cancel := context.WithCancel(context.Background())

	err := e.registerMetrics(e.meter)
	if err != nil {
		t.Fatal(err)
	}

	go e.daemon(ctx)

	unRegistrationMetric := []string{"test_counter"}
	e.updateUnregisteredMetrics(unRegistrationMetric)

	time.Sleep(time.Second * 3)

	cancel()
}

func TestCallback(t *testing.T) {
	commandScript := []testexec.FakeCommandAction{
		func(cmd string, args ...string) exec.Cmd {
			fakeCmd := &testexec.FakeCmd{}
			fakeCmd.CombinedOutputScript = []testexec.FakeAction{
				func() ([]byte, []byte, error) {
					return []byte("[{\"rxWriteRequests\": 100}]"), nil, nil
				},
			}
			return fakeCmd
		},
		func(cmd string, args ...string) exec.Cmd {
			fakeCmd := &testexec.FakeCmd{}
			fakeCmd.CombinedOutputScript = []testexec.FakeAction{
				func() ([]byte, []byte, error) {
					return []byte("src 192.168.1.1"), nil, nil
				},
			}
			return fakeCmd
		},
	}

	meter := noop.NewMeterProvider().Meter("test")

	fakeExec := &testexec.FakeExec{
		CommandScript: commandScript,
	}

	e := &exporter{
		observableMap: make(map[string]metric.Int64ObservableCounter),
		netlinkImpl: NetlinkImpl{
			RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
				return nil, nil
			},
			LinkList: func() ([]netlink.Link, error) {
				return nil, nil
			},
		},
		ch:    make(chan struct{}, 10),
		meter: meter,
		netns: func(netnsID string, toRun func() error) error {
			return toRun()
		},
		exec:  fakeExec,
		log:   logutils.Logger.Named("rdma-metrics-exporter"),
		cache: &FakeCache{},
	}
	err := e.registerMetrics(e.meter)
	if err != nil {
		t.Fatal(err)
	}

	readDir = func(name string) ([]os.DirEntry, error) {
		return nil, nil
	}

	ctx2 := context.Background()
	observer := noop.Observer{}
	err = e.Callback(ctx2, observer)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReverseMACAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"00:1A:2B:3C:4D:5E", "5E:4D:3C:2B:1A:00"},
		{"12:34:56:78:9A:BC", "BC:9A:78:56:34:12"},
		{"AA:BB:CC:DD:EE:FF", "FF:EE:DD:CC:BB:AA"},
		{"01:02:03:04:05:06", "06:05:04:03:02:01"},
		{"", ""},
		{"A1:B2:C3", "C3:B2:A1"},
	}

	for _, tt := range tests {
		result := reverseMACAddress(tt.input)
		if result != tt.expected {
			t.Errorf("reverseMACAddress(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetIfnameNetDevMap(t *testing.T) {
	linkList := []netlink.Link{
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: ""}},
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "mock-empty-addr", HardwareAddr: nil}},
		&netlink.Device{LinkAttrs: netlink.LinkAttrs{
			Name: "enp5s0f0v6",
			HardwareAddr: func() net.HardwareAddr {
				mac, _ := net.ParseMAC("ae:36:ac:d1:c9:1d")
				return mac
			}(),
		}},
		&netlink.IPoIB{LinkAttrs: netlink.LinkAttrs{
			Name: "ibp13s0v7",
			HardwareAddr: func() net.HardwareAddr {
				mac, _ := net.ParseMAC("00:00:00:68:fe:80:00:00:00:00:00:00:08:f6:5c:9c:0c:05:65:b6")
				return mac
			}(),
		}},
	}

	rdmaList := []*netlink.RdmaLink{
		{Attrs: netlink.RdmaLinkAttrs{
			Name:     "mlx5_34",
			NodeGuid: "1d:c9:d1:fe:ff:ac:36:ae",
		}},
		{Attrs: netlink.RdmaLinkAttrs{
			Name:     "mlx5_6",
			NodeGuid: "b6:65:05:0c:9c:5c:f6:08",
		}},
	}

	tests := []struct {
		name   string
		nl     NetlinkImpl
		expErr bool
		exp    map[string]string
	}{
		{
			name: "call netlink.LinkList func get err",
			nl: NetlinkImpl{
				LinkList: func() ([]netlink.Link, error) {
					return nil, fmt.Errorf("mock err")
				},
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					return nil, fmt.Errorf("mock err")
				},
			},
			expErr: true,
		},
		{
			name: "call netlink.RdmaLinkList func get list",
			nl: NetlinkImpl{
				LinkList: func() ([]netlink.Link, error) {
					return linkList, nil
				},
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					return nil, fmt.Errorf("mock err")
				},
			},
			expErr: true,
		},
		{
			name: "success",
			nl: NetlinkImpl{
				LinkList: func() ([]netlink.Link, error) {
					return linkList, nil
				},
				RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
					return rdmaList, nil
				},
			},
			expErr: false,
			exp: map[string]string{
				"mlx5_34": "enp5s0f0v6",
				"mlx5_6":  "ibp13s0v7",
			},
		},
	}

	for _, tt := range tests {
		netDevMap, err := getIfnameNetDevMap(tt.nl)
		if (err != nil) != tt.expErr {
			t.Errorf("expected error: %v, but got: %v", tt.expErr, err)
		}
		got := make(map[string]string)
		for s, device := range netDevMap {
			got[s] = device.NetDevName
		}
		if tt.exp != nil {
			eq := reflect.DeepEqual(tt.exp, got)
			if !eq {
				t.Errorf("expected: %v, but got: %v", tt.exp, got)
			}
		}
	}
}
