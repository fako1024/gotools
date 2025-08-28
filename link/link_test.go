//go:build linux
// +build linux

package link

import (
	"errors"
	"io/fs"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUp(t *testing.T) {
	link, err := New("lo")
	require.Nil(t, err)

	isUp, err := link.IsUp()
	require.Nil(t, err)
	require.True(t, isUp)
}

func TestNotExist(t *testing.T) {
	link, err := New("thisinterfacedoesnotexist")
	require.ErrorIs(t, err, ErrNotExist)
	require.Nil(t, link)
}

func TestFindAllLinks(t *testing.T) {
	links, err := HostLinks()

	if err != nil {
		t.Errorf("FindAllLinks() returned error: %v", err)
	}

	for _, link := range links {
		if link == nil {
			t.Errorf("FindAllLinks() returned nil link")
		}
	}
}

func TestGetLinkType(t *testing.T) {
	link, err := New("lo")
	require.Nil(t, err)

	require.Equal(t, TypeLoopback, link.Type)
}

func TestGetIPs(t *testing.T) {
	link, err := New("lo")
	require.Nil(t, err)

	ips, err := link.IPs()
	require.Nil(t, err)
	require.True(t, len(ips) > 0)
	require.Equal(t, net.IPv4(127, 0, 0, 1), ips[0])

	if len(ips) < 2 {
		t.Skip("Skipping IPv6 test, no IPv6 address assigned to loopback")
	}
	require.Equal(t, net.IPv6loopback, ips[1])
}

func TestIpHeaderOffset(t *testing.T) {
	tests := []struct {
		name     string
		linkType Type
		want     byte
	}{
		{"TypeEthernet", TypeEthernet, IPLayerOffsetEthernet},
		{"TypeLoopback", TypeLoopback, IPLayerOffsetEthernet},
		{"TypePPP", TypePPP, 0},
		{"TypeGRE", TypeGRE, 0},
		{"TypeNone", TypeNone, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.linkType.IPHeaderOffset(); got != tt.want {
				t.Errorf("IpHeaderOffset() = %v, want %v for link type %v", got, tt.want, tt.linkType)
			}
		})
	}
}

