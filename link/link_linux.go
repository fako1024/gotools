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
		var err error
		names, err = hostLinkNamesFromSysfs()
		if err != nil {
			if isSysfsUnavailableErr(err) {
				return hostLinksFromNetlink()
			}
			return nil, err
		}

		return hostLinksByNames(names)
	}

	if err := ensureSysfsNetBaseReadable(); err != nil {
		if isSysfsUnavailableErr(err) {
			return hostLinksFromNetlink(names...)
		}
		return nil, err
	}

	return hostLinksByNames(names)
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

func hostLinkNamesFromSysfs() (names []string, err error) {
	linkDir, err := os.Open(netBasePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := linkDir.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	dirEnts, err := linkDir.ReadDir(-1)
	if err != nil {
		return nil, err
	}

	names = make([]string, 0, len(dirEnts))
	for _, ent := range dirEnts {
		mode := ent.Type()
		if mode&os.ModeSymlink != 0 {
			names = append(names, ent.Name())
			continue
		}

		if mode == 0 {
			info, ierr := os.Lstat(netBasePath + ent.Name())
			if ierr != nil {
				return nil, ierr
			}

			if info.Mode()&os.ModeSymlink != 0 {
				names = append(names, ent.Name())
			}
		}
	}

	return names, nil
}

func ensureSysfsNetBaseReadable() error {
	linkDir, err := os.Open(netBasePath)
	if err != nil {
		return err
	}
	return linkDir.Close()
}

func hostLinksByNames(names []string) (Links, error) {
	ifaces := make([]*Link, len(names))
	for i, name := range names {
		link, err := newLink(name)
		if err != nil {
			return nil, err
		}
		ifaces[i] = link
	}

	return ifaces, nil
}

func hostLinksFromNetlink(names ...string) (Links, error) {
	links, err := parseNetlinkLinks()
	if err != nil {
		return nil, err
	}

	if len(names) == 0 {
		return links, nil
	}

	return selectLinksByName(links, names)
}

func parseNetlinkLinks() (Links, error) {
	tab, err := syscall.NetlinkRIB(syscall.RTM_GETLINK, syscall.AF_UNSPEC)
	if err != nil {
		return nil, os.NewSyscallError("netlinkrib", err)
	}

	msgs, err := syscall.ParseNetlinkMessage(tab)
	if err != nil {
		return nil, os.NewSyscallError("parsenetlinkmessage", err)
	}

	links := make(Links, 0, len(msgs))
loop:
	for _, m := range msgs {
		switch m.Header.Type {
		case syscall.NLMSG_DONE:
			break loop
		case syscall.RTM_NEWLINK:
			if len(m.Data) < syscall.SizeofIfInfomsg {
				continue
			}

			ifim := (*syscall.IfInfomsg)(unsafe.Pointer(&m.Data[0])) // #nosec G103
			attrs, err := syscall.ParseNetlinkRouteAttr(&m)
			if err != nil {
				return nil, os.NewSyscallError("parsenetlinkrouteattr", err)
			}

			link := &Link{
				Index: int(ifim.Index),
				Type:  Type(ifim.Type),
			}

			for _, attr := range attrs {
				switch routeAttrType(attr.Attr.Type) {
				case syscall.IFLA_IFNAME:
					link.Name = strings.TrimRight(string(attr.Value), "\x00")
				case syscall.IFLA_LINKINFO:
					link.IsVLAN = parseLinkInfoIsVLAN(attr.Value)
				}
			}

			if link.Name != "" {
				links = append(links, link)
			}
		}
	}

	return links, nil
}

func selectLinksByName(links Links, names []string) (Links, error) {
	linksByName := make(map[string]*Link, len(links))
	for _, link := range links {
		if link == nil || link.Name == "" {
			continue
		}
		if _, ok := linksByName[link.Name]; !ok {
			linksByName[link.Name] = link
		}
	}

	selectedLinks := make(Links, len(names))
	for i, name := range names {
		link, ok := linksByName[name]
		if !ok {
			return nil, &os.PathError{
				Op:   "open",
				Path: netBasePath + name + netUEventPath,
				Err:  os.ErrNotExist,
			}
		}

		linkCpy := *link
		selectedLinks[i] = &linkCpy
	}

	return selectedLinks, nil
}

func routeAttrType(attrType uint16) uint16 {
	return attrType & ^uint16(unix.NLA_F_NESTED|unix.NLA_F_NET_BYTEORDER)
}

func parseRouteAttrs(data []byte) ([]syscall.NetlinkRouteAttr, error) {
	attrs := make([]syscall.NetlinkRouteAttr, 0, 4)
	for len(data) >= syscall.SizeofRtAttr {
		attr := (*syscall.RtAttr)(unsafe.Pointer(&data[0])) // #nosec G103
		if int(attr.Len) < syscall.SizeofRtAttr || int(attr.Len) > len(data) {
			return nil, syscall.EINVAL
		}

		attrs = append(attrs, syscall.NetlinkRouteAttr{
			Attr:  *attr,
			Value: data[syscall.SizeofRtAttr:int(attr.Len)],
		})

		next := (int(attr.Len) + syscall.RTA_ALIGNTO - 1) & ^(syscall.RTA_ALIGNTO - 1)
		if next > len(data) {
			return nil, syscall.EINVAL
		}
		data = data[next:]
	}

	return attrs, nil
}

func parseLinkInfoIsVLAN(data []byte) bool {
	attrs, err := parseRouteAttrs(data)
	if err != nil {
		return false
	}

	for _, attr := range attrs {
		if routeAttrType(attr.Attr.Type) != unix.IFLA_INFO_KIND {
			continue
		}

		if strings.EqualFold(strings.TrimRight(string(attr.Value), "\x00"), netUEventDevTypeVLAN) {
			return true
		}
	}

	return false
}

func isSysfsUnavailableErr(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, syscall.ENOTDIR) ||
		errors.Is(err, syscall.EACCES) ||
		errors.Is(err, syscall.EPERM)
}
