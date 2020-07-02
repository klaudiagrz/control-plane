APP_NAME = kcp-installer
DOCKER_PATH=$(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)
IMG = $(DOCKER_PUSH_REPOSITORY)$(DOCKER_PUSH_DIRECTORY)/$(APP_NAME)
TAG = $(DOCKER_TAG)

.PHONY: build-image
build-image:
	cd ./../../; \
	docker build -t $(IMG):$(TAG) \
		-f ./tools/kcp-installer/kcp.Dockerfile  \
		--build-arg INSTALLER_DIR=$(DOCKER_PATH) .

.PHONY: push-image
push-image:
	docker push $(IMG):$(TAG)

.PHONY: ci-pr
ci-pr: build-image push-image

.PHONY: ci-master
ci-master: build-image push-image

.PHONY: ci-release
ci-release: build-image push-image

.PHONY: release
release: build-image push-image

# Creates release artifacts. Artifacts are stored in $(ARTIFACTS) directory.
# This variable is provided by Prow and files stored there are automatically uploaded to GCS.
.PHONY: ci-create-release-artifacts
ci-create-release-artifacts:
	cd ./../..; \
	docker run --rm --volume $(CURDIR)/../..:/control-plane  \
		--volume $(ARTIFACTS):$(ARTIFACTS) \
		-e KCP_INSTALLER_VERSION=$(TAG) \
		-e ARTIFACTS_DIR=$(ARTIFACTS) \
		--entrypoint /control-plane/installation/scripts/generate-installer-artifacts.sh \
		eu.gcr.io/kyma-project/acs-installer:0.0.4