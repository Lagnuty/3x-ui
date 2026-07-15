package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
)

const defaultTrafficControlInterface = "eth0"

var (
	tcLimiterMu          sync.Mutex
	tcLimiterSignature   string
	tcLimiterWarnedNoTC  bool
	tcLimiterWarnedApply bool
)

type tcInboundLimit struct {
	Port     int
	UpMbps   int64
	DownMbps int64
}

// ApplyInboundSpeedLimits applies Linux traffic-control limits for inbound ports.
// It is intentionally port-scoped: Linux cannot distinguish VLESS/VMess/Trojan
// users that share one encrypted inbound port, but it can reliably police the
// port itself across the common protocols.
func (s *XrayService) ApplyInboundSpeedLimits() {
	if runtime.GOOS != "linux" {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("XUI_TC_SPEED_LIMIT_ENABLE")), "false") {
		return
	}

	tcPath, err := exec.LookPath("tc")
	if err != nil {
		if !tcLimiterWarnedNoTC {
			logger.Warning("tc speed limiter disabled: tc binary not found")
			tcLimiterWarnedNoTC = true
		}
		return
	}

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("tc speed limiter: failed to load inbounds:", err)
		return
	}

	limits := make([]tcInboundLimit, 0)
	for _, inbound := range inbounds {
		if inbound == nil || !inbound.Enable || inbound.Port <= 0 {
			continue
		}
		if !inboundSupportsTCSpeedLimit(inbound.Protocol) {
			continue
		}
		if inbound.SpeedLimitUpMbps <= 0 && inbound.SpeedLimitDownMbps <= 0 {
			continue
		}
		limits = append(limits, tcInboundLimit{
			Port:     inbound.Port,
			UpMbps:   inbound.SpeedLimitUpMbps,
			DownMbps: inbound.SpeedLimitDownMbps,
		})
	}
	sort.Slice(limits, func(i, j int) bool {
		return limits[i].Port < limits[j].Port
	})

	iface := strings.TrimSpace(os.Getenv("XUI_TC_INTERFACE"))
	if iface == "" {
		iface = defaultTrafficControlInterface
	}

	signature := tcLimiterHash(iface, limits)
	tcLimiterMu.Lock()
	defer tcLimiterMu.Unlock()
	if signature == tcLimiterSignature {
		return
	}

	if err := applyTCLimits(tcPath, iface, limits); err != nil {
		if !tcLimiterWarnedApply {
			logger.Warning("tc speed limiter failed:", err)
			tcLimiterWarnedApply = true
		}
		return
	}

	tcLimiterSignature = signature
	tcLimiterWarnedApply = false
	if len(limits) > 0 {
		logger.Infof("tc speed limiter applied on %s for %d inbound(s)", iface, len(limits))
	}
}

func tcLimiterHash(iface string, limits []tcInboundLimit) string {
	builder := strings.Builder{}
	builder.WriteString(iface)
	for _, limit := range limits {
		builder.WriteString(fmt.Sprintf("|%d:%d:%d", limit.Port, limit.UpMbps, limit.DownMbps))
	}
	sum := sha1.Sum([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func applyTCLimits(tcPath, iface string, limits []tcInboundLimit) error {
	_ = runTC(tcPath, "qdisc", "del", "dev", iface, "clsact")
	if len(limits) == 0 {
		return nil
	}
	if err := runTC(tcPath, "qdisc", "add", "dev", iface, "clsact"); err != nil {
		return err
	}

	for i, limit := range limits {
		prefBase := 1000 + i*20
		if limit.UpMbps > 0 {
			if err := addPortPolice(tcPath, iface, "ingress", prefBase, "dst_port", limit.Port, limit.UpMbps); err != nil {
				return err
			}
		}
		if limit.DownMbps > 0 {
			if err := addPortPolice(tcPath, iface, "egress", prefBase+10, "src_port", limit.Port, limit.DownMbps); err != nil {
				return err
			}
		}
	}
	return nil
}

func addPortPolice(tcPath, iface, direction string, pref int, portKey string, port int, mbps int64) error {
	for _, protocol := range []string{"ip", "ipv6"} {
		for offset, ipProto := range []string{"tcp", "udp"} {
			args := []string{
				"filter", "add", "dev", iface, direction,
				"protocol", protocol,
				"pref", strconv.Itoa(pref + offset),
				"flower",
				"ip_proto", ipProto,
				portKey, strconv.Itoa(port),
				"action", "police",
				"rate", fmt.Sprintf("%dmbit", mbps),
				"burst", tcBurst(mbps),
				"conform-exceed", "drop",
			}
			if err := runTC(tcPath, args...); err != nil {
				return err
			}
		}
		pref += 2
	}
	return nil
}

func tcBurst(mbps int64) string {
	if mbps < 1 {
		mbps = 1
	}
	burstKB := mbps * 128
	if burstKB < 64 {
		burstKB = 64
	}
	if burstKB > 4096 {
		burstKB = 4096
	}
	return fmt.Sprintf("%dk", burstKB)
}

func runTC(tcPath string, args ...string) error {
	cmd := exec.Command(tcPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func inboundSupportsTCSpeedLimit(protocol model.Protocol) bool {
	switch protocol {
	case model.VMESS, model.VLESS, model.Trojan, model.Shadowsocks, model.HTTP, model.Mixed, model.WireGuard, model.Hysteria:
		return true
	default:
		return false
	}
}
