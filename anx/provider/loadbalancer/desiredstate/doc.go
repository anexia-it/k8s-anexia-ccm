// Package desiredstate builds a deterministic, side-effect-free description of
// the Anexia LBaaS resources required by a Kubernetes LoadBalancer Service.
//
// It deliberately does not call the Anexia Engine, inspect Engine resource
// states, wait for automations, or read addresses from Service.Status. Engine
// identifiers which only exist after creation are represented by logical
// dependency keys. An executor can resolve those keys to identifiers while
// creating resources in dependency order.
package desiredstate
