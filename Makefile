.PHONY: clean test deps build bootstrap-tools image

HACKDIR=./hack/bin
GORELEASER_CMD=$(HACKDIR)/goreleaser
ORG ?= ca-gip
VERSION=$(shell git rev-parse --short HEAD)
KIND = kind

$(HACKDIR):
	mkdir -p $(HACKDIR)

bootstrap-tools: $(HACKDIR)
	command -v $(HACKDIR)/goreleaser || VERSION=v2.5.0 TMPDIR=$(HACKDIR) bash hack/goreleaser-install.sh
	command -v staticcheck || go install honnef.co/go/tools/cmd/staticcheck@latest
	chmod +x $(HACKDIR)/goreleaser

clean:
	rm -rf vendor build/*

build: bootstrap-tools deps
	ORG=${ORG} $(GORELEASER_CMD) release --clean --snapshot

deps:
	go mod tidy
	go mod vendor
	bash hack/update-codegen.sh
	go mod tidy

test: bootstrap-tools
	go test ./...
	staticcheck ./...

image: build

.PHONY: install-ldap
install-ldap:
	kubectl apply -f test/e2e/conf/ldap/config.yaml
	helm repo add helm-openldap https://jp-gouin.github.io/helm-openldap/
	helm upgrade --install openldap helm-openldap/openldap-stack-ha  -f test/e2e/conf/ldap/myvalues.yaml

.PHONY: uninstall-ldap
uninstall-ldap:
	kubectl delete -f test/e2e/conf/ldap/config.yaml
	helm uninstall openldap

.PHONY: setup-kind
setup-kind: delete-kind ## create a kind cluster for running the acceptance tests locally
	$(KIND) create cluster \
		--wait=5m \
		--image kindest/node:$(KIND_K8S_VERSION) \
		--name $(KIND_CLUSTER_NAME)  \
		--config $(KIND_CONFIG_FILE)
	kubectl config use-context $(K8S_CLUSTER_CONTEXT)

.PHONY: delete-kind
delete-kind: ## delete the kind cluster
	$(KIND) delete cluster --name $(KIND_CLUSTER_NAME) || true

.PHONY: load-docker-image
load-docker-image: images-download ## Pull image if not present and load image into kind cluster instead of fetching from a push
	docker image inspect --format="ignore output" $(IMG) || docker image pull $(IMG)
	$(KIND) load docker-image --name $(KIND_CLUSTER_NAME) $(IMG)
	$(KIND) load docker-image --name $(KIND_CLUSTER_NAME) $(RBAC-PROXY-IMG)
	$(KIND) load docker-image --name $(KIND_CLUSTER_NAME) $(EXTERNAL-SECRETS-IMG)

.PHONY: images-download
images-download:
	## Until I can figure out why docker mirror doesn't work directly, let's pull from remote image, and retag.
	$(CONTAINER_TOOL) pull $(RBAC-PROXY-IMG)
	$(CONTAINER_TOOL) pull $(EXTERNAL-SECRETS-MIRROR-IMG)
	$(CONTAINER_TOOL) tag $(EXTERNAL-SECRETS-MIRROR-IMG) $(EXTERNAL-SECRETS-IMG)

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: components-download
components-download:
	test -d $(INSTALL_FOLDER_ESO) || mkdir -p $(INSTALL_FOLDER_ESO)
	rm -f $(INSTALL_FOLDER_ESO)/*.tgz
	curl -sSLo $(EXTERNAL_SECRETS_CHART_FILENAME) https://github.com/external-secrets/external-secrets/releases/download/helm-chart-$(EXTERNAL_SECRETS_CHART_VERSION)/external-secrets-$(EXTERNAL_SECRETS_CHART_VERSION).tgz && mv $(EXTERNAL_SECRETS_CHART_FILENAME) $(INSTALL_FOLDER_ESO)
	
	test -d $(INSTALL_FOLDER_VAULT) || mkdir -p $(INSTALL_FOLDER_VAULT)
	rm -f $(INSTALL_FOLDER_VAULT)/*.tgz
	curl -sSLo $(VAULT_CHART_FILENAME) https://helm.releases.hashicorp.com/vault-$(VAULT_CHART_VERSION).tgz && mv $(VAULT_CHART_FILENAME) $(INSTALL_FOLDER_VAULT)/

.PHONY: components-install
components-install:
	$(info If this fails, please run make components-download first. To save bandwidth, we do not run components-download at all times.)
	kubectl apply -f $(INSTALL_FOLDER_LDAP)/config.yaml
	kubectl apply -f $(INSTALL_FOLDER_LDAP)/deploy.yaml
	$(HELM) install external-secrets $(INSTALL_FOLDER_ESO)/$(EXTERNAL_SECRETS_CHART_FILENAME) -f $(INSTALL_FOLDER_ESO)/eso-values.yaml
	$(SHELL) $(.SHELLFLAGS)  $(INSTALL_FOLDER_VAULT)/config.sh
	$(HELM) install vault $(INSTALL_FOLDER_VAULT)/$(VAULT_CHART_FILENAME) -f $(INSTALL_FOLDER_VAULT)/vault-values.yaml --wait
	@echo "Sleep 30s to allow helm installed resources to become ready ..."
	sleep 30

.PHONY: components-uninstall
components-uninstall:
	-$(HELM) uninstall external-secrets
	-$(HELM) uninstall vault



.PHONY: test-e2e
test-e2e: setup-kind load-docker-image install deploy components-download components-install
	NO_PROXY=$(NO_PROXY) no_proxy=$(NO_PROXY) VAULT_URL=http://127.0.0.1:38300 VAULT_ADDR=http://127.0.0.1:38300 \
	INTEGRATION_TESTS=true KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) K8S_CLUSTER_CONTEXT=$(K8S_CLUSTER_CONTEXT) CGO_ENABLED=0 \
	K8S_VAULT_NAMESPACE=$(K8S_VAULT_NAMESPACE) \
	go test github.com/ca-gip/vault-operator/test/e2e/... $(TESTARGS) -timeout=30m -v -count=1

