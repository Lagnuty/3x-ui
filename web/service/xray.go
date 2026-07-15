package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"go.uber.org/atomic"
)

var (
	p                 *xray.Process
	lock              sync.Mutex
	isNeedXrayRestart atomic.Bool // Indicates that restart was requested for Xray
	isManuallyStopped atomic.Bool // Indicates that Xray was stopped manually from the panel
	result            string
)

// XrayService provides business logic for Xray process management.
// It handles starting, stopping, restarting Xray, and managing its configuration.
type XrayService struct {
	inboundService InboundService
	settingService SettingService
	xrayAPI        xray.XrayAPI
}

// IsXrayRunning checks if the Xray process is currently running.
func (s *XrayService) IsXrayRunning() bool {
	return p != nil && p.IsRunning()
}

func XrayProcess() *xray.Process {
	return p
}

// GetXrayErr returns the error from the Xray process, if any.
func (s *XrayService) GetXrayErr() error {
	if p == nil {
		return nil
	}

	err := p.GetErr()
	if err == nil {
		return nil
	}

	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

// GetXrayResult returns the result string from the Xray process.
func (s *XrayService) GetXrayResult() string {
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	if p == nil {
		return ""
	}

	result = p.GetResult()

	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}

	return result
}

// GetXrayVersion returns the version of the running Xray process.
func (s *XrayService) GetXrayVersion() string {
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

// RemoveIndex removes an element at the specified index from a slice.
// Returns a new slice with the element removed.
func RemoveIndex(s []any, index int) []any {
	return append(s[:index], s[index+1:]...)
}

// GetXrayConfig retrieves and builds the Xray configuration from settings and inbounds.
func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}
	if err := s.mergeOutboundSubscriptions(xrayConfig); err != nil {
		return nil, err
	}
	if egressTag, err := s.settingService.GetPanelOutbound(); err != nil {
		logger.Warning("read panelOutbound setting failed:", err)
	} else if strings.TrimSpace(egressTag) != "" {
		injectPanelEgress(xrayConfig, strings.TrimSpace(egressTag))
	}

	s.inboundService.AddTraffic(nil, nil)

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		// get settings clients
		settings := map[string]any{}
		json.Unmarshal([]byte(inbound.Settings), &settings)
		clients, ok := settings["clients"].([]any)
		if ok {
			// check users active or not
			clientStats := inbound.ClientStats
			for _, clientTraffic := range clientStats {
				indexDecrease := 0
				for index, client := range clients {
					c := client.(map[string]any)
					if c["email"] == clientTraffic.Email {
						if !clientTraffic.Enable {
							clients = RemoveIndex(clients, index-indexDecrease)
							indexDecrease++
							logger.Infof("Remove Inbound User %s due to expiration or traffic limit", c["email"])
						}
					}
				}
			}

			// clear client config for additional parameters
			var final_clients []any
			for _, client := range clients {
				c := client.(map[string]any)
				if c["enable"] != nil {
					if enable, ok := c["enable"].(bool); ok && !enable {
						continue
					}
				}
				for key := range c {
					if key != "email" && key != "id" && key != "password" && key != "flow" && key != "method" && key != "auth" && key != "speedLimitUpMbps" && key != "speedLimitDownMbps" {
						delete(c, key)
					}
					if c["flow"] == "xtls-rprx-vision-udp443" {
						c["flow"] = "xtls-rprx-vision"
					}
				}
				final_clients = append(final_clients, any(c))
			}

			settings["clients"] = final_clients
			modifiedSettings, err := json.MarshalIndent(settings, "", "  ")
			if err != nil {
				return nil, err
			}

			inbound.Settings = string(modifiedSettings)
		}

		if inbound.Protocol == model.Hysteria {
			s.syncHysteriaRuntimeAuth(inbound, clients)
		}

		if len(inbound.StreamSettings) > 0 {
			// Unmarshal stream JSON
			var stream map[string]any
			json.Unmarshal([]byte(inbound.StreamSettings), &stream)

			// Remove the "settings" field under "tlsSettings" and "realitySettings"
			tlsSettings, ok1 := stream["tlsSettings"].(map[string]any)
			realitySettings, ok2 := stream["realitySettings"].(map[string]any)
			if ok1 || ok2 {
				if ok1 {
					delete(tlsSettings, "settings")
				} else if ok2 {
					delete(realitySettings, "settings")
				}
			}

			delete(stream, "externalProxy")

			newStream, err := json.MarshalIndent(stream, "", "  ")
			if err != nil {
				return nil, err
			}
			inbound.StreamSettings = string(newStream)
		}

		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func (s *XrayService) syncHysteriaRuntimeAuth(inbound *model.Inbound, clients []any) {
	if len(clients) == 0 {
		return
	}

	auth := ""
	speedLimitUpMbps := int64(0)
	speedLimitDownMbps := int64(0)
	for _, client := range clients {
		c, ok := client.(map[string]any)
		if !ok {
			continue
		}
		if enable, ok := c["enable"].(bool); ok && !enable {
			continue
		}
		if value, ok := c["auth"].(string); ok && strings.TrimSpace(value) != "" {
			auth = strings.TrimSpace(value)
			speedLimitUpMbps = int64FromAny(c["speedLimitUpMbps"])
			speedLimitDownMbps = int64FromAny(c["speedLimitDownMbps"])
			break
		}
	}
	if auth == "" {
		return
	}

	stream := map[string]any{}
	if strings.TrimSpace(inbound.StreamSettings) != "" {
		if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
			logger.Warningf("Unable to parse Hysteria stream settings for inbound %d: %v", inbound.Id, err)
			return
		}
	}
	if stream == nil {
		stream = map[string]any{}
	}
	stream["network"] = "hysteria"
	stream["security"] = "tls"

	hysteriaSettings, ok := stream["hysteriaSettings"].(map[string]any)
	if !ok {
		hysteriaSettings = map[string]any{}
		stream["hysteriaSettings"] = hysteriaSettings
	}
	hysteriaSettings["version"] = 2
	hysteriaSettings["auth"] = auth
	if speedLimitUpMbps > 0 {
		hysteriaSettings["up"] = fmt.Sprintf("%d mbps", speedLimitUpMbps)
	} else {
		delete(hysteriaSettings, "up")
	}
	if speedLimitDownMbps > 0 {
		hysteriaSettings["down"] = fmt.Sprintf("%d mbps", speedLimitDownMbps)
	} else {
		delete(hysteriaSettings, "down")
	}
	if _, ok := hysteriaSettings["udpIdleTimeout"]; !ok {
		hysteriaSettings["udpIdleTimeout"] = 60
	}

	raw, err := json.MarshalIndent(stream, "", "  ")
	if err != nil {
		logger.Warningf("Unable to marshal Hysteria stream settings for inbound %d: %v", inbound.Id, err)
		return
	}
	inbound.StreamSettings = string(raw)
}

