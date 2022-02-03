package components

import (
	"bytes"
	"crypto/md5"
	"go.anx.io/go-anxcloud/pkg/lbaas/backend"
	"strconv"
)

type Hash = [16]byte

type HashedBackend struct {
	Backend *backend.Backend
	hash    Hash
}

func NewHashedBackend(base backend.Backend) HashedBackend {
	buffer := bytes.Buffer{}
	buffer.WriteString(base.Name)
	buffer.WriteString(base.HealthCheck)
	buffer.WriteString(strconv.Itoa(base.ServerTimeout))
	buffer.WriteString(string(base.Mode))

	return HashedBackend{
		Backend: &base,
		hash:    md5.Sum(buffer.Bytes()),
	}
}

func (b HashedBackend) Hash() Hash {
	return b.hash
}

func GetBackendByName(name string, backends []HashedBackend) *HashedBackend {
	for _, backend := range backends {
		if backend.Backend.Name == name {
			return &backend
		}
	}
	return nil
}

func DeleteBackend(identifier string, backends []HashedBackend) []HashedBackend {
	result := make([]HashedBackend, 0, len(backends))
	for _, hashedBackend := range backends {
		if hashedBackend.Backend.Identifier != identifier {
			result = append(result, hashedBackend)
		}
	}
	return result
}
