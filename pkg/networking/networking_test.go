package networking

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRandomMac(t *testing.T) {
	res, err := getRandomMac()
	require.Nil(t, err)

	require.Equal(t, len(res), 6)
	require.Equal(t, res[0]&0x02, uint8(0x02)) // Locally administered?
	require.Equal(t, res[0]&0x01, uint8(0x00)) // Unicast address?
}

func TestNextIP(t *testing.T) {
	_, network, err := net.ParseCIDR("192.168.0.0/24")
	require.Nil(t, err)

	ip, err := getNextIP(network, network.IP)
	require.Nil(t, err)

	require.Equal(t, "192.168.0.1", ip.String())
}

func TestNextUnusual(t *testing.T) {
	netIP, network, err := net.ParseCIDR("192.168.0.254/23")
	require.Nil(t, err)

	ip, err := getNextIP(network, netIP)
	require.Nil(t, err)

	require.Equal(t, "192.168.0.255", ip.String())
}

func TestNextSmall(t *testing.T) {
	netIP, network, err := net.ParseCIDR("192.168.0.254/31")
	require.Nil(t, err)

	ip, err := getNextIP(network, netIP)
	require.Nil(t, err)

	require.Equal(t, "192.168.0.255", ip.String())
}

func TestNextIPFull(t *testing.T) {
	netIP, network, err := net.ParseCIDR("192.168.0.254/24")
	require.Nil(t, err)

	_, err = getNextIP(network, netIP)
	require.NotNil(t, err)
}
