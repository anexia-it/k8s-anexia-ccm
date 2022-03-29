package address

import (
	"fmt"
	"net"
	"testing"
)

func TestIPCalc(t *testing.T) {
	testcases := map[string]string{
		"10.0.0.0/8":        "10.255.255.254",
		"10.1.0.0/16":       "10.1.255.254",
		"10.1.2.0/24":       "10.1.2.254",
		"10.1.2.192/26":     "10.1.2.254",
		"10.1.2.192/27":     "10.1.2.222",
		"172.20.239.128/26": "172.20.239.190",

		"fda0:23:1f05:42::/64": "fda0:23:1f05:42:ffff:ffff:ffff:fffe",
		"2001:db8:42:34::/64":  "2001:db8:42:34:ffff:ffff:ffff:fffe",
		"2001:db8:42:34::/80":  "2001:db8:42:34::ffff:ffff:fffe",
		"2001:db8:42:34::/96":  "2001:db8:42:34::ffff:fffe",
		"2001:db8:42:34::/97":  "2001:db8:42:34::7fff:fffe",
		"2001:db8:42:34::/98":  "2001:db8:42:34::3fff:fffe",
		"2001:db8:42:34::/99":  "2001:db8:42:34::1fff:fffe",
		"2001:db8:42:34::/100": "2001:db8:42:34::fff:fffe",
		"2001:db8:42:34::/112": "2001:db8:42:34::fffe",
		"2001:db8:42:34::/124": "2001:db8:42:34::e",
	}

	for cidr, exp := range testcases {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("error parsing testcase CIDR: %w", err))
		}

		ip := net.ParseIP(exp)

		vip := calculateVIP(*n)
		if !vip.Equal(ip) {
			t.Errorf("VIP %q does not match expected %q", vip.String(), exp)
		}
	}
}

func TestBitmaskForN(t *testing.T) {
	testcases := []byte{
		0,
		0x80,
		0xc0,
		0xe0,
		0xf0,
		0xf8,
		0xfc,
		0xfe,
		0xff,

		// some more cases each returning 0xFF as prefix len > 8
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
		0xff,
	}

	for n, expMask := range testcases {
		mask := bitMaskForN(n)

		if mask != expMask {
			t.Errorf("bitMaskForN(%d) result %x does not match expected mask %x", n, mask, expMask)
		}
	}
}
