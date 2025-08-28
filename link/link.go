//go:build linux
// +build linux

package link

import (
	"errors"
	"io/fs"
)

var (

	// ErrNotExist denotes that the interface in question does not exist
	ErrNotExist = errors.New("interface does not exist")

	// ErrNotUp denotes that the interface in question is not up
	ErrNotUp = errors.New("interface is currently not up")
)

// EmptyEthernetLink provides a quick access to a plain / empty ethernet-type link
var EmptyEthernetLink = Link{
	Type: TypeEthernet,
}

// Link is the low-level representation of a network interface
type Link struct {
	Name   string
	Index  int
	Type   Type
	IsVLAN bool
}

// New instantiates a new link / interface
// It will return an error if the interface does not exist or is not up
func New(name string, opts ...func(*Link)) (link *Link, err error) {

	link, lerr := newLink(name)
	if lerr != nil {
		if errors.Is(lerr, fs.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, lerr
	}

	isUp, uerr := link.IsUp()
	if uerr != nil {
		if errors.Is(uerr, fs.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, uerr
	}

	if !isUp {
		return nil, ErrNotUp
	}

	// Apply functional options, if any
	for _, opt := range opts {
		opt(link)
	}

	return
}

// String returns the name of the network interface (Stringer interface)
func (l Link) String() string {
	return l.Name
}
