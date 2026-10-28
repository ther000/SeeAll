//go:build windows

package system

import (
	"syscall"
	"unsafe"
)

// Which link the machine is talking over, through iphlpapi. The dashboard's
// network card says "Wi-Fi" or "以太网" beside the rate, and the rate itself
// sums every adapter, so what is named here is the adapter Windows would send
// a packet out of — not the one that happens to be listed first.

var iphlpapi = syscall.NewLazyDLL("iphlpapi.dll")

var (
	procGetAdaptersAddresses = iphlpapi.NewProc("GetAdaptersAddresses")
	procGetBestInterface     = iphlpapi.NewProc("GetBestInterface")
)

const (
	// afUnspec asks for both address families: IPv6-only adapters are
	// adapters too.
	afUnspec = 0
	// The address lists themselves are not read, so they are not fetched.
	gaaSkipAnycast   = 0x0002
	gaaSkipMulticast = 0x0004
	gaaSkipDNSServer = 0x0008

	errorBufferOverflow = 111

	ifTypeEthernet  = 6
	ifTypeLoopback  = 24
	ifTypeIEEE80211 = 71
	ifTypeTunnel    = 131
	ifOperStatusUp  = 1
)

// adapterPrefix is the front of IP_ADAPTER_ADDRESSES: enough to tell one
// adapter's index and kind, which is all the badge needs.
type adapterPrefix struct {
	Length                uint32
	IfIndex               uint32
	Next                  *adapterPrefix
	AdapterName           *byte
	FirstUnicastAddress   uintptr
	FirstAnycastAddress   uintptr
	FirstMulticastAddress uintptr
	FirstDnsServerAddress uintptr
	DnsSuffix             *uint16
	Description           *uint16
	FriendlyName          *uint16
	PhysicalAddress       [8]byte
	PhysicalAddressLength uint32
	Flags                 uint32
	Mtu                   uint32
	IfType                uint32
	OperStatus            uint32
}

// netKind names the link carrying the machine's traffic, or "" when no adapter
// is up. A machine with only a tunnel up has no wired or wireless link to
// name, and the card then says "全部网卡".
func netKind() string {
	adapters := networkAdapters()
	if len(adapters) == 0 {
		return ""
	}
	if best, ok := bestInterfaceIndex(); ok {
		for _, a := range adapters {
			if a.IfIndex == best {
				return linkName(a)
			}
		}
	}
	// Windows could not say which route it uses: the first adapter that is
	// really up answers the same question, less exactly.
	for _, a := range adapters {
		if a.OperStatus == ifOperStatusUp && a.IfType != ifTypeLoopback && a.IfType != ifTypeTunnel {
			if name := linkName(a); name != "" {
				return name
			}
		}
	}
	return ""
}

// linkName is what the badge calls an adapter's technology.
func linkName(a adapterPrefix) string {
	if a.OperStatus != ifOperStatusUp {
		return ""
	}
	switch a.IfType {
	case ifTypeIEEE80211:
		return "Wi-Fi"
	case ifTypeEthernet:
		return "以太网"
	}
	return ""
}

// bestInterfaceIndex is the adapter Windows would route an internet-bound
// packet out of. 8.8.8.8 is only a destination here: nothing is sent.
func bestInterfaceIndex() (uint32, bool) {
	var index uint32
	// An IPAddr is an address in network order; 8.8.8.8 is the same bytes
	// whichever way it is written.
	const destination = 0x08080808
	if status, _, _ := procGetBestInterface.Call(uintptr(destination), uintptr(unsafe.Pointer(&index))); status != 0 {
		return 0, false
	}
	return index, true
}

// networkAdapters lists the adapters Windows has, in its order.
func networkAdapters() []adapterPrefix {
	flags := uintptr(gaaSkipAnycast | gaaSkipMulticast | gaaSkipDNSServer)
	size := uint32(16 << 10)
	for attempt := 0; attempt < 3; attempt++ {
		buf := make([]byte, size)
		status, _, _ := procGetAdaptersAddresses.Call(uintptr(afUnspec), flags, 0,
			uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if status == 0 {
			var out []adapterPrefix
			for a := (*adapterPrefix)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
				out = append(out, *a)
			}
			return out
		}
		if status != errorBufferOverflow {
			// No adapter at all is a fact about the machine, not a failure.
			return nil
		}
	}
	return nil
}
