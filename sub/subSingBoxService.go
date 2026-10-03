package sub

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-json"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// GetSingBox emits an outbound-only sing-box subscription. Version selects
// legacy or current names for XHTTP extension fields.
func (s *SubJsonService) GetSingBox(subID, host, version string) (string, string, error) {
	inbounds, err := s.SubService.getInboundsBySubId(subID)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}
	outbounds := make([]map[string]any, 0, len(inbounds))
	var traffic xray.ClientTraffic
	firstTraffic := true
	for _, inbound := range inbounds {
		clients, clientErr := s.inboundService.GetClients(inbound)
		if clientErr != nil {
			logger.Error("SubJsonService - GetSingBox: unable to get clients from inbound")
			continue
		}
		for _, client := range clients {
			if client.SubID != subID {
				continue
			}
			item := s.SubService.getClientTraffics(inbound.ClientStats, client.Email)
			mergeSubscriptionTraffic(&traffic, item, firstTraffic)
			firstTraffic = false
			outbounds = append(outbounds, s.buildSingBoxOutbounds(inbound, client, host, version)...)
		}
	}
	if len(outbounds) == 0 {
		return "", "", nil
	}
	payload := map[string]any{
		"$schema":   "https://sing-box.sagernet.org/schema.json",
		"outbounds": outbounds,
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", "", err
	}
	header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(encoded), header, nil
}

func mergeSubscriptionTraffic(total *xray.ClientTraffic, item xray.ClientTraffic, first bool) {
	if first {
		*total = item
		return
	}
	total.Up += item.Up
	total.Down += item.Down
	if total.Total == 0 || item.Total == 0 {
		total.Total = 0
	} else {
		total.Total += item.Total
	}
	if total.ExpiryTime != item.ExpiryTime {
		total.ExpiryTime = 0
	}
}

func (s *SubJsonService) buildSingBoxOutbounds(inbound *model.Inbound, client model.Client, host, version string) []map[string]any {
	stream := unmarshalStreamSettings(inbound.StreamSettings)
	external, _ := stream["externalProxy"].([]any)
	if len(external) == 0 {
		external = []any{map[string]any{"dest": host, "port": float64(inbound.Port), "remark": ""}}
	}
	result := make([]map[string]any, 0, len(external))
	for _, raw := range external {
		ep, _ := raw.(map[string]any)
		server := strings.TrimSpace(fmt.Sprint(ep["dest"]))
		if server == "" {
			server = host
		}
		port := inbound.Port
		if value, ok := ep["port"].(float64); ok && value > 0 {
			port = int(value)
		}
		remark := strings.TrimSpace(fmt.Sprint(ep["remark"]))
		outbound := map[string]any{
			"type":        string(inbound.Protocol),
			"tag":         s.SubService.genRemark(inbound, client.Email, remark),
			"server":      server,
			"server_port": port,
		}
		switch inbound.Protocol {
		case model.VLESS:
			outbound["uuid"] = client.ID
			if client.Flow != "" {
				outbound["flow"] = client.Flow
			}
			var settings map[string]any
			_ = json.Unmarshal([]byte(inbound.Settings), &settings)
			if encoding, ok := settings["encryption"].(string); ok && encoding != "" {
				outbound["packet_encoding"] = encoding
			}
		case model.VMESS:
			outbound["uuid"] = client.ID
			outbound["security"] = client.Security
		case model.Trojan:
			outbound["password"] = client.Password
		case model.Shadowsocks:
			var settings map[string]any
			_ = json.Unmarshal([]byte(inbound.Settings), &settings)
			outbound["method"] = settings["method"]
			outbound["password"] = client.Password
		default:
			continue
		}
		applySingBoxStream(outbound, stream, version)
		result = append(result, outbound)
	}
	return result
}

