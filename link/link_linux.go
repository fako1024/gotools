//go:build linux
// +build linux

package link

import (
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"syscall"

	"golang.org/x/sys/unix"
)

const (
	netBasePath   = "/sys/class/net/"
	netUEventPath = "/uevent"
	netTypePath   = "/type"
	netFlagsPath  = "/flags"

	netUEventIfIndexPrefix    = "IFINDEX="
	netUEventDevTypePrefix    = "DEVTYPE="
	netUEventDevTypeVLAN      = "vlan"
	netUEventIfIndexPrefixLen = len(netUEventIfIndexPrefix)
	netUEventDevTypePrefixLen = len(netUEventDevTypePrefix)
)

var (
	// ErrIndexOutOfBounds denotes the (unlikely) case of an invalid index being outside the range of an int
	ErrIndexOutOfBounds = errors.New("interface index out of bounds")
)

// HostLinks returns all (or selected) host interfaces
// The function will not fail if a link is down, but will fail if a link does not exist
func HostLinks(names ...string) (Links, error) {
	if len(names) == 0 {
		linkDir, err := os.OpenFile(netBasePath, os.O_RDONLY, 0600)
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := linkDir.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}()

		names, err = linkDir.Readdirnames(-1)
		if err != nil {
			return nil, err
		}
	}

	ifaces := make([]*Link, len(names))
	var err error
	for i, name := range names {
		if ifaces[i], err = newLink(name); err != nil {
			return nil, err
		}
	}

	return ifaces, nil
}

// IsUp determines if an interface is currently up (at the time of the call)
func (l *Link) IsUp() (bool, error) {

	data, err := os.ReadFile(netBasePath + l.Name + netFlagsPath)
	if err != nil {
		return false, err
	}

	flags, err := strconv.ParseInt(
		strings.TrimSpace(string(data)), 0, 64)
	if err != nil {
		return false, err
	}

	return flags&unix.IFF_UP != 0, nil
}

// IPs retrieves all IPv4 and IPv6 addresses assigned to the interface using
// a minimal netlink RTM_GETADDR dump to avoid the higher-level net package.
// Mostly extracted from the net package internals (net.Interface.Addrs()).
func (l *Link) IPs() ([]net.IP, error) {
	tab, err := syscall.NetlinkRIB(syscall.RTM_GETADDR, syscall.AF_UNSPEC)
	if err != nil {
		return nil, os.NewSyscallError("netlinkrib", err)
	}
	msgs, err := syscall.ParseNetlinkMessage(tab)
	if err != nil {
		return nil, os.NewSyscallError("parsenetlinkmessage", err)
	}
	ifat, err := addrTable(l, msgs)
	if err != nil {
		return nil, err
	}
	return ifat, nil
}

func (l *Link) getIndexVLAN() (int, bool, error) {
	data, err := os.ReadFile(netBasePath + l.Name + netUEventPath)
	if err != nil {
		return -1, false, err
	}

	return extractIndexVLAN(data)
}

func (l *Link) getLinkType() (Type, error) {
	data, err := os.ReadFile(netBasePath + l.Name + netTypePath)
	if err != nil {
		return -1, err
	}

	val, err := strconv.Atoi(
		strings.TrimSpace(string(data)))
	if err != nil {
		return -1, err
	}

	if val < 0 || val > 65535 {
		return -1, fmt.Errorf("invalid link type read from `%s`: %d", netBasePath+l.Name+netTypePath, val)
	}

	return Type(val), nil
}

////////////////////////////////////////////////////////////////////////////////

func newLink(name string) (link *Link, err error) {
	link = &Link{
		Name: name,
	}

	if link.Index, link.IsVLAN, err = link.getIndexVLAN(); err != nil {
		return nil, err
	}
	if link.Type, err = link.getLinkType(); err != nil {
		return nil, err
	}

	return link, nil
}

func extractIndexVLAN(data []byte) (int, bool, error) {
	var (
		index  int64
		isVLAN bool
		err    error
	)

	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		if strings.HasPrefix(line, netUEventIfIndexPrefix) {
			index, err = strconv.ParseInt(
				strings.TrimSpace(line[netUEventIfIndexPrefixLen:]), 0, 64)
			if err != nil {
				return -1, false, err
			}
			continue
		}

		if strings.HasPrefix(line, netUEventDevTypePrefix) {
			isVLAN = strings.EqualFold(strings.TrimSpace(line[netUEventDevTypePrefixLen:]),
				netUEventDevTypeVLAN)
		}
	}

	// Validate integer upper / lower bounds
	if index > 0 && index <= math.MaxInt {
		return int(index), isVLAN, nil
	}

	return -1, false, ErrIndexOutOfBounds
}

func addrTable(link *Link, msgs []syscall.NetlinkMessage) ([]net.IP, error) {
	var ifat []net.IP
loop:
	for _, m := range msgs {
		switch m.Header.Type {
		case syscall.NLMSG_DONE:
			break loop
		case syscall.RTM_NEWADDR:
			ifam := (*syscall.IfAddrmsg)(unsafe.Pointer(&m.Data[0])) // #nosec G103
			if link.Index == int(ifam.Index) {
				attrs, err := syscall.ParseNetlinkRouteAttr(&m)
				if err != nil {
					return nil, os.NewSyscallError("parsenetlinkrouteattr", err)
				}
				ifa := newAddr(ifam, attrs)
				if ifa != nil {
					ifat = append(ifat, ifa)
				}
			}
		}
	}
	return ifat, nil
}

func newAddr(ifam *syscall.IfAddrmsg, attrs []syscall.NetlinkRouteAttr) net.IP {
	var ipPointToPoint bool
	// Seems like we need to make sure whether the IP interface
	// stack consists of IP point-to-point numbered or unnumbered
	// addressing.
	for _, a := range attrs {
		if a.Attr.Type == syscall.IFA_LOCAL {
			ipPointToPoint = true
			break
		}
	}
	for _, a := range attrs {
		if ipPointToPoint && a.Attr.Type == syscall.IFA_ADDRESS {
			continue
		}
		switch ifam.Family {
		case syscall.AF_INET:
			return net.IPv4(a.Value[0], a.Value[1], a.Value[2], a.Value[3])
		case syscall.AF_INET6:
			ip := make(net.IP, net.IPv6len)
			copy(ip, a.Value[:])
			return ip
		}
	}
	return nil
}
