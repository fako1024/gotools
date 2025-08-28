package link

import "fmt"

const (

	// IPLayerOffsetEthernet denotes the ethernet header offset
	IPLayerOffsetEthernet = 14

	// IPLayerOffsetLinuxSLL2 denotes the Linux SLL2 header offset
	IPLayerOffsetLinuxSLL2 = 20

	// LayerOffsetPPPOE denotes the additional offset for PPPOE (session) packets
	LayerOffsetPPPOE = 8
)

// Type denotes the linux interface type
type Type int

const (

	// TypeInvalid denotes an invalid link type
	TypeInvalid Type = iota

	// TypeEthernet denotes a link of type ARPHRD_ETHER
	TypeEthernet Type = 1

	// TypeLoopback denotes a link of type ARPHRD_LOOPBACK
	TypeLoopback Type = 772

	// TypePPP denotes a link of type ARPHRD_PPP
	TypePPP Type = 512

	// TypeIP6IP6 denotes a link of type ARPHRD_TUNNEL6
	TypeIP6IP6 Type = 769

	// TypeGRE denotes a link of type ARPHRD_IPGRE
	TypeGRE Type = 778

	// TypeGRE6 denotes a link of type ARPHRD_IP6GRE
	TypeGRE6 Type = 823

	// TypeLinuxSLL2 denotes a link of type LINUX_SLL2
	TypeLinuxSLL2 Type = 276

	// TypeNone denotes a link of type ARPHRD_NONE:
	// Tunnel / anything else (confirmed: Wireguard, OpenVPN)
	TypeNone Type = 65534
)

// IPHeaderOffset returns the link / interface specific payload offset for the IP header
// c.f. https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/uapi/linux/if_arp.h
func (l Type) IPHeaderOffset() byte {
	switch l {
	case TypeEthernet,
		TypeLoopback:
		return IPLayerOffsetEthernet
	case TypePPP,
		TypeIP6IP6,
		TypeGRE,
		TypeGRE6,
		TypeNone:
		return 0
	case TypeLinuxSLL2:
		return IPLayerOffsetLinuxSLL2
	}

	// Panic if unknown
	panic(fmt.Sprintf("LinkType %d not supported by slimcap (yet), please open a GitHub issue", l))
}
