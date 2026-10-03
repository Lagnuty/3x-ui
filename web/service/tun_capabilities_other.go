//go:build !linux && !windows

package service

import "runtime"

func detectTunCapabilities() TunCapabilityStatus {
	return TunCapabilityStatus{
		Platform: runtime.GOOS,
		Detail:   "Automatic TUN privilege detection is not available on this platform",
	}
}
