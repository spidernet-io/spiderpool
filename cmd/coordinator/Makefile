# Copyright 2023 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

include ../../Makefile.defs

TARGET := coordinator

GO_BUILD_FLAG= CGO_ENABLED=0 $(GO) build

all: $(TARGET)

.PHONY: all $(TARGET)

$(TARGET): ../../Makefile ../../Makefile.defs Makefile
	@$(ECHO_GO)
	$(QUIET)$(GO_BUILD_FLAG) -o $(TARGET)

clean:
	@$(ECHO_CLEAN)
	$(QUIET)rm -f $(TARGET)
	$(QUIET)$(GO_CLEAN)

install:
	$(QUIET)echo "install $(TARGET) to $(DESTDIR_BIN)"
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BIN)
	$(QUIET)$(INSTALL) -m 0755 $(TARGET) $(DESTDIR_BIN)
