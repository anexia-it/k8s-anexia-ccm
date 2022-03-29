package reconciliation

import "net"

// Port describes a port for LBaaS reconciliation.
type Port struct {
	// External is the port from outside the cluster, as configured into the LBaaS Bind resource.
	External uint16

	// Internal is the port LBaaS connects to on the backend servers, in Kubernetes this is the NodePort.
	Internal uint16
}

// Server describes a backend server for LBaaS reconciliation, in Kubernetes this is a Node.
type Server struct {
	// Name of the server, used for naming the LBaaS Server resources.
	Name string

	// IP address to configure in LBaaS Server resources.
	Address net.IP
}
