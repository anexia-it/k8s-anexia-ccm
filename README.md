[![CI Build & Test](https://github.com/anexia-it/k8s-anexia-ccm/actions/workflows/push.yml/badge.svg?branch=main&event=push)](https://github.com/anexia-it/k8s-anexia-ccm/actions/workflows/push.yml)

# Kubernetes cloud-controller-manager for Anexia Cloud

## Development quickstart

Requires a Go (>= 1.17) toolchain and `make`. For compiling and testing, use the `make` targets
`k8s-anexia-ccm` (default target) and `test`. Targets for running and interactive debugging are
`run` and `debug`, but you need some more setup first:

* create API key for Anexia Engine
* create cluster in Anexia Kubernetes Service
* copy `envrc-sample` to `.envrc`, fill your values and run `direnv allow`
  - alternatively, you can export `ANEXIA_TOKEN` with your token and `KKP_HUMAN_READABLE_NAME` with your cluster's name

Interactive debugging requires [`delve`](https://github.com/go-delve/delve) to be installed in path.

Running (and debugging) is handled by `hack/anxkube-dev-run`, which first retrieves configs, command line arguments
and co from the seed cluster, pauses the Cluster object and scales in-cluster CCM down to 0 - make sure you revert
that when not using your cluster only for development.

Because of that magic, you need `kubectl` for running and debug and it has to be configured for the correct seed
cluster as current context. The script also has more dependencies, but it will tell you.

