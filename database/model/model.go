// Package model defines the database models and data structures used by the 3x-ui panel.
package model

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// Protocol represents the protocol type for Xray inbounds.
type Protocol string

// Protocol constants for different Xray inbound protocols
const (
	VMESS       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Tunnel      Protocol = "tunnel"
	HTTP        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Mixed       Protocol = "mixed"
	WireGuard   Protocol = "wireguard"
	// UI stores Hysteria v1 and v2 both as "hysteria" and uses
	// settings.version to discriminate. Imports from outside the panel
	// can carry the literal "hysteria2" string, so IsHysteria below
	// accepts both.
	Hysteria  Protocol = "hysteria"
	Hysteria2 Protocol = "hysteria2"
)

// IsHysteria returns true for both "hysteria" and "hysteria2".
// Use instead of a bare ==model.Hysteria check: a v2 inbound stored
// with the literal v2 string would otherwise fall through (#4081).
func IsHysteria(p Protocol) bool {
	return p == Hysteria || p == Hysteria2
}

// User represents a user account in the 3x-ui panel.
type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Inbound represents an Xray inbound configuration with traffic statistics and settings.
type Inbound struct {
	Id                   int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`                                                    // Unique identifier
	UserId               int                  `json:"-"`                                                                                               // Associated user ID
	Up                   int64                `json:"up" form:"up"`                                                                                    // Upload traffic in bytes
	Down                 int64                `json:"down" form:"down"`                                                                                // Download traffic in bytes
	Total                int64                `json:"total" form:"total"`                                                                              // Total traffic limit in bytes
	AllTime              int64                `json:"allTime" form:"allTime" gorm:"default:0"`                                                         // All-time traffic usage
	Remark               string               `json:"remark" form:"remark"`                                                                            // Human-readable remark
	Enable               bool                 `json:"enable" form:"enable" gorm:"index:idx_enable_traffic_reset,priority:1"`                           // Whether the inbound is enabled
	ExpiryTime           int64                `json:"expiryTime" form:"expiryTime"`                                                                    // Expiration timestamp
	TrafficReset         string               `json:"trafficReset" form:"trafficReset" gorm:"default:never;index:idx_enable_traffic_reset,priority:2"` // Traffic reset schedule
	LastTrafficResetTime int64                `json:"lastTrafficResetTime" form:"lastTrafficResetTime" gorm:"default:0"`                               // Last traffic reset timestamp
	ClientStats          []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`                        // Client traffic statistics

	// Xray configuration fields
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

// OutboundTraffics tracks traffic statistics for Xray outbound connections.
type OutboundTraffics struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag   string `json:"tag" form:"tag" gorm:"unique"`
	Up    int64  `json:"up" form:"up" gorm:"default:0"`
	Down  int64  `json:"down" form:"down" gorm:"default:0"`
	Total int64  `json:"total" form:"total" gorm:"default:0"`
}

