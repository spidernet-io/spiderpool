# Spiderpool Repository Guide

## Repository Purpose

**Spiderpool** is a Cloud Native Computing Foundation (CNCF) sandbox project that provides an underlay and RDMA network solution for Kubernetes. It enhances the capabilities of Macvlan CNI, IPvlan CNI, and SR-IOV CNI, delivering exceptional network performance particularly beneficial for network I/O-intensive and low-latency applications like storage, middleware, and AI workloads.

### Key Features
- **Underlay CNI Support**: macvlan, ipvlan, SR-IOV for bare metal, VM, and public cloud environments
- **RDMA Network Acceleration**: RoCE and InfiniBand support for AI workloads
- **CRD-based Dual-stack IPAM**: IPv4/IPv6 support with static IP assignment
- **Multiple Network Interfaces**: Support for overlay and underlay CNI combinations
- **eBPF Enhancements**: Kube-proxy replacement and socket short-circuiting
- **Simplified Installation**: Eliminates need for manual Multus CNI installation

## Repository Structure

### Core Components
```
spiderpool/
├── cmd/                          # Main binaries
│   ├── spiderpool-controller/    # Controller deployment
│   ├── spiderpool-agent/         # Agent daemonset
│   ├── spiderpool-init/          # Initialization jobs
│   ├── coordinator/              # Network coordination plugin
│   └── spiderpoolctl/            # CLI tool
├── pkg/                          # Core Go packages
│   ├── ipam/                     # IP address management
│   ├── ippoolmanager/            # IP pool management
│   ├── subnetmanager/            # Subnet management
│   ├── nodemanager/              # Node management
│   ├── podmanager/               # Pod management
│   └── ...                       # Additional managers
├── api/                          # API definitions and OpenAPI specs
├── charts/                       # Helm charts for deployment
│   └── spiderpool/               # Main Helm chart
├── images/                       # Container image definitions
├── docs/                         # Documentation
├── test/                         # E2E and integration tests
└── tools/                        # Build and development tools
```

### Key Directories
- **cmd/**: Contains all main executables for Spiderpool components
- **pkg/**: Core business logic organized by functional areas
- **charts/spiderpool/**: Helm chart for deploying Spiderpool
- **images/**: Dockerfiles for all Spiderpool container images
- **docs/**: Comprehensive documentation including usage guides
- **test/**: End-to-end tests and CI configurations

## CI/CD and GitHub Workflows

### Linting and Quality Checks
- **lint-golang.yaml**: Go linting, unit tests, and SDK validation
- **lint-markdown.yaml**: Markdown linting and link checking
- **lint-yaml.yaml**: YAML syntax validation
- **lint-spell.yaml**: Spell checking across codebase
- **lint-license.yaml**: License header validation
- **lint-openapi.yaml**: OpenAPI specification validation
- **lint-codeowners.yaml**: CODEOWNERS file validation
- **lint-doc.yaml**: Documentation linting
- **lint-grafana-file.yaml**: Grafana dashboard validation

### Build and Image Workflows
- **build-image-*.yaml**: Multiple workflows for building container images
  - `build-image-base.yaml`: Base image builds
  - `build-image-ci.yaml`: CI image builds
  - `build-image-release.yaml`: Release image builds
  - `build-image-latest.yaml`: Latest tag builds
  - `build-image-plugins.yaml**: Plugin image builds

### Testing Workflows
- **auto-pr-ci.yaml**: PR-triggered E2E testing
- **auto-nightly-ci.yaml**: Nightly comprehensive testing
- **auto-diff-k8s-ci.yaml**: Kubernetes version compatibility testing
- **auto-upgrade-ci.yaml**: Upgrade path testing
- **e2e-init.yaml**: E2E test initialization

### Release and Automation
- **auto-version-release.yaml**: Automated version releases
- **auto-cherrypick.yaml**: Cherry-pick automation
- **auto-update-authors.yaml**: Author list updates
- **call-release-*.yaml**: Release automation workflows
- **update-cniplugins-version.yaml**: CNI plugin version updates
- **update-golang-version.yaml**: Golang version updates

### Maintenance Workflows
- **auto-clean-image.yaml**: Image cleanup
- **close-stale-issues-pr.yaml**: Stale issue/PR management
- **ci-image-gc-by-pr.yaml**: Image garbage collection
- **trivy-scan-image.yaml**: Security scanning

## Development Setup

### Prerequisites
- Go 1.24.5+ (as specified in workflows)
- Docker and Docker Buildx
- Helm 3.x
- kubectl
- Kind (for local development)

### Quick Start
```bash
# Clone the repository
git clone https://github.com/spidernet-io/spiderpool.git
cd spiderpool

# Build all binaries
make build-bin

# Build container images
make build_image

# Run unit tests
make unittest-tests

# Deploy using Helm
helm install spiderpool charts/spiderpool
```

### Key Make Targets
- `make build-bin`: Build all binaries
- `make build_image`: Build container images
- `make lint-golang`: Run Go linting
- `make unittest-tests`: Run unit tests
- `make e2e-test`: Run end-to-end tests
- `make install`: Install binaries

## Testing Strategy

### Test Types
1. **Unit Tests**: Go unit tests with coverage reporting
2. **Integration Tests**: Component integration testing
3. **E2E Tests**: Full Kubernetes cluster testing
4. **Performance Tests**: Network performance benchmarking
5. **Upgrade Tests**: Version upgrade path validation

### Test Environments
- **Kind**: Local development testing
- **Bare Metal**: Production-like testing
- **Cloud Providers**: Multi-cloud compatibility
- **Multiple K8s Versions**: Version compatibility matrix

## Deployment Options

### Helm Chart
Primary deployment method using the official Helm chart:
```bash
helm repo add spiderpool https://spidernet-io.github.io/spiderpool
helm install spiderpool spiderpool/spiderpool
```

### Manual Installation
- YAML manifests available in `charts/spiderpool/templates/`
- Custom resource definitions in `charts/spiderpool/crds/`

### Supported Environments
- **Bare Metal**: Full feature support
- **Virtual Machines**: All features supported
- **Public Cloud**: AWS, GCP, Azure compatibility
- **Hybrid Cloud**: Unified underlay network solution

## Documentation

### Key Documentation Areas
- **Installation**: Quick start and production deployment guides
- **Usage**: IPAM, network policies, and configuration examples
- **Architecture**: Detailed component descriptions and interactions
- **Performance**: Benchmarking and optimization guides
- **Troubleshooting**: Common issues and solutions
- **API Reference**: CRD specifications and API documentation

### Documentation Location
- **Main docs**: `docs/` directory
- **API docs**: Auto-generated from OpenAPI specs
- **Examples**: `docs/usage/` and `docs/example/` directories
- **Architecture**: `docs/concepts/` directory

## Community and Support

### Communication Channels
- **Slack**: #spiderpool on CNCF Slack
- **Email**: Maintainer contact list
- **Community Meetings**: Monthly on the 1st
- **WeChat**: Technical discussion group

### Contribution Guidelines
- **Contributing Guide**: `docs/develop/contributing.md`
- **Code of Conduct**: Standard CNCF code of conduct
- **Governance**: Maintainer and committer governance model
- **Issue Templates**: Available in `.github/ISSUE_TEMPLATE/`

## Security

### Security Scanning
- **Trivy**: Container image vulnerability scanning
- **CodeQL**: Static code analysis
- **Dependency Scanning**: Go module vulnerability checks
- **License Scanning**: License compliance verification

### Security Reporting
- Security issues should be reported to maintainers privately
- Responsible disclosure process outlined in security policy