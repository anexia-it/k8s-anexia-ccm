package components

import (
	"bytes"
	"crypto/md5"
	"go.anx.io/go-anxcloud/pkg/lbaas/server"
	"strconv"
)

type HashedServer struct {
	Server *server.Server
	hash   Hash
}

func NewHashedServer(base server.Server) HashedServer {
	buffer := bytes.Buffer{}
	buffer.WriteString(base.Name)
	buffer.WriteString(base.Check)
	buffer.WriteString(base.IP)
	buffer.WriteString(strconv.Itoa(base.Port))

	return HashedServer{
		Server: &base,
		hash:   md5.Sum(buffer.Bytes()),
	}
}

func (h HashedServer) Hash() Hash {
	return h.hash
}

func DeleteServer(identifier string, servers []HashedServer) []HashedServer {
	result := make([]HashedServer, 0, len(servers))
	for _, hashedServer := range servers {
		if hashedServer.Server.Identifier != identifier {
			result = append(result, hashedServer)
		}
	}
	return result
}
