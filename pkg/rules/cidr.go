package rules

import (
	"encoding/binary"
	"net"
	"strings"
)

type IPAddr []uint32

func NewIPAddr(ip net.IP) IPAddr {
	if ip == nil {
		return nil
	}
	tmpIp := ip.To4()
	length := 1
	if tmpIp == nil {
		tmpIp = ip.To16()
		length = 4
	}
	if tmpIp == nil {
		return nil
	}
	addr := make(IPAddr, length)
	for i := 0; i < length; i++ {
		idx := i * net.IPv4len
		addr[i] = binary.BigEndian.Uint32(tmpIp[idx : idx+net.IPv4len])
	}
	return addr
}

func NetworkContains(network net.IPNet, ip net.IP) bool {
	addr := NewIPAddr(ip)
	number := NewIPAddr(network.IP)
	mask := NewIPAddr(net.IP(network.Mask))
	if len(mask) != len(addr) {
		return false
	}
	if addr[0]&mask[0] != number[0] {
		return false
	}
	if len(addr) == 4 {
		return addr[1]&mask[1] == number[1] &&
			addr[2]&mask[2] == number[2] &&
			addr[3]&mask[3] == number[3]
	}
	return true
}

func IsCIDR(s string) bool {
	if !strings.Contains(s, "/") {
		return false
	}
	_, _, err := net.ParseCIDR(s)
	return err == nil
}