// InboundClientIps stores IP addresses associated with inbound clients for access control.
type InboundClientIps struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientEmail string `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	Ips         string `json:"ips" form:"ips"`
}

// HistoryOfSeeders tracks which database seeders have been executed to prevent re-running.
type HistoryOfSeeders struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SeederName string `json:"seederName"`
}

// GenXrayInboundConfig generates an Xray inbound configuration from the Inbound model.
func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	migration := MigrateInboundConfig(i.Protocol, i.Settings, i.StreamSettings)
	listen := i.Listen
	// Default to 0.0.0.0 (all interfaces) when listen is empty
	// This ensures proper dual-stack IPv4/IPv6 binding in systems where bindv6only=0
	if listen == "" {
		listen = "0.0.0.0"
	}
	listen = fmt.Sprintf("\"%v\"", listen)
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(migration.Protocol),
		Settings:       json_util.RawMessage(migration.Settings),
		StreamSettings: json_util.RawMessage(migration.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

func normalizeInboundSettings(protocol Protocol, settings string) string {
	var cfg map[string]any
	if err := json.Unmarshal([]byte(settings), &cfg); err != nil {
		return settings
	}
	changed := false
	if protocol == WireGuard {
		for _, key := range []string{"workers", "num_workers"} {
			if _, exists := cfg[key]; exists {
				delete(cfg, key)
				changed = true
			}
		}
	}
	if IsHysteria(protocol) {
		if _, exists := cfg["clients"]; !exists {
			if users, ok := cfg["users"].([]any); ok {
				cfg["clients"] = users
				changed = true
			} else if auth, ok := cfg["auth"].(string); ok && strings.TrimSpace(auth) != "" {
				cfg["clients"] = []any{map[string]any{"auth": auth}}
				changed = true
			}
		}
		if protocol == Hysteria2 {
			if _, exists := cfg["version"]; !exists {
				cfg["version"] = 2
				changed = true
			}
		}
		if _, exists := cfg["users"]; exists {
			delete(cfg, "users")
			changed = true
		}
		if _, exists := cfg["auth"]; exists {
			delete(cfg, "auth")
			changed = true
		}
	}
	if !changed {
		return settings
	}

	normalized, err := json.Marshal(cfg)
	if err != nil {
		return settings
	}
	return string(normalized)
}

func normalizeStreamSettings(streamSettings string) string {
	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return streamSettings
	}

	changed := normalizeFinalMaskStreamSettings(stream)
	if normalizeXHTTPStreamSettings(stream) {
		changed = true
	}
	if normalizeRealityStreamSettings(stream) {
		changed = true
	}
	if normalizeTLSStreamSettings(stream) {
		changed = true
	}
	if normalizeHysteriaStreamSettings(stream) {
		changed = true
	}
	if !changed {
		return streamSettings
	}

	normalized, err := json.Marshal(stream)
	if err != nil {
		return streamSettings
	}
	return string(normalized)
}

func normalizeXHTTPStreamSettings(stream map[string]any) bool {
	xhttp, ok := stream["xhttpSettings"].(map[string]any)
	if !ok || xhttp == nil {
		return false
	}
	changed := normalizeXHTTPCoreDefaults(xhttp)
	for _, suffix := range []string{"Placement", "Key", "Table", "Length"} {
		legacy := "session" + suffix
		current := "sessionID" + suffix
		if value, exists := xhttp[legacy]; exists {
			if _, hasCurrent := xhttp[current]; !hasCurrent {
				xhttp[current] = value
			}
			delete(xhttp, legacy)
			changed = true
		}
	}
	return changed
}

func normalizeRealityStreamSettings(stream map[string]any) bool {
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality == nil {
		return false
	}
	// 26.3.27 was the old built-in floor copied by older panel versions.
	// Since v26.9.8 an absent/empty value means that no minimum is enforced.
	if value, ok := reality["minClientVer"].(string); ok && strings.TrimSpace(value) == "26.3.27" {
		delete(reality, "minClientVer")
		return true
	}
	return false
}

func normalizeTLSStreamSettings(stream map[string]any) bool {
	tls, _ := stream["tlsSettings"].(map[string]any)
	if tls == nil {
		return false
	}
	client, _ := tls["settings"].(map[string]any)
	if client == nil {
		client = map[string]any{}
	}
	changed := false
	for _, key := range []string{"verifyPeerCertByName", "pinnedPeerCertSha256"} {
		if value, exists := tls[key]; exists {
			if _, current := client[key]; !current {
				client[key] = value
			}
			delete(tls, key)
			changed = true
		}
	}
	for _, legacy := range []string{"pinnedPeerCertificateSha256", "pinnedPeerCertificateChainSha256"} {
		if value, exists := client[legacy]; exists {
			if _, current := client["pinnedPeerCertSha256"]; !current {
				client["pinnedPeerCertSha256"] = value
			}
			delete(client, legacy)
			changed = true
		}
	}
	for _, target := range []map[string]any{tls, client} {
		if _, exists := target["allowInsecure"]; exists {
			delete(target, "allowInsecure")
			changed = true
		}
	}
	if len(client) > 0 {
		tls["settings"] = client
	}
	return changed
}

func normalizeHysteriaStreamSettings(stream map[string]any) bool {
	changed := false
	legacyV2 := false
	if _, exists := stream["hysteriaSettings"]; !exists {
		for _, legacy := range []string{"hysteria2Settings", "hy2Settings"} {
			if value, ok := stream[legacy].(map[string]any); ok {
				stream["hysteriaSettings"] = value
				delete(stream, legacy)
				changed = true
				legacyV2 = true
				break
			}
		}
	}
	if network, _ := stream["network"].(string); network == "hysteria2" {
		stream["network"] = "hysteria"
		changed = true
		legacyV2 = true
	}
	hysteria, _ := stream["hysteriaSettings"].(map[string]any)
	if hysteria == nil {
		return changed
	}
	for legacy, current := range map[string]string{
		"authString":        "auth",
		"udpIdleTimeoutSec": "udpIdleTimeout",
	} {
		if value, exists := hysteria[legacy]; exists {
			if _, currentExists := hysteria[current]; !currentExists {
				hysteria[current] = value
			}
			delete(hysteria, legacy)
			changed = true
		}
	}
	if _, exists := hysteria["version"]; !exists && legacyV2 {
		hysteria["version"] = 2
		changed = true
	}
	return changed
}

// ConfigMigrationResult describes the persistent normalization of one inbound.
type ConfigMigrationResult struct {
	Protocol       Protocol
	Settings       string
	StreamSettings string
	Warnings       []string
	Changed        bool
}

// MigrateInboundConfig centralizes stored-config migrations and diagnostics.
func MigrateInboundConfig(protocol Protocol, settings, streamSettings string) ConfigMigrationResult {
	result := ConfigMigrationResult{Protocol: protocol, Settings: settings, StreamSettings: streamSettings}
	result.Settings = normalizeInboundSettings(protocol, settings)
	result.StreamSettings = normalizeStreamSettings(streamSettings)
	if protocol == Hysteria2 {
		result.Protocol = Hysteria
	}
	result.Changed = result.Protocol != protocol || result.Settings != settings || result.StreamSettings != streamSettings

	var before, after map[string]any
	if json.Unmarshal([]byte(streamSettings), &before) == nil && json.Unmarshal([]byte(result.StreamSettings), &after) == nil {
		if hasLegacyAllowInsecure(before) && !hasLegacyAllowInsecure(after) {
			result.Warnings = append(result.Warnings, "removed deprecated TLS allowInsecure; configure certificate pinning or name verification")
		}
		if finalMaskXMCCount(before) > finalMaskXMCCount(after) {
			result.Warnings = append(result.Warnings, "removed incomplete FinalMask XMC entry")
		}
	}
	return result
}

func hasLegacyAllowInsecure(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if _, exists := typed["allowInsecure"]; exists {
			return true
		}
		for _, child := range typed {
			if hasLegacyAllowInsecure(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if hasLegacyAllowInsecure(child) {
				return true
			}
		}
	}
	return false
}

func finalMaskXMCCount(stream map[string]any) int {
	finalmask, _ := stream["finalmask"].(map[string]any)
	tcp, _ := finalmask["tcp"].([]any)
	count := 0
	for _, raw := range tcp {
		mask, _ := raw.(map[string]any)
		if mask["type"] == "xmc" {
			count++
		}
	}
	return count
}

func normalizeXHTTPCoreDefaults(xhttp map[string]any) bool {
	changed := false
	if v, ok := xhttp["scMinPostsIntervalMs"].(string); ok && strings.TrimSpace(v) == "30" {
		delete(xhttp, "scMinPostsIntervalMs")
		changed = true
	}
	if v, ok := xhttp["scMaxEachPostBytes"].(string); ok && strings.TrimSpace(v) == "1000000" {
		delete(xhttp, "scMaxEachPostBytes")
		changed = true
	}

	xmux, ok := xhttp["xmux"].(map[string]any)
	if !ok || xmux == nil {
		return changed
	}
	if isDefaultXHTTPXMUX(xmux) {
		delete(xhttp, "xmux")
		return true
	}
	return changed
}

func isDefaultXHTTPXMUX(xmux map[string]any) bool {
	defaults := map[string]string{
		"maxConcurrency":   "16-32",
		"maxConnections":   "0",
		"cMaxReuseTimes":   "0",
		"hMaxRequestTimes": "600-900",
		"hMaxReusableSecs": "1800-3000",
		"hKeepAlivePeriod": "0",
	}
	for key, want := range defaults {
		got := strings.TrimSpace(fmt.Sprint(xmux[key]))
		if got == "" {
			got = "0"
		}
		if got != want {
			return false
		}
	}
	return true
}

func normalizeFinalMaskStreamSettings(stream map[string]any) bool {
	finalmask, ok := stream["finalmask"].(map[string]any)
	if !ok || finalmask == nil {
		return false
	}

	rawTCP, _ := finalmask["tcp"].([]any)

	filteredTCP := make([]any, 0, len(rawTCP))
	changed := false
	for _, rawMask := range rawTCP {
		mask, _ := rawMask.(map[string]any)
		if mask == nil {
			changed = true
			continue
		}
		maskType, _ := mask["type"].(string)
		if maskType == "xmc" && !hasCompleteFinalMaskXMCSettings(mask["settings"]) {
			changed = true
			continue
		}
		filteredTCP = append(filteredTCP, rawMask)
	}

	if len(rawTCP) > 0 && len(filteredTCP) > 0 {
		finalmask["tcp"] = filteredTCP
	} else if len(rawTCP) > 0 {
		delete(finalmask, "tcp")
	}
	if rawUDP, ok := finalmask["udp"].([]any); ok {
		normalizedUDP := make([]any, 0, len(rawUDP))
		for _, rawMask := range rawUDP {
			mask, _ := rawMask.(map[string]any)
			if mask == nil {
				changed = true
				continue
			}
			normalized := normalizeLegacyFinalMaskUDPSettings(mask)
			if fmt.Sprint(normalized) != fmt.Sprint(mask) {
				changed = true
			}
			normalizedUDP = append(normalizedUDP, normalized)
		}
		finalmask["udp"] = normalizedUDP
	}
	if len(finalmask) == 0 {
		delete(stream, "finalmask")
	}
	return changed
}

func normalizeLegacyFinalMaskUDPSettings(mask map[string]any) map[string]any {
	maskType, _ := mask["type"].(string)
	settings, _ := mask["settings"].(map[string]any)
	switch maskType {
	case "mkcp-aes128gcm":
		return map[string]any{"type": "mkcp-legacy", "settings": map[string]any{"value": settings["password"]}}
	case "mkcp-original":
		return map[string]any{"type": "mkcp-legacy"}
	case "header-dns":
		return map[string]any{"type": "mkcp-legacy", "settings": map[string]any{"header": "dns", "value": settings["domain"]}}
	case "header-dtls", "header-srtp", "header-utp", "header-wechat", "header-wireguard":
		return map[string]any{"type": "mkcp-legacy", "settings": map[string]any{"header": strings.TrimPrefix(maskType, "header-")}}
	case "xicmp":
		if _, exists := settings["ips"]; !exists {
			if ip, ok := settings["ip"].(string); ok && strings.TrimSpace(ip) != "" {
				copySettings := map[string]any{}
				for key, value := range settings {
					copySettings[key] = value
				}
				copySettings["ips"] = []any{ip}
				delete(copySettings, "ip")
				delete(copySettings, "id")
				return map[string]any{"type": "xicmp", "settings": copySettings}
			}
		}
	}
	return mask
}

func hasCompleteFinalMaskXMCSettings(value any) bool {
	settings, _ := value.(map[string]any)
	if settings == nil {
		return false
	}
	if strings.TrimSpace(fmt.Sprint(settings["password"])) == "" || settings["password"] == nil {
		return false
	}
	profiles, ok := settings["profiles"].([]any)
	if !ok || len(profiles) == 0 {
		return false
	}
	for _, raw := range profiles {
		profile, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		for _, key := range []string{"username", "uuid", "texturesValue", "texturesSignature"} {
			field, exists := profile[key]
			if !exists || field == nil || strings.TrimSpace(fmt.Sprint(field)) == "" {
				return false
			}
		}
	}
	return true
}

// Setting stores key-value configuration settings for the 3x-ui panel.
type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type CustomGeoResource struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type          string `json:"type" gorm:"not null;uniqueIndex:idx_custom_geo_type_alias;column:geo_type"`
	Alias         string `json:"alias" gorm:"not null;uniqueIndex:idx_custom_geo_type_alias"`
	Url           string `json:"url" gorm:"not null"`
	LocalPath     string `json:"localPath" gorm:"column:local_path"`
	LastUpdatedAt int64  `json:"lastUpdatedAt" gorm:"default:0;column:last_updated_at"`
	LastModified  string `json:"lastModified" gorm:"column:last_modified"`
	CreatedAt     int64  `json:"createdAt" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt     int64  `json:"updatedAt" gorm:"autoUpdateTime;column:updated_at"`
}

