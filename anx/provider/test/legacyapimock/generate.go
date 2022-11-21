package legacyapimock

//go:generate mockgen -package legacyapimock -destination ipam_address.go -mock_names API=MockIPAMAddressAPI go.anx.io/go-anxcloud/pkg/ipam/address API
//go:generate mockgen -package legacyapimock -destination ipam_prefix.go -mock_names API=MockIPAMPrefixAPI go.anx.io/go-anxcloud/pkg/ipam/prefix API
//go:generate mockgen -package legacyapimock -destination ipam.go -mock_names API=MockIPAMAPI go.anx.io/go-anxcloud/pkg/ipam API
