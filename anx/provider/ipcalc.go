// XXX: nuke this file when we can add and delete LoadBalancer IPs, which is not the case
// for Anexia Kubernetes Service MVP
// I'm seriously surprised "get nth address of network" isn't in Go's standard library o.o
// -- Mara @LittleFox94 Grosch, 2022-02-18

package provider

import (
	"net"
)

// calculateVIP calculates the IP address before the broadcast address of a given network, which is
// the one virtual IP we currently configure on the LoadBalancer VMs.
func calculateVIP(n net.IPNet) net.IP {
	net, size := n.Mask.Size()
	ret := n.IP

	// we iterate through the address byte by byte
	for i := 0; i < size/8; i++ {
		n := net - i*8
		byteMask := bitMaskForN(n)

		// "set all host bits to one"
		ret[i] |= ^byteMask
	}

	// set LSB of address to zero, resulting in the address below the broadcast address of the network
	ret[len(ret)-1] -= 1

	return ret
}

// bitMaskForN generates a bit mask with the given number of bits set, starting from MSB.
func bitMaskForN(n int) byte {
	if n >= 8 {
		return 0xFF
	}

	var ret byte = 0

	for n > 0 {
		ret |= 1 << (8 - n)
		n--
	}

	return ret
}
