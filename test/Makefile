include ../Makefile.defs Makefile.defs

.PHONY: all
all: usage

#============ kind-e2e ====================
.PHONY: e2e
e2e:  kind-init e2e-test

DOWNLOAD_DIR := $(ROOT_DIR)/test/.download
CLUSTER_DIR := $(ROOT_DIR)/test/.cluster
E2E_KUBECONFIG = $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/.kube/config


.PHONY: kind-init
kind-init: kind-clean prepare
	$(QUIET) [ -n "${E2E_CLUSTER_NAME}" ] || { echo "error, miss E2E_CLUSTER_NAME " ; false ; }
	$(QUIET) RED='\E[1;31m'; CLEAR='\E[0m' ; echo "$${RED} Install kind cluster with name ${E2E_CLUSTER_NAME} $${CLEAR}"
	$(QUIET) mkdir -p -v $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)
	$(QUIET) kube_proxy_mode=$(E2E_KUBE_PROXY_MODE) ip_family=$(E2E_IP_FAMILY) \
			singe_node=$(E2E_SINGE_NODE) kind_image_tag=$(E2E_KIND_IMAGE_TAG) \
			disable_default_cni=$(E2E_DISABLE_DEFAULT_CNI) \
			p2ctl -t $(ROOT_DIR)/test/yamls/kind-config.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) cat $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET) kind create cluster --config $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/kind-config.yaml --name $(E2E_CLUSTER_NAME) --kubeconfig $(E2E_KUBECONFIG)
	$(QUIET) bash scripts/cni-install.sh $(E2E_CLUSTER_NAME) $(DOWNLOAD_DIR)
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_WHEREABOUTS),true_true)
	$(QUIET) echo "Install macvlan + whereabouts"
	$(QUIET) bash scripts/install-whereabouts.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
	type=whereabouts p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
	$(QUIET) bash scripts/install-macvlan.sh $(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
endif
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_SPIDER),true_true)
	$(QUIET) echo "Install macvlan + spiderpool"
	type=spiderpool p2ctl -t yamls/10-macvlan.tmpl > $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
	$(QUIET) bash scripts/install-macvlan.sh $(E2E_CLUSTER_NAME) $(CLUSTER_DIR)/$(E2E_CLUSTER_NAME)/10-macvlan-spider.conflist
	$(MAKE) -C $(ROOT_DIR) install
endif
ifeq ($(INSTALL_MULTUS),true)
	@echo "Install multus..."
	@bash scripts/install-multus.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
endif
	@kubectl get nodes --kubeconfig $(E2E_KUBECONFIG) > /dev/null 2>&1 || echo "failed to setup cluster"
	@echo "Please use: kubectl get nodes --kubeconfig $(E2E_KUBECONFIG)"


.PHONY: kind-clean
kind-clean:
	@echo "Cleanup kind cluster"
	rm -rf $(CLUSTER_DIR)
	kind delete cluster --name $(E2E_CLUSTER_NAME) &>/dev/null


.PHONY: prepare
prepare:
	$(QUEIT) JUST_CLI_CHECK=true scripts/install-tools.sh
	$(QUEIT) mkdir -p $(DOWNLOAD_DIR)
	$(QUEIT) IMAGE_MUTLUS=$(IMAGE_MUTLUS_NAME) \
			IMAGE_WHEREABOUTS=$(IMAGE_WHEREABOUTS_NAME) \
			TEST_IMAGE=$(TEST_IMAGE_NAME) \
			CNI_PACKAGE_VERSION=$(CNI_PACKAGE_VERSION) \
			scripts/prepare.sh $(DOWNLOAD_DIR)

#============ e2e ====================
.PHONY: e2e-test
e2e-test:
	$(QUIET)@echo "Run E2E"
	$(QUIET) $(ROOT_DIR)/tools/scripts/ginkgo.sh  --race --timeout=$(E2E_TIMEOUT) --label-filter="$(E2E_GINKGO_LABELS)" -r e2e/* -- --ipFamily=$(E2E_IP_FAMILY) --kubeconfig=$(E2E_KUBECONFIG)

.PHONY: usage
usage:
	@echo "usage:"
	@echo -e "  \033[35m make prepare \033[0m:       --- Check some required tools is exist like docker/helm.etc and download cni-plugins"
	@echo -e "  \033[35m make init \033[0m:          --- Setup a kind cluster, Such as: kind-init E2E_CLUSTER_NAME=spider,More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make e2e-test \033[0m:      --- Ginkgo test,Such as: make e2e-test, More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make kind-clean \033[0m:    --- Clean kind cluster and some config file, Such as: make kind-clean E2E_CLUSTER_NAME=spider"
	@echo -e "  \033[35m make e2e \033[0m:           --- prepare -> kind-init -> e2e-test "