func applySingBoxStream(outbound, stream map[string]any, version string) {
	network, _ := stream["network"].(string)
	if transport := singBoxTransport(network, stream, version); transport != nil {
		outbound["transport"] = transport
	}
	security, _ := stream["security"].(string)
	if security == "tls" || security == "reality" {
		tls := map[string]any{"enabled": true}
		settings, _ := stream[security+"Settings"].(map[string]any)
		if names, ok := settings["serverNames"].([]any); ok && len(names) > 0 {
			tls["server_name"] = fmt.Sprint(names[0])
		} else if name, ok := settings["serverName"].(string); ok && name != "" {
			tls["server_name"] = name
		}
		clientSettings, _ := settings["settings"].(map[string]any)
		fingerprint, _ := clientSettings["fingerprint"].(string)
		if security == "reality" && fingerprint == "" {
			fingerprint = "chrome"
		}
		if fingerprint != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
		}
		if security == "reality" {
			reality := map[string]any{"enabled": true, "public_key": clientSettings["publicKey"]}
			if ids, ok := settings["shortIds"].([]any); ok && len(ids) > 0 {
				reality["short_id"] = fmt.Sprint(ids[0])
			}
			tls["reality"] = reality
		}
		outbound["tls"] = tls
	}
	if finalmask, ok := stream["finalmask"].(map[string]any); ok {
		if normalized := normalizeFinalMask(finalmask); normalized != nil {
			outbound["final_mask"] = normalized
		}
	}
}

func singBoxTransport(network string, stream map[string]any, version string) map[string]any {
	var source map[string]any
	transport := map[string]any{}
	switch network {
	case "ws":
		transport["type"] = "ws"
		source, _ = stream["wsSettings"].(map[string]any)
	case "grpc":
		transport["type"] = "grpc"
		source, _ = stream["grpcSettings"].(map[string]any)
		if value, ok := source["serviceName"].(string); ok && value != "" {
			transport["service_name"] = value
		}
		return transport
	case "httpupgrade":
		transport["type"] = "httpupgrade"
		source, _ = stream["httpupgradeSettings"].(map[string]any)
	case "xhttp":
		transport["type"] = "xhttp"
		source, _ = stream["xhttpSettings"].(map[string]any)
		copySingBoxXHTTP(transport, source, version)
		return transport
	default:
		return nil
	}
	if path, ok := source["path"].(string); ok && path != "" {
		transport["path"] = path
	}
	if headers, ok := nonEmptyShareObject(source["headers"]); ok {
		transport["headers"] = headers
	}
	return transport
}

func copySingBoxXHTTP(dst, src map[string]any, version string) {
	fields := map[string]string{
		"path": "path", "host": "host", "mode": "mode", "xPaddingBytes": "x_padding_bytes",
		"xPaddingKey": "x_padding_key", "xPaddingHeader": "x_padding_header", "xPaddingPlacement": "x_padding_placement",
		"xPaddingMethod": "x_padding_method", "uplinkDataPlacement": "uplink_data_placement", "uplinkDataKey": "uplink_data_key",
		"uplinkHTTPMethod": "uplink_http_method",
	}
	for source, target := range fields {
		if value, ok := src[source].(string); ok && value != "" {
			dst[target] = value
		}
	}
	modern := compareRelease(version, 26, 6, 22) >= 0 || strings.TrimSpace(version) == ""
	for _, field := range []string{"Placement", "Key", "Table", "Length"} {
		source := "sessionID" + field
		targetPrefix := "session_id_"
		if !modern {
			source = "session" + field
			targetPrefix = "session_"
		}
		if value, ok := nonZeroShareValue(src[source]); ok {
			dst[targetPrefix+strings.ToLower(field)] = value
		}
	}
	if headers, ok := nonEmptyShareObject(src["headers"]); ok {
		dst["headers"] = headers
	}
}

func compareRelease(version string, major, minor, patch int) int {
	version = strings.TrimLeft(strings.TrimSpace(version), "vV")
	parts := strings.SplitN(version, ".", 4)
	if len(parts) < 3 {
		return 0
	}
	want := []int{major, minor, patch}
	for index := range want {
		value, err := strconv.Atoi(strings.TrimRightFunc(parts[index], func(r rune) bool { return r < '0' || r > '9' }))
		if err != nil {
			return 0
		}
		if value < want[index] {
			return -1
		}
		if value > want[index] {
			return 1
		}
	}
	return 0
}
