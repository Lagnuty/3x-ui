package link

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Outbound map[string]any

type ParseResult struct {
	Outbound Outbound
	Identity string
}

func ParseSubscriptionBody(body []byte) ([]Outbound, []string, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, nil, nil
	}
	if decoded, ok := tryBase64(text); ok {
		text = strings.TrimSpace(decoded)
	}
	lines := strings.FieldsFunc(strings.ReplaceAll(text, `\n`, "\n"), func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	outbounds := []Outbound{}
	identities := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		res, err := ParseLink(line)
		if err != nil || res == nil {
			continue
		}
		outbounds = append(outbounds, res.Outbound)
		identities = append(identities, res.Identity)
	}
	return outbounds, identities, nil
}

func ParseLink(raw string) (*ParseResult, error) {
	switch {
	case strings.HasPrefix(raw, "vmess://"):
		return parseVmess(raw)
	case strings.HasPrefix(raw, "vless://"):
		return parseVless(raw)
	case strings.HasPrefix(raw, "trojan://"):
		return parseTrojan(raw)
	case strings.HasPrefix(raw, "ss://"):
		return parseShadowsocks(raw)
	default:
		return nil, fmt.Errorf("unsupported link scheme")
	}
}

func parseVmess(raw string) (*ParseResult, error) {
	decoded, err := base64DecodeFlexible(strings.TrimPrefix(raw, "vmess://"))
	if err != nil {
		return nil, err
	}
	obj := map[string]any{}
	if err := json.Unmarshal([]byte(decoded), &obj); err != nil {
		return nil, err
	}
	network := getString(obj, "net", "tcp")
	security := "none"
	if getString(obj, "tls", "") == "tls" {
		security = "tls"
	}
	stream := buildStream(network, security)
	if network == "ws" {
		ws := stream["wsSettings"].(map[string]any)
		ws["host"] = getString(obj, "host", "")
		ws["path"] = getString(obj, "path", "/")
	}
	if network == "grpc" {
		grpc := stream["grpcSettings"].(map[string]any)
		grpc["serviceName"] = getString(obj, "path", "")
	}
	if security == "tls" {
		tls := stream["tlsSettings"].(map[string]any)
		tls["serverName"] = getString(obj, "sni", "")
		tls["fingerprint"] = getString(obj, "fp", "")
	}
	outbound := Outbound{
		"protocol": "vmess",
		"tag":      getString(obj, "ps", ""),
		"settings": map[string]any{
			"vnext": []any{map[string]any{
				"address": getString(obj, "add", ""),
				"port":    toInt(obj["port"]),
				"users": []any{map[string]any{
					"id":       getString(obj, "id", ""),
					"security": getString(obj, "scy", "auto"),
				}},
			}},
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: outbound, Identity: "vmess:" + decoded}, nil
}

func parseVless(raw string) (*ParseResult, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	params := u.Query()
	network := firstNonEmpty(params.Get("type"), "tcp")
	security := firstNonEmpty(params.Get("security"), "none")
	stream := buildStream(network, security)
	applyTransport(stream, params)
	applySecurity(stream, params)
	host := u.Hostname()
	port := defaultPort(u.Port(), 443)
	id := u.User.Username()
	outbound := Outbound{
		"protocol": "vless",
		"tag":      decodeFragment(u.Fragment),
		"settings": map[string]any{
			"address":    host,
			"port":       port,
			"id":         id,
			"flow":       params.Get("flow"),
			"encryption": firstNonEmpty(params.Get("encryption"), "none"),
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: outbound, Identity: "vless:" + id + "@" + host + ":" + strconv.Itoa(port)}, nil
}

func parseTrojan(raw string) (*ParseResult, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	params := u.Query()
	network := firstNonEmpty(params.Get("type"), "tcp")
	security := firstNonEmpty(params.Get("security"), "tls")
	stream := buildStream(network, security)
	applyTransport(stream, params)
	applySecurity(stream, params)
	host := u.Hostname()
	port := defaultPort(u.Port(), 443)
	password := u.User.Username()
	outbound := Outbound{
		"protocol": "trojan",
		"tag":      decodeFragment(u.Fragment),
		"settings": map[string]any{
			"servers": []any{map[string]any{"address": host, "port": port, "password": password}},
		},
		"streamSettings": stream,
	}
	return &ParseResult{Outbound: outbound, Identity: "trojan:" + password + "@" + host + ":" + strconv.Itoa(port)}, nil
}

func parseShadowsocks(raw string) (*ParseResult, error) {
	remark := ""
	if idx := strings.Index(raw, "#"); idx >= 0 {
		remark = decodeFragment(raw[idx+1:])
		raw = raw[:idx]
	}
	core := strings.TrimPrefix(raw, "ss://")
	if at := strings.Index(core, "@"); at >= 0 {
		userInfo := core[:at]
		if decoded, err := base64DecodeFlexible(userInfo); err == nil {
			userInfo = decoded
		}
		host, port, err := splitHostPort(core[at+1:])
		if err != nil {
			return nil, err
		}
		method, password := splitMethodPassword(userInfo)
		return shadowsocksResult(remark, method, password, host, port), nil
	}
	decoded, err := base64DecodeFlexible(core)
	if err != nil {
		return nil, err
	}
	at := strings.Index(decoded, "@")
	if at < 0 {
		return nil, fmt.Errorf("invalid ss link")
	}
	host, port, err := splitHostPort(decoded[at+1:])
	if err != nil {
		return nil, err
	}
	method, password := splitMethodPassword(decoded[:at])
	return shadowsocksResult(remark, method, password, host, port), nil
}

func shadowsocksResult(remark, method, password, host string, port int) *ParseResult {
	outbound := Outbound{
		"protocol": "shadowsocks",
		"tag":      remark,
		"settings": map[string]any{
			"servers": []any{map[string]any{"address": host, "port": port, "method": method, "password": password}},
		},
	}
	return &ParseResult{Outbound: outbound, Identity: "ss:" + method + ":" + password + "@" + host + ":" + strconv.Itoa(port)}
}

func buildStream(network, security string) map[string]any {
	stream := map[string]any{"network": network, "security": security}
	switch network {
	case "ws":
		stream["wsSettings"] = map[string]any{"path": "/", "host": "", "headers": map[string]any{}}
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": "", "authority": "", "multiMode": false}
	default:
		stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
	}
	switch security {
	case "tls":
		stream["tlsSettings"] = map[string]any{"serverName": "", "alpn": []any{}, "fingerprint": ""}
	case "reality":
		stream["realitySettings"] = map[string]any{"publicKey": "", "fingerprint": "chrome", "serverName": "", "shortId": "", "spiderX": ""}
	}
	return stream
}

func applyTransport(stream map[string]any, params url.Values) {
	network, _ := stream["network"].(string)
	switch network {
	case "ws":
		ws := stream["wsSettings"].(map[string]any)
		ws["host"] = params.Get("host")
		ws["path"] = firstNonEmpty(params.Get("path"), "/")
	case "grpc":
		grpc := stream["grpcSettings"].(map[string]any)
		grpc["serviceName"] = firstNonEmpty(params.Get("serviceName"), params.Get("path"))
		grpc["authority"] = params.Get("authority")
		grpc["multiMode"] = params.Get("mode") == "multi"
	}
}

func applySecurity(stream map[string]any, params url.Values) {
	security, _ := stream["security"].(string)
	switch security {
	case "tls":
		tls := stream["tlsSettings"].(map[string]any)
		tls["serverName"] = params.Get("sni")
		tls["fingerprint"] = params.Get("fp")
	case "reality":
		reality := stream["realitySettings"].(map[string]any)
		reality["serverName"] = params.Get("sni")
		reality["fingerprint"] = firstNonEmpty(params.Get("fp"), "chrome")
		reality["publicKey"] = params.Get("pbk")
		reality["shortId"] = params.Get("sid")
		reality["spiderX"] = params.Get("spx")
	}
}

func tryBase64(s string) (string, bool) {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)
	decoded, err := base64DecodeFlexible(clean)
	return decoded, err == nil
}

func base64DecodeFlexible(s string) (string, error) {
	for len(s)%4 != 0 {
		s += "="
	}
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return string(decoded), nil
	}
	return "", fmt.Errorf("base64 decode failed")
}

func getString(obj map[string]any, key string, fallback string) string {
	if value, ok := obj[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func splitComma(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func toInt(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	case int:
		return v
	}
	return 0
}

func defaultPort(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return fallback
	}
	return port
}

func decodeFragment(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func splitHostPort(value string) (string, int, error) {
	idx := strings.LastIndex(value, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf("missing host port")
	}
	port, err := strconv.Atoi(value[idx+1:])
	if err != nil {
		return "", 0, err
	}
	return value[:idx], port, nil
}

func splitMethodPassword(value string) (string, string) {
	idx := strings.Index(value, ":")
	if idx < 0 {
		return "aes-128-gcm", value
	}
	return value[:idx], value[idx+1:]
}