func TestLink_IpHeaderOffset(t *testing.T) {
	tests := []struct {
		name string
		l    Type
		want byte
	}{
		{
			name: "Test Ethernet link IP Header Offset",
			l:    TypeEthernet,
			want: IPLayerOffsetEthernet,
		},
		{
			name: "Test Loopback link IP Header Offset",
			l:    TypeLoopback,
			want: IPLayerOffsetEthernet,
		},
		{
			name: "Test PPP link IP Header Offset",
			l:    TypePPP,
			want: 0,
		},
		{
			name: "Test GRE link IP Header Offset",
			l:    TypeGRE,
			want: 0,
		},
		{
			name: "Test None link IP Header Offset",
			l:    TypeNone,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.IPHeaderOffset(); got != tt.want {
				t.Errorf("Link.IpHeaderOffset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLink_FindHostLinks(t *testing.T) {
	tests := []struct {
		name          string
		selectedLinks []string
		mockFn        func() ([]Link, error)
		wantErr       error
	}{
		{
			name: "Test Find All Links Success",
			mockFn: func() ([]Link, error) {
				return []Link{
					{Name: "eth0", Index: 1},
					{Name: "eth1", Index: 2},
					{Name: "lo", Index: 0},
				}, nil
			},
			wantErr: nil,
		},
		{
			name: "Test Find All Links Error",
			mockFn: func() ([]Link, error) {
				return nil, fs.ErrNotExist
			},
			wantErr: &fs.PathError{
				Op:   "open",
				Path: "/sys/class/net/",
				Err:  fs.ErrNotExist,
			},
		},
		{
			name:          "Test Find single Link Success",
			selectedLinks: []string{"lo"},
			mockFn: func() ([]Link, error) {
				return []Link{
					{Name: "lo", Index: 0},
				}, nil
			},
			wantErr: nil,
		},
		{
			name:          "Test Find single Link Error",
			selectedLinks: []string{"doesnotexist"},
			mockFn: func() ([]Link, error) {
				return nil, fs.ErrNotExist
			},
			wantErr: &fs.PathError{
				Op:   "open",
				Path: "/sys/class/net/doesnotexist/uevent",
				Err:  errors.New("no such file or directory"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netInterfaces = &mockInterfaces{mockFn: tt.mockFn}
			got, err := HostLinks(tt.selectedLinks...)
			if err != nil {
				if !assert.EqualError(t, err, tt.wantErr.Error()) {
					t.Errorf("Link_FindAllLinks() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			require.True(t, len(got) > 0)
			for _, l := range got {
				assert.NotNil(t, l)
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		ifName string
	}
	tests := []struct {
		name    string
		args    args
		mockFn  func(ifName string) (Type, error)
		want    *Link
		wantErr bool
	}{
		{
			name: "Test New Fail Interface not found",
			args: args{ifName: "ethDoesNotReallyExist"},
			mockFn: func(string) (Type, error) {
				return -1, fs.ErrNotExist
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test New Fail Interface not up",
			args: args{ifName: "ethDoesNotReallyExist"},
			mockFn: func(string) (Type, error) {
				return TypeEthernet, nil
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test New Fail Invalid Link Type",
			args: args{ifName: "ethDoesNotReallyExist"},
			mockFn: func(string) (Type, error) {
				return TypeInvalid, nil
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getLinkTypeF = tt.mockFn
			got, err := New(tt.args.ifName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Link.New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.ObjectsAreEqual(got, tt.want) {
				t.Errorf("Link.New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractIndexVLAN(t *testing.T) {

	tests := []struct {
		name       string
		data       []byte
		wantID     int
		wantIsVLAN bool
		wantErr    bool
	}{
		{
			name:       "Test nil uevents file",
			data:       nil,
			wantID:     -1,
			wantIsVLAN: false,
			wantErr:    true,
		},
		{
			name:       "Test empty uevents file",
			data:       []byte{},
			wantID:     -1,
			wantIsVLAN: false,
			wantErr:    true,
		},
		{
			name: "Test non-VLAN link",
			data: []byte(`INTERFACE=eth1
IFINDEX=42`),
			wantID:     42,
			wantIsVLAN: false,
			wantErr:    false,
		},
		{
			name: "Test VLAN link",
			data: []byte(`INTERFACE=eth1.100
IFINDEX=43
DEVTYPE=vlan`),
			wantID:     43,
			wantIsVLAN: true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linkID, isVLAN, err := extractIndexVLAN(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractIndexVLAN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !assert.Equal(t, linkID, tt.wantID) {
				t.Errorf("extractIndexVLAN() = %v, want %v", linkID, tt.wantID)
			}
			if !assert.Equal(t, isVLAN, tt.wantIsVLAN) {
				t.Errorf("extractIndexVLAN() = %v, want %v", isVLAN, tt.wantIsVLAN)
			}
		})
	}
}

func BenchmarkNewLink(b *testing.B) {
	b.Run("gotools/link", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			iface, _ := New("lo")
			_ = iface
		}
	})
	b.Run("net.Interface", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			iface, _ := net.InterfaceByName("lo")
			_ = iface
		}
	})
}

func BenchmarkGetIPs(b *testing.B) {
	b.Run("gotools/link", func(b *testing.B) {
		iface, _ := New("lo")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _ = iface.IPs()
		}
	})
	b.Run("net.Interface", func(b *testing.B) {
		iface, _ := net.InterfaceByName("lo")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _ = iface.Addrs()
		}
	})
}

type mockInterfaces struct {
	mockFn func() ([]Link, error)
}

var netInterfaces = &mockInterfaces{
	mockFn: func() ([]Link, error) {
		return nil, nil
	},
}

func (m *mockInterfaces) Interfaces() ([]Link, error) {
	return m.mockFn()
}

var getLinkTypeF = func(string) (Type, error) { return 1, nil }
