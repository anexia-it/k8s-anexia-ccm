package components

import (
	"bytes"
	"crypto/md5"
	"go.anx.io/go-anxcloud/pkg/lbaas/bind"
	"strconv"
)

type HashedBind struct {
	Bind *bind.Bind
	hash Hash
}

func NewHashedBind(base bind.Bind) HashedBind {
	buffer := bytes.Buffer{}
	buffer.WriteString(base.Name)
	buffer.WriteString(strconv.Itoa(base.Port))
	buffer.WriteString(base.Address)

	return HashedBind{
		Bind: &base,
		hash: md5.Sum(buffer.Bytes()),
	}
}

func (h HashedBind) Hash() Hash {
	return h.hash
}

func DeleteBind(identifier string, binds []HashedBind) []HashedBind {
	result := make([]HashedBind, 0, len(binds))
	for _, hashedBind := range binds {
		if hashedBind.Bind.Identifier != identifier {
			result = append(result, hashedBind)
		}
	}
	return result
}
