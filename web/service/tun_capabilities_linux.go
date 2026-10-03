//go:build linux

package service

import (
	"fmt"
	"os"
)

func detectTunCapabilities() TunCapabilityStatus {
	result := TunCapabilityStatus{Platform: "linux"}
	if _, err := os.Stat("/dev/net/tun"); err == nil {
		result.DeviceExists = true
	}
	result.Root = os.Geteuid() == 0
	if status, err := os.ReadFile("/proc/self/status"); err == nil {
		result.NetAdmin, result.NetRaw = linuxCapabilityFlags(string(status))
	}
	if result.Root {
		result.NetAdmin = true
		result.NetRaw = true
	}
	result.Ready = result.DeviceExists && result.NetAdmin && result.NetRaw
	if result.Ready {
		result.Detail = "TUN device and required Linux privileges are available"
	} else {
		result.Detail = fmt.Sprintf("/dev/net/tun=%t, root=%t, CAP_NET_ADMIN=%t, CAP_NET_RAW=%t", result.DeviceExists, result.Root, result.NetAdmin, result.NetRaw)
	}
	return result
}
