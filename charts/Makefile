# get VERSION
include  ../Makefile.defs

VERSION_REGEX := '[vV]*[0-9]\+\.[0-9]\+\.[0-9]\+.*'
CHART_FILE := "./spiderpool/Chart.yaml"
VALUES_FILE := "./spiderpool/values.yaml"

.PHONY: all lint update-versions
all: update-versions lint package

#update version in chart
update-versions:
	$(ECHO_GEN) " Updating Chart version to $(VERSION)"
	echo "VERSION_MAJOR=$(VERSION_MAJOR)"
	echo "GIT_VERSION=$(GIT_VERSION)"
	echo "FULL_BUILD_VERSION=$(FULL_BUILD_VERSION)"
	@# Update chart versions to point to the current version.
	$(QUIET) \
		spiderpool_version="$(VERSION)";		\
		chart_version=` echo $(VERSION) | tr -d 'v' ` ; \
		sed -i 's/version: "*'$(VERSION_REGEX)'"*/version: '$$chart_version'/g' $(CHART_FILE);		\
		sed -i 's/appVersion: "*'$(VERSION_REGEX)'"*/appVersion: "'$$chart_version'"/g' $(CHART_FILE);	\
		sed -i 's/tag: ""/tag: '$$spiderpool_version'/g' $(VALUES_FILE)

lint: update-versions
	$(QUIET)helm lint --with-subcharts --values ./spiderpool/values.yaml ./spiderpool

package: lint
	$(QUIET)helm package ./spiderpool

clean:
	$(QUIET)rm -f *.tgz
