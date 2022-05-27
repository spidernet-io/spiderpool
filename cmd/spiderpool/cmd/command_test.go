// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/onsi/gomega/ghttp"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
)

const ifname string = "eth0"
const nspath string = "/some/where"
const containerID string = "dummy"
const CNITimeoutSec = 220

const (
	healthCheckRoute = "/v1/ipam/healthy"
	ipamReqRoute     = "/v1/ipam/ip"
)

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

		cniVersion = cmd.SupportCNIVersion

		netConf = cmd.NetConf{
			NetConf:            types.NetConf{CNIVersion: cniVersion},
			LogLevel:           constant.LogDebugLevelStr,
			IpamUnixSocketPath: sockPath,
		}

		addChan = make(chan struct{})
		delChan = make(chan struct{})

	})

	Context("mock ipam plugin interacts with agent through unix socket", func() {
		var server *ghttp.Server
		BeforeEach(func() {
			listener, err := net.Listen("unix", sockPath)
			Expect(err).NotTo(HaveOccurred())
			server = ghttp.NewUnstartedServer()
			server.HTTPTestServer.Listener = listener
			server.Start()
		})

		AfterEach(func() {
			server.Close()
		})

		DescribeTable("test cmdAdd",
			func(isHealthy, isPostIPAM bool, cmdArgs func() *skel.CmdArgs, mockServerResponse func() *models.IpamAddResponse, expectResponse func() *current.Result) {
				var ipamPostHandleFunc http.HandlerFunc

				// GET /v1/ipam/healthy
				server.RouteToHandler("GET", healthCheckRoute, ghttp.CombineHandlers(getHealthHandleFunc(isHealthy)))

				// POST /v1/ipam/ip
				if isPostIPAM {
					// You must pre-define this even if the mockServerResponse is nil!
					// And mockServerResponse is nil only use for bad health check!
					var mockServerResp *models.IpamAddResponse
					if nil != mockServerResponse {
						mockServerResp = mockServerResponse()
					}
					ipamPostHandleFunc = ghttp.RespondWithJSONEncoded(daemonset.PostIpamIpsOKCode, mockServerResp)
				} else {
					ipamPostHandleFunc = ghttp.RespondWithJSONEncoded(daemonset.PostIpamIpsInternalServerErrorCode, nil)
				}
				server.RouteToHandler("POST", ipamReqRoute, ghttp.CombineHandlers(ipamPostHandleFunc))

				// start client test.
				r, _, err := testutils.CmdAddWithArgs(cmdArgs(), func() error {
					return cmd.CmdAdd(cmdArgs())
				})

				// bad response check
				if !isHealthy || !isPostIPAM {
					var expectErr error
					if !isHealthy {
						expectErr = cmd.ErrAgentHealthCheck
					} else {
						expectErr = cmd.ErrPostIPAM
					}

					Expect(err).To(HaveOccurred())
					Expect(err).Should(MatchError(expectErr))
					return
				}

				Expect(err).NotTo(HaveOccurred())

				addResult, err := current.GetResult(r)
				Expect(err).NotTo(HaveOccurred())

				var expectResp *current.Result
				if nil != expectResponse {
					expectResp = expectResponse()
				} else {
					Fail("You must define expectResp if every route good in CmdAdd situation.")
				}

				// No need to check result.CNIVersion since cni types 100 library would hard code it with "1.0.0"

				// check Result.DNS
				Expect(reflect.DeepEqual(addResult.DNS, expectResp.DNS)).To(Equal(true))

				// check Result.IPs
				Expect(reflect.DeepEqual(addResult.IPs, expectResp.IPs)).To(Equal(true))

				// check Result.Interfaces
				Expect(reflect.DeepEqual(addResult.Interfaces, expectResp.Interfaces)).To(Equal(true))

				// check Result.Routes
				Expect(reflect.DeepEqual(addResult.Routes, expectResp.Routes))
			},
			Entry("returning an error on bad health check with ADD", false, true, func() *skel.CmdArgs {
				netConfBytes, err := json.Marshal(netConf)
				Expect(err).NotTo(HaveOccurred())
				args.StdinData = netConfBytes
				return args
			}, nil, nil),
			Entry("allocates addresses with ADD", true, true, func() *skel.CmdArgs {
				netConfBytes, err := json.Marshal(netConf)
				Expect(err).NotTo(HaveOccurred())
				args.StdinData = netConfBytes
				return args
			}, func() *models.IpamAddResponse {
				ipamAddResp := &models.IpamAddResponse{
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
					Routes: []*models.Route{{Dst: new(string), Gw: new(string)}},
				}
				// Routes
				*ipamAddResp.Routes[0].Dst = "15.5.6.8"
				*ipamAddResp.Routes[0].Gw = "15.5.6.1"

				// multi nic, ip responses
				*ipamAddResp.Ips[0].Address = "10.1.0.5"
				*ipamAddResp.Ips[0].Nic = "eth1"
				*ipamAddResp.Ips[0].Version = 4

				*ipamAddResp.Ips[1].Address = "1.2.3.30"
				*ipamAddResp.Ips[1].Nic = "eth0"
				*ipamAddResp.Ips[1].Version = 4

				return ipamAddResp
			}, func() *current.Result {
				expectResult := new(current.Result)
				// CNIVersion
				expectResult.CNIVersion = cniVersion
				// DNS
				expectResult.DNS = types.DNS{
					Nameservers: []string{"10.1.0.1"},
					Domain:      "local",
					Search:      []string{"foo"},
					Options:     []string{"somedomain.com"},
				}
				// IPs
				expectResult.IPs = []*current.IPConfig{{Interface: new(int)}}
				*expectResult.IPs[0].Interface = 0
				expectResult.IPs[0].Gateway = net.ParseIP("1.2.3.1")
				expectResult.IPs[0].Address = net.IPNet{IP: net.ParseIP("1.2.3.30"), Mask: net.CIDRMask(32, 32)}
				// Routes
				expectResult.Routes = []*types.Route{{Dst: net.IPNet{IP: net.ParseIP("15.5.6.8"), Mask: net.CIDRMask(32, 32)}, GW: net.ParseIP("15.5.6.1")}}
				//Interfaces
				expectResult.Interfaces = []*current.Interface{{Name: "eth0"}}
				return expectResult
			}),
		)

		DescribeTable("test cmdDel",
			func(isHealthy, isDeleteIPAM bool, cmdArgs func() *skel.CmdArgs) {
				var ipamDeleteHandleFunc http.HandlerFunc

				// GET /v1/ipam/healthy
				server.RouteToHandler("GET", healthCheckRoute, ghttp.CombineHandlers(getHealthHandleFunc(isHealthy)))

				// DELETE /v1/ipam/ip
				if isDeleteIPAM {
					ipamDeleteHandleFunc = ghttp.RespondWith(daemonset.DeleteIpamIPOKCode, nil)
				} else {
					ipamDeleteHandleFunc = ghttp.RespondWith(daemonset.DeleteIpamIPInternalServerErrorCode, nil)
				}
				server.RouteToHandler("DELETE", ipamReqRoute, ghttp.CombineHandlers(ipamDeleteHandleFunc))

				// start client test
				err := testutils.CmdDelWithArgs(cmdArgs(), func() error {
					return cmd.CmdDel(cmdArgs())
				})

				// bad response check
				if !isHealthy || !isDeleteIPAM {
					var expectErr error
					if !isHealthy {
						expectErr = cmd.ErrAgentHealthCheck
					} else {
						expectErr = cmd.ErrDeleteIPAM
					}

					Expect(err).To(HaveOccurred())
					Expect(err).Should(MatchError(expectErr))
					return
				}

				Expect(err).NotTo(HaveOccurred())
			},
			Entry("returning an error on bad health check with DEL", false, true, func() *skel.CmdArgs {
				netConf.LogLevel = constant.LogInfoLevelStr
				netConfBytes, err := json.Marshal(netConf)
				Expect(err).NotTo(HaveOccurred())
				args.StdinData = netConfBytes
				return args
			}),
			Entry("release addresses with DEL", true, true, func() *skel.CmdArgs {
				netConf.LogLevel = constant.LogWarnLevelStr
				netConfBytes, err := json.Marshal(netConf)
				Expect(err).NotTo(HaveOccurred())
				args.StdinData = netConfBytes
				return args
			}),
			Entry("release addresses with DEL", true, false, func() *skel.CmdArgs {
				netConf.LogLevel = constant.LogErrorLevelStr
				netConfBytes, err := json.Marshal(netConf)
				Expect(err).NotTo(HaveOccurred())
				args.StdinData = netConfBytes
				return args
			}),
		)
	})

	// TODO (Icarus9913): refactoring below
	Describe("test ipam plugin configuration ", func() {
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

})

func getHealthHandleFunc(isHealthy bool) http.HandlerFunc {
	var healthHandleFunc http.HandlerFunc

	if isHealthy {
		healthHandleFunc = ghttp.RespondWith(connectivity.GetIpamHealthyOKCode, nil)
	} else {
		healthHandleFunc = ghttp.RespondWith(connectivity.GetIpamHealthyInternalServerErrorCode, nil)
	}

	return healthHandleFunc
}
