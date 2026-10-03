//go:build windows

package service

import (
	"runtime"

	"golang.org/x/sys/windows"
)

func detectTunCapabilities() TunCapabilityStatus {
	elevated := windows.GetCurrentProcessToken().IsElevated()
	detail := "Windows process is not elevated; run the panel as Administrator and install a supported TUN driver"
	if elevated {
		detail = "Windows process is elevated; TUN driver availability is verified when Xray starts"
	}
	return TunCapabilityStatus{
		Platform: runtime.GOOS,
		Root:     elevated,
		NetAdmin: elevated,
		NetRaw:   elevated,
		Ready:    elevated,
		Detail:   detail,
	}
}
