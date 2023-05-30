# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

include ../../Makefile.defs

TARGET := coordinator

GO_BUILD_LDFLGAS= -ldflags "-X github.com/spidernet-io/spiderpool/internal/version.coordinatorGitCommit=$(GIT_COMMIT_VERSION) \
-X github.com/spidernet-io/spiderpool/internal/version.coordinatorGitBranch=$(GIT_BRANCH) \
-X github.com/spidernet-io/spiderpool/internal/version.coordinatorBuildDate=$(GO_BUILD_TIME) \
-X github.com/spidernet-io/spiderpool/internal/version.coordinatorVersion=$(GIT_TAG)"

all: $(TARGET)

.PHONY: all $(TARGET)

$(TARGET): ../../Makefile ../../Makefile.defs Makefile
	@$(ECHO_GO)
	$(QUIET)$(GO_BUILD) $(GO_BUILD_LDFLGAS) -o $(TARGET)

clean:
	@$(ECHO_CLEAN)
	$(QUIET)rm -f $(TARGET)
	$(QUIET)$(GO_CLEAN)

install:
	$(QUIET)echo "install $(TARGET) to $(DESTDIR_BIN)"
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BIN)
	$(QUIET)$(INSTALL) -m 0755 $(TARGET) $(DESTDIR_BIN)



