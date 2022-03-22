# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

include ../../Makefile.defs

TARGET := spiderpoolctl

all: $(TARGET)

.PHONY: all $(TARGET)

$(TARGET): ../../Makefile ../../Makefile.defs Makefile
	@$(ECHO_GO)
	$(QUIET)$(GO_BUILD) -o $(TARGET)

clean:
	@$(ECHO_CLEAN)
	$(QUIET)rm -f $(TARGET)
	$(QUIET)$(GO_CLEAN)

install-bash-completion: $(TARGET) install-bash-completion-only

install-bash-completion-only:
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BASH_COMPLETION)
	$(QUIET)touch $(DESTDIR_BASH_COMPLETION)/$(TARGET).bash
	#./$(TARGET) completion bash > $(DESTDIR_BASH_COMPLETION)/$(TARGET).bash


install:
	$(QUIET)echo "install $(TARGET) to $(DESTDIR_BIN)"
	$(QUIET)$(INSTALL) -m 0755 -d $(DESTDIR_BIN)
	$(QUIET)$(INSTALL) -m 0755 $(TARGET) $(DESTDIR_BIN)



