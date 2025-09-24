// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package rdmametrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
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
	"github.com/spidernet-io/spiderpool/pkg/rdmametrics/oteltype"
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
		expected                    []NetnsItem
		expectError                 bool
	}{
		{
			name:                        "read system rdma mode error",
			rdmaSystemGetNetnsModeError: true,
			expectError:                 true,
		},
		{
			name:       "exclusive mode with entries",
			mode:       "exclusive",
			dirEntries: []os.DirEntry{mockDirEntry("netns1"), mockDirEntry("netnsimpl")},
			expected: []NetnsItem{
				{
					ID: "netns1",
					Fd: "/var/run/netns/netns1",
				},
				{
					ID: "netnsimpl",
					Fd: "/var/run/netns/netnsimpl",
				},
				{
					ID: "netns1",
					Fd: "/var/run/docker/netns/netns1",
				},
				{
					ID: "netnsimpl",
					Fd: "/var/run/docker/netns/netnsimpl",
				},
			},
			expectError: false,
		},
		{
			name:        "non-exclusive mode",
			mode:        "non-exclusive",
			expected:    []NetnsItem{},
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

func TestProcessNetNS(t *testing.T) {
	tests := []struct {
		name          string
		netnsID       NetnsItem
		ipPodMap      map[string]podownercache.Pod
		commandScript []testexec.FakeCommandAction
		getObservable GetObservable
		expectError   bool
	}{
		{
			name:    "Empty netnsID and ipPodMap",
			netnsID: NetnsItem{},
			ipPodMap: map[string]podownercache.Pod{
				"192.168.1.1": {
					NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"},
					OwnerInfo:      podownercache.OwnerInfo{},
				},
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
			netnsID: NetnsItem{},
			ipPodMap: map[string]podownercache.Pod{
				"192.168.1.1": {
					NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"},
					OwnerInfo:      podownercache.OwnerInfo{},
				},
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
			netnsID: NetnsItem{},
			ipPodMap: map[string]podownercache.Pod{
				"192.168.1.1": {
					NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"},
					OwnerInfo:      podownercache.OwnerInfo{},
				},
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

		{
			name:    "sriov",
			netnsID: NetnsItem{ID: "12345"},
			ipPodMap: map[string]podownercache.Pod{
				"192.168.1.1": {
					NamespacedName: types.NamespacedName{Namespace: "default", Name: "demo"},
					OwnerInfo:      podownercache.OwnerInfo{},
				},
			},
			commandScript: []testexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							res := `
								[
									{
										"ifname": "mlx5_1",
										"port": 1,
										"rx_write_requests": 2331678,
										"rx_read_requests": 1161608,
										"rx_atomic_requests": 0,
										"rx_dct_connect": 0,
										"out_of_buffer": 0,
										"out_of_sequence": 0,
										"duplicate_request": 0,
										"rnr_nak_retry_err": 0,
										"packet_seq_err": 0,
										"implied_nak_seq_err": 0,
										"local_ack_timeout_err": 0,
										"resp_local_length_error": 0,
										"resp_cqe_error": 0,
										"req_cqe_error": 0,
										"req_remote_invalid_request": 0,
										"req_remote_access_errors": 0,
										"resp_remote_access_errors": 0,
										"resp_cqe_flush_error": 0,
										"req_cqe_flush_error": 0,
										"req_transport_retries_exceeded": 0,
										"req_rnr_retries_exceeded": 0,
										"roce_adp_retrans": 0,
										"roce_adp_retrans_to": 0,
										"roce_slow_restart": 0,
										"roce_slow_restart_cnps": 0,
										"roce_slow_restart_trans": 0,
										"rp_cnp_ignored": 0,
										"rp_cnp_handled": 0,
										"np_ecn_marked_roce_packets": 0,
										"np_cnp_sent": 0,
										"rx_icrc_encapsulated": 0
									}
								]
								`
							return []byte(res), nil, nil
						},
					}
					return fakeCmd
				},
				func(cmd string, args ...string) exec.Cmd {
					fakeCmd := &testexec.FakeCmd{}
					fakeCmd.CombinedOutputScript = []testexec.FakeAction{
						func() ([]byte, []byte, error) {
							return []byte("1.1.1.1 via 10.193.78.1 dev ens29f1 src 192.168.1.1 uid 0"), nil, nil
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
				cache: &FakeCache{
					IPToPodMap: tt.ipPodMap,
				},
				meter: meter,
				netlinkImpl: NetlinkImpl{
					RdmaLinkList: func() ([]*netlink.RdmaLink, error) {
						rdmaList := []*netlink.RdmaLink{
							{
								Attrs: netlink.RdmaLinkAttrs{
									Name:     "mlx5_34",
									NodeGuid: "1d:c9:d1:fe:ff:ac:36:ae",
								},
							},
							{
								Attrs: netlink.RdmaLinkAttrs{
									Name:         "mlx5_6",
									NodeGuid:     "b6:65:05:0c:9c:5c:f6:08",
									SysImageGuid: "b6:65:05:0c:9c:5c:f6:00",
								},
							},
							{
								Attrs: netlink.RdmaLinkAttrs{
									Name:         "mlx5_1",
									NodeGuid:     "ea:6d:2d:00:03:c0:63:9c",
									SysImageGuid: "ea:6d:2d:00:03:c0:63:9c",
								},
							},
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
							&netlink.Device{LinkAttrs: netlink.LinkAttrs{
								Name: "ens841np0",
								HardwareAddr: func() net.HardwareAddr {
									mac, _ := net.ParseMAC("9c:63:c0:2d:6d:ea")
									return mac
								}(),
							}},
						}
						return linkList, nil
					},
				},
				netns: func(netnsID NetnsItem, toRun func() error) error {
					return toRun()
				},
				exec: fakeExec,
				ethtool: EthtoolImpl{Stats: func(netIfName string) ([]oteltype.Metrics, error) {
					return []oteltype.Metrics{
						{
							Name:  "vport_speed",
							Value: int64(400000),
						},
					}, nil
				}},
				log: logutils.Logger.Named("rdma-metrics-exporter"),
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
		netns: func(netnsID NetnsItem, toRun func() error) error {
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
		netns: func(netnsID NetnsItem, toRun func() error) error {
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

func TestGetNodeGuidNetDeviceNameMap(t *testing.T) {
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
		&netlink.Device{
			LinkAttrs: netlink.LinkAttrs{
				Name: "ens841np0",
				HardwareAddr: func() net.HardwareAddr {
					mac, _ := net.ParseMAC("9c:63:c0:2d:6d:ea")
					return mac
				}(),
			},
		},
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
		{Attrs: netlink.RdmaLinkAttrs{
			Name:         "mlx5_1",
			NodeGuid:     "ea:6d:2d:00:03:c0:63:9c",
			SysImageGuid: "ea:6d:2d:00:03:c0:63:9c",
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
				"ea:6d:2d:00:03:c0:63:9c": "ens841np0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNodeGuidNetDeviceNameMap(tt.nl)
			if (err != nil) != tt.expErr {
				t.Errorf("expected error: %v, but got: %v", tt.expErr, err)
			}
			if tt.exp != nil {
				eq := reflect.DeepEqual(tt.exp, got)
				if !eq {
					t.Errorf("expected: %v, but got: %v", tt.exp, got)
				}
			}
		})
	}
}

func TestExtractSrcIPStringIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		ok       bool
	}{
		{
			name:     "Valid src IP with space after",
			input:    "random data src 192.168.1.1 other data",
			expected: "192.168.1.1",
			ok:       true,
		},
		{
			name:     "Valid src IP at the end of string",
			input:    "random data src 192.168.1.1",
			expected: "192.168.1.1",
			ok:       true,
		},
		{
			name:     "Marker not found",
			input:    "random data without marker",
			expected: "",
			ok:       false,
		},
		{
			name:     "Marker at the end",
			input:    "random data src ",
			expected: "",
			ok:       false,
		},
		{
			name:     "No space after IP",
			input:    "random data src 192.168.1.1someotherdata",
			expected: "192.168.1.1someotherdata",
			ok:       true,
		},
		{
			name:     "Empty input string",
			input:    "",
			expected: "",
			ok:       false,
		},
		{
			name:     "Short input without room for IP",
			input:    " src",
			expected: "",
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := extractSrcIPStringIndex(tt.input)
			if result != tt.expected || ok != tt.ok {
				t.Errorf("for input '%s', expected (%s, %v), got (%s, %v)",
					tt.input, tt.expected, tt.ok, result, ok)
			}
		})
	}
}
