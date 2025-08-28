//go:build linux
// +build linux

package link

import "net"

// Links is a slice of Link pointers
type Links []*Link

// IPs returns all IP addresses assigned to all links in the slice
func (l Links) IPs() (map[string][]net.IP, error) {
	res := make(map[string][]net.IP)
	for _, link := range l {
		ips, err := link.IPs()
		if err != nil {
			return nil, err
		}
		res[link.Name] = ips
	}
	return res, nil
}
