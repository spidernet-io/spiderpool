include ../Makefile.defs Makefile.defs

.PHONY: all
all: usage

#============ kind-e2e ====================
.PHONY: e2e
e2e:  kind-init e2e-test

.PHONY: kind-init
kind-init: kind-clean prepare
	$(QUIET)@echo "Install kind cluster..."
	@mkdir -p -v $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)
	$(QUIET)kube_proxy_mode=$(E2E_KUBE_PROXY_MODE) ip_family=$(E2E_IP_FAMILY) singe_node=$(E2E_SINGE_NODE) kind_image_tag=$(E2E_KIND_IMAGE_TAG) disable_default_cni=$(E2E_DISABLE_DEFAULT_CNI) p2ctl -t $(ROOT_DIR)/test/yamls/kind-config.tmpl > $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/kind-config.yaml
	$(QUIET)@cat $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/kind-config.yaml
	kind create cluster --config $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/kind-config.yaml --name $(E2E_CLUSTER_NAME) --kubeconfig $(E2E_KUBECONFIG)
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_WHEREABOUTS),true_true)
	$(QUIET)@echo "Install macvlan + whereabouts"
	$(QUIET)bash scripts/install-whereabouts.sh $(E2E_CLUSTER_NAME) $(E2E_KUBECONFIG)
	type=whereabouts p2ctl -t yamls/10-macvlan.tmpl > $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
endif
ifeq ($(INSTALL_MACVLAN)_$(INSTALL_SPIDER),true_true)
	$(QUIET) echo "Install macvlan + spiderpool"
	type=spiderpool p2ctl -t yamls/10-macvlan.tmpl > $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
	$(MAKE) -C $(ROOT_DIR) install
endif
	$(QUIET)@bash scripts/cni-install.sh $(E2E_CLUSTER_NAME) $(ROOT_DIR)
	$(QUIET)@bash scripts/install-macvlan.sh $(E2E_CLUSTER_NAME) $(ROOT_DIR)/test/.tmp/$(E2E_CLUSTER_NAME)/10-macvlan.conflist
	@kubectl get nodes --kubeconfig $(E2E_KUBECONFIG) > /dev/null 2>&1 || echo "failed to setup cluster"
	@echo "Please use: kubectl get nodes --kubeconfig $(E2E_KUBECONFIG) "

.PHONY: kind-clean
kind-clean:
	@echo "Cleanup kind cluster"
	rm -rf $(ROOT_DIR)/test/.tmp
	kind delete cluster --name $(E2E_CLUSTER_NAME)

.PHONY: prepare
prepare:
	$(QUEIT) @bash scripts/prepare.sh $(ROOT_DIR)

#============ e2e ====================
.PHONY: e2e-test
e2e-test:
	$(QUIET)@echo "Run E2E"
	$(QUIET) $(ROOT_DIR)/tools/scripts/ginkgo.sh  --race --timeout=$(E2E_TIMEOUT) --label-filter="$(E2E_GINKGO_LABELS)" -r e2e/* -- --ipFamily=$(E2E_IP_FAMILY) --kubeconfig=$(E2E_KUBECONFIG)
usage:
	@echo "usage:"
	@echo -e "  \033[35m make prepare \033[0m:       --- Check some required tools is exist like docker/helm.etc and download cni-plugins"
	@echo -e "  \033[35m make init \033[0m:          --- Setup a kind cluster, Such as: kind-init E2E_CLUSTER_NAME=spider,More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make e2e-test \033[0m:      --- Ginkgo test,Such as: make e2e-test, More config refer to Makefile.defs(e2e-kind-config)"
	@echo -e "  \033[35m make kind-clean \033[0m:    --- Clean kind cluster and some config file, Such as: make kind-clean E2E_CLUSTER_NAME=spider"
	@echo -e "  \033[35m make e2e \033[0m:           --- prepare -> kind-init -> e2e-test "
