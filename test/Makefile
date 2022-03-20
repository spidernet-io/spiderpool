include ../Makefile.defs

all: test

test:
ifeq (${shell (which ginkgo &>/dev/null) && echo EXIST },EXIST)
	@echo "use ginkgo from local machine"
	$(QUIET)ginkgo -vv -r ./
else
	@echo "use ginkgo from vendor"
	$(QUIET)./ginkgo.sh
endif