func int64FromAny(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	default:
		return 0
	}
}

func (s *XrayService) mergeOutboundSubscriptions(xrayConfig *xray.Config) error {
	subService := &OutboundSubscriptionService{}
	prepend, appendList, err := subService.ActiveOutboundsSplit()
	if err != nil {
		return err
	}
	if len(prepend) == 0 && len(appendList) == 0 {
		return nil
	}

	var manual []any
	if len(xrayConfig.OutboundConfigs) > 0 {
		if err := json.Unmarshal(xrayConfig.OutboundConfigs, &manual); err != nil {
			return err
		}
	}
	merged := make([]any, 0, len(prepend)+len(manual)+len(appendList))
	seen := map[string]bool{}
	appendUnique := func(rows []any) {
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				merged = append(merged, raw)
				continue
			}
			tag, _ := row["tag"].(string)
			if tag != "" {
				if seen[tag] {
					continue
				}
				seen[tag] = true
			}
			merged = append(merged, row)
		}
	}
	appendUnique(prepend)
	appendUnique(manual)
	appendUnique(appendList)
	raw, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	xrayConfig.OutboundConfigs = raw
	return nil
}

// GetXrayTraffic fetches the current traffic statistics from the running Xray process.
func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !s.IsXrayRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	apiPort := p.GetAPIPort()
	s.xrayAPI.Init(apiPort)
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic(true)
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

// RestartXray restarts the Xray process, optionally forcing a restart even if config unchanged.
func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart Xray, force:", isForce)
	isManuallyStopped.Store(false)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if s.IsXrayRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) && !isNeedXrayRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		p.Stop()
	}

	p = xray.NewProcess(xrayConfig)
	result = ""
	err = p.Start()
	if err != nil {
		return err
	}

	return nil
}

// StopXray stops the running Xray process.
func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	isManuallyStopped.Store(true)
	logger.Debug("Attempting to stop Xray...")
	if s.IsXrayRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

// SetToNeedRestart marks that Xray needs to be restarted.
func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

// IsNeedRestartAndSetFalse checks if restart is needed and resets the flag to false.
func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CompareAndSwap(true, false)
}

// DidXrayCrash checks if Xray crashed by verifying it's not running and wasn't manually stopped.
func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !isManuallyStopped.Load()
}

const PanelEgressInboundTag = "panel-egress"
const panelEgressBasePort = 62790

func injectPanelEgress(cfg *xray.Config, outboundTag string) {
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == PanelEgressInboundTag {
			logger.Warning("panel egress: inbound tag [", PanelEgressInboundTag, "] already exists, skipping injection")
			return
		}
	}
	routing := map[string]any{}
	if len(cfg.RouterConfig) > 0 {
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			logger.Warning("panel egress: routing section is unparsable, skipping injection:", err)
			return
		}
	}
	rules, _ := routing["rules"].([]any)
	rule := map[string]any{"type": "field", "inboundTag": []any{PanelEgressInboundTag}}
	if routingTagIsBalancer(routing, outboundTag) {
		rule["balancerTag"] = outboundTag
	} else {
		rule["outboundTag"] = outboundTag
	}
	routing["rules"] = append([]any{rule}, rules...)
	newRouting, err := json.Marshal(routing)
	if err != nil {
		logger.Warning("panel egress: failed to rebuild routing section, skipping injection:", err)
		return
	}
	cfg.RouterConfig = json_util.RawMessage(newRouting)

	used := make(map[int]struct{}, len(cfg.InboundConfigs))
	for i := range cfg.InboundConfigs {
		used[cfg.InboundConfigs[i].Port] = struct{}{}
	}
	port := panelEgressBasePort
	for {
		if _, taken := used[port]; !taken {
			break
		}
		port++
	}
	cfg.InboundConfigs = append(cfg.InboundConfigs, xray.InboundConfig{
		Listen:   json_util.RawMessage(`"127.0.0.1"`),
		Port:     port,
		Protocol: "socks",
		Settings: json_util.RawMessage(`{"auth":"noauth","udp":false}`),
		Tag:      PanelEgressInboundTag,
	})
}

func routingTagIsBalancer(routing map[string]any, tag string) bool {
	if tag == "" {
		return false
	}
	balancers, ok := routing["balancers"].([]any)
	if !ok {
		return false
	}
	for _, b := range balancers {
		bm, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := bm["tag"].(string); ok && t == tag {
			return true
		}
	}
	return false
}