type ClientReverse struct {
	Tag string `json:"tag"`
}

// Client represents a client configuration for Xray inbounds with traffic limits and settings.
type Client struct {
	ID         string         `json:"id,omitempty"`                 // Unique client identifier
	Security   string         `json:"security"`                     // Security method (e.g., "auto", "aes-128-gcm")
	Password   string         `json:"password,omitempty"`           // Client password
	Flow       string         `json:"flow,omitempty"`               // Flow control (XTLS)
	Reverse    *ClientReverse `json:"reverse,omitempty"`            // VLESS simple reverse proxy settings
	Auth       string         `json:"auth,omitempty"`               // Auth password (Hysteria)
	Email      string         `json:"email"`                        // Client email identifier
	LimitIP    int            `json:"limitIp"`                      // IP limit for this client
	TotalGB    int64          `json:"totalGB" form:"totalGB"`       // Total traffic limit in GB
	ExpiryTime int64          `json:"expiryTime" form:"expiryTime"` // Expiration timestamp
	Enable     bool           `json:"enable" form:"enable"`         // Whether the client is enabled
	TgID       int64          `json:"tgId" form:"tgId"`             // Telegram user ID for notifications
	SubID      string         `json:"subId" form:"subId"`           // Subscription identifier
	Comment    string         `json:"comment" form:"comment"`       // Client comment
	Reset      int            `json:"reset" form:"reset"`           // Reset period in days
	CreatedAt  int64          `json:"created_at,omitempty"`         // Creation timestamp
	UpdatedAt  int64          `json:"updated_at,omitempty"`         // Last update timestamp
}
