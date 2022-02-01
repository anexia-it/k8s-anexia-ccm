package components

import (
	"bytes"
	"crypto/md5"
	"go.anx.io/go-anxcloud/pkg/lbaas/frontend"
)

type HashedFrontend struct {
	Frontend *frontend.Frontend
	hash     Hash
}

func NewHashedFrontend(base frontend.Frontend) HashedFrontend {
	buffer := bytes.Buffer{}
	buffer.WriteString(base.DefaultBackend.Name)
	buffer.WriteString(base.ClientTimeout)
	buffer.WriteString(string(base.Mode))

	return HashedFrontend{
		Frontend: &base,
		hash:     md5.Sum(buffer.Bytes()),
	}
}

func (h HashedFrontend) Hash() Hash {
	return h.hash
}

func FindCorrespondingFrontend(name string, source []HashedFrontend, target []HashedFrontend) *HashedFrontend {
	var searchHash Hash
	for _, hashedFrontend := range source {
		if hashedFrontend.Frontend.Name == name {
			searchHash = hashedFrontend.Hash()
		}
	}
	for _, hashedFrontend := range target {
		if hashedFrontend.Hash() == searchHash {
			return &hashedFrontend
		}
	}
	return nil
}

func DeleteFrontend(identifier string, frontends []HashedFrontend) []HashedFrontend {
	res := make([]HashedFrontend, 0, len(frontends))
	for _, hashedFrontend := range frontends {
		if identifier != hashedFrontend.Frontend.Identifier {
			res = append(res, hashedFrontend)
		}
	}
	return res
}
