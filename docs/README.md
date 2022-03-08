# Documentation

## Development Guide

### Build
```shell
git clone https://github.com/spidernet-io/spiderpool.git && cd spiderpool
make test
make dameon
```

### Continuous Integration
Use the `.github/ workflows/ci.yml` file to configure continuous integration 
for each commit. The `.github/ workflows/releases.yml` file for the release 
workflow, which will push the image to the production repository.