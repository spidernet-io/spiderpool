// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"encoding/json"
	"fmt"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/connectivity"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
)

const ifname string = "eth0"
const nspath string = "/some/where"
const containerID string = "dummy"
const CNITimeoutSec = 220
const healthCheckRoute = "/v1/ipam/healthy"
const ipamReqRoute = "/v1/ipam/ip"

var cniVersion string
var args *skel.CmdArgs
var netConf cmd.NetConf
var sockPath string

var addChan, delChan chan struct{}

var _ = Describe("spiderpool plugin", Label("unitest", "ipam_plugin_test"), func() {
	BeforeEach(func() {
		// generate one temp unix file.
		tempDir := GinkgoT().TempDir()
		sockPath = tempDir + "/tmp.sock"

		// cleanup the temp unix file at the end.
		DeferCleanup(func() {
			err := os.RemoveAll(sockPath)
			Expect(err).NotTo(HaveOccurred())
		})

		args = &skel.CmdArgs{
			ContainerID: containerID,
			Netns:       nspath,
			IfName:      ifname,
		}

		cniVersion = "0.3.1"

		netConf = cmd.NetConf{
			NetConf:            types.NetConf{CNIVersion: cniVersion},
			LogLevel:           "DEBUG",
			IpamUnixSocketPath: sockPath,
		}

		addChan = make(chan struct{})
		delChan = make(chan struct{})
	})

	It(fmt.Sprintf("[%s] allocates addresses with ADD/DEL", cniVersion), func() {
		// use httptest to mock one server that handles healthcheck, ipam add/del routes response.
		func() {
			listener, err := net.Listen("unix", sockPath)
			Expect(err).NotTo(HaveOccurred())

			mux := http.NewServeMux()

			mux.HandleFunc(healthCheckRoute, func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(connectivity.GetIpamHealthyOKCode)
			})

			mux.HandleFunc(ipamReqRoute, func(resp http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPost {
					// POST /ipam/ip will
					resp.Header().Set("Content-Type", "application/json")
					resp.WriteHeader(daemonset.PostIpamIPOKCode)

					ipamIP := daemonset.NewPostIpamIPOK()
					ipamIP.Payload = &models.IpamAddResponse{
						DNS: &models.DNS{
							Domain:      "local",
							Nameservers: []string{"10.1.0.1"},
							Options:     []string{"somedomain.com"},
							Search:      []string{"foo"},
						},
						Ips: []*models.IPConfig{
							{
								Address: new(string),
								Gateway: "10.1.0.1",
								Nic:     new(string),
								Version: new(int64),
								Vlan:    8,
							},
							{
								Address: new(string),
								Gateway: "1.2.3.1",
								Nic:     new(string),
								Version: new(int64),
								Vlan:    6,
							},
						},
						Routes: []*models.Route{
							{Dst: "15.5.6.8", Gw: "15.5.6.1"},
						},
					}

					// multi nic, ip responses
					*ipamIP.Payload.Ips[0].Address = "10.1.0.5"
					*ipamIP.Payload.Ips[0].Nic = "eth1"
					*ipamIP.Payload.Ips[0].Version = 4

					*ipamIP.Payload.Ips[1].Address = "1.2.3.30"
					*ipamIP.Payload.Ips[1].Nic = "eth0"
					*ipamIP.Payload.Ips[1].Version = 4

					err = json.NewEncoder(resp).Encode(ipamIP.Payload)
					Expect(err).NotTo(HaveOccurred())
				} else if req.Method == http.MethodDelete {
					// DELETE /ipam/ip just return http status code
					resp.WriteHeader(daemonset.DeleteIpamIPOKCode)
				} else {
					resp.WriteHeader(http.StatusInternalServerError)
				}
			})

			server := httptest.NewUnstartedServer(mux)
			server.Listener = listener
			server.Start()
		}()

		netConfBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())
		args.StdinData = netConfBytes

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			r, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).NotTo(HaveOccurred())

			addResult, err := current.GetResult(r)
			Expect(err).NotTo(HaveOccurred())

			// check Result.DNS
			Expect(addResult.DNS.Domain).To(Equal("local"))
			Expect(len(addResult.DNS.Nameservers)).To(Equal(1))
			Expect(addResult.DNS.Nameservers[0]).To(Equal("10.1.0.1"))
			Expect(len(addResult.DNS.Options)).To(Equal(1))
			Expect(addResult.DNS.Options[0]).To(Equal("somedomain.com"))
			Expect(len(addResult.DNS.Search)).To(Equal(1))
			Expect(addResult.DNS.Search[0]).To(Equal("foo"))

			// check Result.IPs
			Expect(len(addResult.IPs)).To(Equal(1))
			Expect(addResult.IPs[0].Address.String()).To(Equal("1.2.3.30/32"))
			Expect(addResult.IPs[0].Gateway.String()).To(Equal("1.2.3.1"))
			Expect(*addResult.IPs[0].Interface).To(Equal(0))

			// check Result.Interfaces
			Expect(len(addResult.Interfaces)).To(Equal(1))
			Expect(addResult.Interfaces[0].Name).To(Equal("eth0"))

			// check Result.Routes
			Expect(len(addResult.Routes)).To(Equal(1))
			Expect(addResult.Routes[0].Dst.String()).To(Equal("15.5.6.8/32"))
			Expect(addResult.Routes[0].GW.String()).To(Equal("15.5.6.1"))

			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It(fmt.Sprintf("[%s] is returning an error on required properties lost with ADD/DEL", cniVersion), func() {
		// use httptest to mock one server that handles healthcheck, ipam add/del routes response.
		func() {
			listener, err := net.Listen("unix", sockPath)
			Expect(err).NotTo(HaveOccurred())

			mux := http.NewServeMux()

			mux.HandleFunc(healthCheckRoute, func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(connectivity.GetIpamHealthyOKCode)
			})

			mux.HandleFunc(ipamReqRoute, func(resp http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPost {
					// POST /ipam/ip will
					resp.Header().Set("Content-Type", "application/json")
					resp.WriteHeader(daemonset.PostIpamIPOKCode)

					ipamIP := daemonset.NewPostIpamIPOK()
					ipamIP.Payload = &models.IpamAddResponse{
						Routes: []*models.Route{
							{Dst: "15.5.6.8", Gw: "15.5.6.1"},
						},
					}

					err = json.NewEncoder(resp).Encode(ipamIP.Payload)
					Expect(err).NotTo(HaveOccurred())
				} else if req.Method == http.MethodDelete {
					// DELETE /ipam/ip just return http status code
					resp.WriteHeader(daemonset.DeleteIpamIPOKCode)
				} else {
					resp.WriteHeader(http.StatusInternalServerError)
				}
			})

			server := httptest.NewUnstartedServer(mux)
			server.Listener = listener
			server.Start()
		}()

		netConfBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())
		args.StdinData = netConfBytes

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).To(HaveOccurred())

			close(addChan)
		}()
		Eventually(addChan).WithTimeout(5 * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).NotTo(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It(fmt.Sprintf("[%s] is returning an error on bad health check with ADD/DEL", cniVersion), func() {
		// use httptest to mock bad health check http route response.
		func() {
			listener, err := net.Listen("unix", sockPath)
			Expect(err).NotTo(HaveOccurred())

			mux := http.NewServeMux()
			mux.HandleFunc(healthCheckRoute, func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(connectivity.GetIpamHealthyInternalServerErrorCode)
			})

			server := httptest.NewUnstartedServer(mux)
			server.Listener = listener
			server.Start()
		}()

		netConfBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())
		args.StdinData = netConfBytes

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).Should(HaveOccurred())
			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It(fmt.Sprintf("[%s] is returning an error on conf broken with ADD/DEL", cniVersion), func() {
		confBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())
		confBytes = append(confBytes, []byte("}")...)
		args.StdinData = confBytes

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).To(HaveOccurred())
			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).To(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It(fmt.Sprintf("[%s] is returning an error on bad log configuration with ADD/DEL", cniVersion), func() {
		netConf.LogLevel = "bad"
		netConfBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())
		args.StdinData = netConfBytes

		// Allocate the IP
		go func() {
			defer GinkgoRecover()

			_, _, err := testutils.CmdAddWithArgs(args, func() error {
				return cmd.CmdAdd(args)
			})
			Expect(err).To(HaveOccurred())
			close(addChan)
		}()
		Eventually(addChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())

		// Release the IP
		go func() {
			defer GinkgoRecover()

			err = testutils.CmdDelWithArgs(args, func() error {
				return cmd.CmdDel(args)
			})
			Expect(err).To(HaveOccurred())
			close(delChan)
		}()
		Eventually(delChan).WithTimeout(CNITimeoutSec * time.Second).Should(BeClosed())
	})

	It("Check default network configuration", func() {
		// set some configurations with empty value.
		netConf.LogLevel = ""
		netConf.IpamUnixSocketPath = ""

		netConfBytes, err := json.Marshal(netConf)
		Expect(err).NotTo(HaveOccurred())

		conf, err := cmd.LoadNetConf(netConfBytes)
		Expect(err).NotTo(HaveOccurred())

		Expect(conf.LogLevel).Should(Equal(cmd.DefaultLogLevelStr))
		Expect(conf.IpamUnixSocketPath).Should(Equal(constant.DefaultIPAMUnixSocketPath))
	})

})
