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
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(normalizeInboundSettings(i.Protocol, i.Settings)),
		StreamSettings: json_util.RawMessage(normalizeStreamSettings(i.StreamSettings)),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

func normalizeInboundSettings(protocol Protocol, settings string) string {
	if protocol != WireGuard {
		return settings
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(settings), &cfg); err != nil {
		return settings
	}

	delete(cfg, "workers")
	delete(cfg, "num_workers")

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

	xhttp, ok := stream["xhttpSettings"].(map[string]any)
	if !ok || xhttp == nil {
		if !changed {
			return streamSettings
		}
		normalized, err := json.Marshal(stream)
		if err != nil {
			return streamSettings
		}
		return string(normalized)
	}

	if _, ok := xhttp["sessionPlacement"]; ok {
		changed = true
	}
	if _, ok := xhttp["sessionKey"]; ok {
		changed = true
	}
	if normalizeXHTTPCoreDefaults(xhttp) {
		changed = true
	}

	if !changed {
		return streamSettings
	}

	if _, ok := xhttp["sessionIDPlacement"]; !ok {
		if v, ok := xhttp["sessionPlacement"]; ok {
			xhttp["sessionIDPlacement"] = v
		}
	}
	if _, ok := xhttp["sessionIDKey"]; !ok {
		if v, ok := xhttp["sessionKey"]; ok {
			xhttp["sessionIDKey"] = v
		}
	}
	delete(xhttp, "sessionPlacement")
	delete(xhttp, "sessionKey")

	normalized, err := json.Marshal(stream)
	if err != nil {
		return streamSettings
	}
	return string(normalized)
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

	rawTCP, hasTCP := finalmask["tcp"].([]any)
	if !hasTCP {
		return false
	}

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

	if !changed {
		return false
	}
	if len(filteredTCP) > 0 {
		finalmask["tcp"] = filteredTCP
	} else {
		delete(finalmask, "tcp")
	}
	if len(finalmask) == 0 {
		delete(stream, "finalmask")
	}
	return true
}

func hasCompleteFinalMaskXMCSettings(value any) bool {
	settings, _ := value.(map[string]any)
	if settings == nil {
		return false
	}
	for _, key := range []string{"profile", "texture", "signature"} {
		if strings.TrimSpace(fmt.Sprint(settings[key])) == "" {
			return false
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
