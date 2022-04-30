include ../Makefile.defs
include Makefile.defs


#============ kind-e2e ====================

.PHONY: all
all: usage

.PHONY: e2e
e2e:  kind-init e2e-test


.PHONY: kind-init
kind-init: check_env kind-clean prepare
	$(QUIET) [ -n "${E2E_CLUSTER_NAME}" ] || { echo "error, miss E2E_CLUSTER_NAME " ; false ; }
	@echo -e "\033[35m [Step 3] Init The Cluster: $(E2E_CLUSTER_NAME) \033[0m"
	$(QUIET) mkdir -p -v $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	$(QUIET) kube_proxy_mode=$(E2E_KUBE_PROXY_MODE) ip_family=$(E2E_IP_FAMILY) \
			 singe_node=$(E2E_SINGE_NODE) kind_image_tag=$(E2E_KIND_IMAGE_TAG) \
			 disable_default_cni=$(E2E_DISABLE_DEFAULT_CNI) \
			 p2ctl -t $(ROOT_DIR)/test/yamls/kind-config.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) kind create cluster \
		--config $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml \
		--name $(E2E_CLUSTER_NAME) --kubeconfig $(E2E_KUBECONFIG)
	@echo -e "\033[35m [Step 4] Install The CNI-Plugins: $(CNI_PACKAGE_VERSION)  \033[0m"
	$(QUIET) bash scripts/cni-install.sh $(E2E_CLUSTER_NAME) $(DOWNLOAD_DIR)
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_WHEREABOUTS),true_true)
	@echo -e "\033[35m [Step 5] Install Macvlan + Whereabouts \033[0m"
	$(QUIET) IMAGE_WHEREABOUTS=$(IMAGE_WHEREABOUTS_NAME) \
			scripts/install-whereabouts.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
	type=whereabouts p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
	$(QUIET) bash scripts/install-macvlan.sh $(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
endif
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_SPIDER),true_true)
	@echo -e "\033[35m [Step 5] Install Macvlan + Spiderpool \033[0m"
	$(MAKE) -C $(ROOT_DIR) install
	type=spiderpool p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
	$(QUIET) bash scripts/install-macvlan.sh \
		$(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
endif
ifeq ($(INSTALL_MULTUS),true)
	@echo -e "\033[35m [Step 6] Install Multus \033[0m"
	@ IMAGE_MULTUS=$(IMAGE_MULTUS_NAME) \
		TEST_IMAGE=$(TEST_IMAGE_NAME) \
		CLUSTER_PATH=$(CLUSTER_DIR)/$(E2E_CLUSTER_NAME) \
		scripts/install-multus.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
endif
	@echo -e "\033[35m Init The Cluster Finished, Please Use 'kubectl get nodes --kubeconfig $(E2E_KUBECONFIG)' \033[0m"
	@echo "--------------------"
	@echo "succeeded to setup cluster $(E2E_CLUSTER_NAME) "



.PHONY: kind-clean
kind-clean:
	@echo -e "\033[35m [Step 1] Clean The Cluster: $(E2E_CLUSTER_NAME) \033[0m"
	@echo "Deleting Tmp files..."
	@rm -rf $(CLUSTER_DIR)
	-@kind delete cluster --name $(E2E_CLUSTER_NAME) &>/dev/null


.PHONY: check_env
check_env:
	$(QUIET) [ -n "${E2E_CLUSTER_NAME}" ] || { echo "error, miss E2E_CLUSTER_NAME " ; false ; }
	$(QUIET) ( [ "$(E2E_IP_FAMILY)" == "ipv4" ] || [ "$(E2E_IP_FAMILY)" == "ipv6" ]  || [ "$(E2E_IP_FAMILY)" == "dual" ] ) \
			|| { echo "error, E2E_IP_FAMILY=$(E2E_IP_FAMILY) must be ipv4/ipv6/dual" ;  exit 1 ; }


.PHONY: prepare
prepare:
	@echo -e "\033[35m [Step 2] Check The Tools For Ready: \033[0m"
	$(QUEIT) JUST_CLI_CHECK=true scripts/install-tools.sh
	$(QUEIT) mkdir -p $(DOWNLOAD_DIR)
	$(QUEIT) IMAGE_MULTUS=$(IMAGE_MULTUS_NAME) \
			 IMAGE_WHEREABOUTS=$(IMAGE_WHEREABOUTS_NAME) \
			 TEST_IMAGE=$(TEST_IMAGE_NAME) \
			 CNI_PACKAGE_VERSION=$(CNI_PACKAGE_VERSION) \
			 scripts/prepare.sh $(DOWNLOAD_DIR)


#============ e2e ====================
.PHONY: e2e-test
e2e-test:
	$(QUIET) ( docker ps | grep "kindest/node" &>/dev/null ) || { echo "error, please setup kind first" ; exit 1 ; }
	@ echo -e "\033[35m [E2E] Run E2E with label ${E2E_GINKGO_LABELS} , timeout=${E2E_TIMEOUT} E2E_EXT_ARGS=${E2E_EXT_ARGS} \033[0m"
	@  FLAGS="" ; \
		FLAGS+=" --ClusterName=$(E2E_CLUSTER_NAME) " ; \
		if [ "$(E2E_IP_FAMILY)" == "ipv4" ] ; then \
		  FLAGS+=" --IpV4Enabled=true --IpV6Enabled=false " ; \
		elif [ "$(E2E_IP_FAMILY)" == "ipv6" ] ; then \
		  FLAGS+=" --IpV4Enabled=false --IpV6Enabled=true " ; \
		else \
		  FLAGS+=" --IpV4Enabled=true --IpV6Enabled=true " ; \
		fi ; \
		if [ -f "$(E2E_KUBECONFIG)" ] ; then \
		   FLAGS+=" --KubeConfigPath=$(E2E_KUBECONFIG) "  ; \
		else \
		   echo "error, does not exist KUBECONFIG $(E2E_KUBECONFIG)" ; exit 1 ; \
        fi; \
		if [ "$(INSTALL_MULTUS)" == "true" ] ; then \
		  	FLAGS+=" --MultusEnabled=true " ; \
 		else \
 			FLAGS+=" --MultusEnabled=false " ; \
 		fi ; \
		if [ "$(INSTALL_WHEREABOUTS)" == "true" ] ; then \
		  	FLAGS+=" --SpiderIPAMEnabled=false " ; \
 		else \
 			FLAGS+=" --SpiderIPAMEnabled=true " ; \
 		fi ; \
		$(ROOT_DIR)/tools/scripts/ginkgo.sh  \
		--race --timeout=$(E2E_TIMEOUT) \
		--label-filter="$(E2E_GINKGO_LABELS)"  -randomize-suites -randomize-all  -vv --keep-going  ${E2E_EXT_ARGS} \
		-r e2e/* -- $${FLAGS} ;  \
		if (($$?==0)) ; then \
		  	echo "succeeded to run all test" ; \
  		else \
  		  	echo "failed to run all test" ; \
  		  	exit 1 ; \
  		fi


.PHONY: usage
usage:
	@echo "usage:"
	@echo -e "  \033[35m make prepare \033[0m:       --- Check some required tools is exist like docker/helm.etc and download cni-plugins"
	@echo -e "  \033[35m make init \033[0m:          --- Setup a kind cluster, Such as: kind-init E2E_CLUSTER_NAME=spider,More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make e2e-test \033[0m:      --- Ginkgo test,Such as: make e2e-test, More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make kind-clean \033[0m:    --- Clean kind cluster and some config file, Such as: make kind-clean E2E_CLUSTER_NAME=spider"
	@echo -e "  \033[35m make e2e \033[0m:           --- prepare -> kind-init -> e2e-test "
