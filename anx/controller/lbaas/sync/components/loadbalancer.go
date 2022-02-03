package components

import "crypto/md5"

type HashedLoadBalancer struct {
	Identifier string
	Frontends  []HashedFrontend
	Backends   []HashedBackend
	Servers    []HashedServer
	Binds      []HashedBind
	hash       Hash
}

func NewHashedLoadBalancer(identifier string, f []HashedFrontend, b []HashedBackend, s []HashedServer, fb []HashedBind) HashedLoadBalancer {
	lbHash := md5.New()
	for _, frontend := range f {
		hash := frontend.Hash()
		lbHash.Write(hash[:])
	}

	for _, backend := range b {
		hash := backend.Hash()
		lbHash.Write(hash[:])
	}

	for _, server := range s {
		hash := server.hash
		lbHash.Write(hash[:])
	}

	for _, bind := range fb {
		hash := bind.Hash()
		lbHash.Write(hash[:])
	}

	var hashsum [16]byte
	copy(hashsum[:], lbHash.Sum(nil)[:16])

	return HashedLoadBalancer{
		Identifier: identifier,
		Frontends:  f,
		Backends:   b,
		Servers:    s,
		Binds:      fb,
		hash:       hashsum,
	}
}
