package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
)

type HeartbeatPatch struct {
	Status        string
	LastHeartbeat int64
	LatencyMs     int
	XrayVersion   string
	PanelVersion  string
	Guid          string
	CpuPct        float64
	MemPct        float64
	UptimeSecs    uint64
	LastError     string
	XrayState     string
	XrayError     string
}

type ProbeResultUI struct {
	Status       string  `json:"status"`
	LatencyMs    int     `json:"latencyMs"`
	XrayVersion  string  `json:"xrayVersion"`
	PanelVersion string  `json:"panelVersion"`
	CpuPct       float64 `json:"cpuPct"`
	MemPct       float64 `json:"memPct"`
	UptimeSecs   uint64  `json:"uptimeSecs"`
	Error        string  `json:"error"`
	XrayState    string  `json:"xrayState"`
	XrayError    string  `json:"xrayError"`
}

type NodeService struct{}

func (s *NodeService) GetAll() ([]*model.Node, error) {
	var nodes []*model.Node
	db := database.GetDB()
	if err := db.Model(model.Node{}).Order("id asc").Find(&nodes).Error; err != nil {
		return nil, err
	}

	type inboundCount struct {
		NodeID int `gorm:"column:node_id"`
		Count  int `gorm:"column:count"`
	}
	var inboundCounts []inboundCount
	if err := db.Table("inbounds").
		Select("node_id, COUNT(*) AS count").
		Where("node_id IS NOT NULL").
		Group("node_id").
		Scan(&inboundCounts).Error; err == nil {
		byID := map[int]int{}
		for _, row := range inboundCounts {
			byID[row.NodeID] = row.Count
		}
		for _, n := range nodes {
			n.InboundCount = byID[n.Id]
		}
	}

	type clientCount struct {
		NodeID int `gorm:"column:node_id"`
		Count  int `gorm:"column:count"`
	}
	var clientCounts []clientCount
	if err := db.Raw(`
		SELECT inbounds.node_id AS node_id, COUNT(DISTINCT client_inbounds.client_id) AS count
		FROM inbounds
		JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id
		WHERE inbounds.node_id IS NOT NULL
		GROUP BY inbounds.node_id
	`).Scan(&clientCounts).Error; err == nil {
		byID := map[int]int{}
		for _, row := range clientCounts {
			byID[row.NodeID] = row.Count
		}
		for _, n := range nodes {
			n.ClientCount = byID[n.Id]
		}
	}

	return nodes, nil
}

func (s *NodeService) GetById(id int) (*model.Node, error) {
	n := &model.Node{}
	if err := database.GetDB().Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

func (s *NodeService) Create(n *model.Node) error {
	if err := s.normalize(n); err != nil {
		return err
	}
	return database.GetDB().Create(n).Error
}

func (s *NodeService) Update(id int, in *model.Node) error {
	if err := s.normalize(in); err != nil {
		return err
	}
	updates := map[string]any{
		"name":                  in.Name,
		"remark":                in.Remark,
		"scheme":                in.Scheme,
		"address":               in.Address,
		"port":                  in.Port,
		"base_path":             in.BasePath,
		"api_token":             in.ApiToken,
		"enable":                in.Enable,
		"allow_private_address": in.AllowPrivateAddress,
		"tls_verify_mode":       in.TlsVerifyMode,
		"pinned_cert_sha256":    in.PinnedCertSha256,
		"inbound_sync_mode":     in.InboundSyncMode,
	}
	if b, err := json.Marshal(in.InboundTags); err == nil {
		updates["inbound_tags"] = string(b)
	}
	return database.GetDB().Model(model.Node{}).Where("id = ?", id).Updates(updates).Error
}

func (s *NodeService) Delete(id int) error {
	db := database.GetDB()
	if err := db.Model(model.Inbound{}).Where("node_id = ?", id).Update("node_id", nil).Error; err != nil {
		return err
	}
	if err := db.Where("node_id = ?", id).Delete(&model.NodeClientTraffic{}).Error; err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(model.Node{}).Error
}

func (s *NodeService) SetEnable(id int, enable bool) error {
	return database.GetDB().Model(model.Node{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *NodeService) UpdateHeartbeat(id int, p HeartbeatPatch) error {
	updates := map[string]any{
		"status":         p.Status,
		"last_heartbeat": p.LastHeartbeat,
		"latency_ms":     p.LatencyMs,
		"xray_version":   p.XrayVersion,
		"panel_version":  p.PanelVersion,
		"cpu_pct":        p.CpuPct,
		"mem_pct":        p.MemPct,
		"uptime_secs":    p.UptimeSecs,
		"last_error":     p.LastError,
		"xray_state":     p.XrayState,
		"xray_error":     p.XrayError,
	}
	if p.Guid != "" {
		updates["guid"] = p.Guid
	}
	return database.GetDB().Model(model.Node{}).Where("id = ?", id).Updates(updates).Error
}

func (s *NodeService) Probe(ctx context.Context, n *model.Node) (HeartbeatPatch, error) {
	patch := HeartbeatPatch{LastHeartbeat: time.Now().Unix()}
	if err := s.normalize(n); err != nil {
		patch.LastError = err.Error()
		return patch, err
	}

	nodeURL := &url.URL{
		Scheme: n.Scheme,
		Host:   net.JoinHostPort(n.Address, strconv.Itoa(n.Port)),
		Path:   strings.TrimSuffix(n.BasePath, "/") + "/panel/api/server/status",
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nodeURL.String(), nil)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	if n.ApiToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.ApiToken)
	}
	req.Header.Set("Accept", "application/json")

	client, err := nodeHTTPClient(n)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	defer resp.Body.Close()
	patch.LatencyMs = int(time.Since(start) / time.Millisecond)

	if resp.StatusCode != http.StatusOK {
		patch.LastError = fmt.Sprintf("HTTP %d from remote panel", resp.StatusCode)
		return patch, errors.New(patch.LastError)
	}
	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     *struct {
			CpuPct float64 `json:"cpu"`
			Mem    struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"mem"`
			Xray struct {
				Version  string `json:"version"`
				State    string `json:"state"`
				ErrorMsg string `json:"errorMsg"`
			} `json:"xray"`
			PanelVersion string `json:"panelVersion"`
			PanelGuid    string `json:"panelGuid"`
			Uptime       uint64 `json:"uptime"`
		} `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		patch.LastError = "decode response: " + err.Error()
		return patch, err
	}
	if !envelope.Success || envelope.Obj == nil {
		patch.LastError = "remote returned success=false: " + envelope.Msg
		return patch, errors.New(patch.LastError)
	}
	obj := envelope.Obj
	patch.CpuPct = obj.CpuPct
	if obj.Mem.Total > 0 {
		patch.MemPct = float64(obj.Mem.Current) * 100 / float64(obj.Mem.Total)
	}
	patch.XrayVersion = obj.Xray.Version
	patch.XrayState = obj.Xray.State
	patch.XrayError = obj.Xray.ErrorMsg
	patch.PanelVersion = obj.PanelVersion
	patch.Guid = obj.PanelGuid
	patch.UptimeSecs = obj.Uptime
	return patch, nil
}

func (p HeartbeatPatch) ToUI(ok bool) ProbeResultUI {
	status := "offline"
	if ok {
		status = "online"
	}
	return ProbeResultUI{
		Status:       status,
		LatencyMs:    p.LatencyMs,
		XrayVersion:  p.XrayVersion,
		PanelVersion: p.PanelVersion,
		CpuPct:       p.CpuPct,
		MemPct:       p.MemPct,
		UptimeSecs:   p.UptimeSecs,
		Error:        FriendlyProbeError(p.LastError),
		XrayState:    p.XrayState,
		XrayError:    p.XrayError,
	}
}

func FriendlyProbeError(msg string) string {
	if strings.Contains(msg, "server gave HTTP response to HTTPS client") {
		return "the server speaks HTTP, not HTTPS; set the node scheme to http"
	}
	return msg
}

func (s *NodeService) normalize(n *model.Node) error {
	n.Name = strings.TrimSpace(n.Name)
	n.Address = strings.TrimSpace(n.Address)
	n.ApiToken = strings.TrimSpace(n.ApiToken)
	if n.Name == "" {
		return common.NewError("node name is required")
	}
	if n.Address == "" {
		return common.NewError("node address is required")
	}
	host := strings.Trim(n.Address, "[]")
	if strings.Contains(host, "/") || strings.Contains(host, "\\") {
		return common.NewError("node address must be a host or IP, not a URL")
	}
	n.Address = host
	if n.Port <= 0 || n.Port > 65535 {
		return common.NewError("node port must be 1-65535")
	}
	if n.Scheme != "http" && n.Scheme != "https" {
		n.Scheme = "https"
	}
	if n.TlsVerifyMode != "skip" && n.TlsVerifyMode != "pin" {
		n.TlsVerifyMode = "verify"
	}
	if n.TlsVerifyMode == "pin" {
		if _, err := decodeCertPin(n.PinnedCertSha256); err != nil {
			return err
		}
	}
	if n.InboundSyncMode != "selected" {
		n.InboundSyncMode = "all"
		n.InboundTags = nil
	} else {
		n.InboundTags = normalizeTagList(n.InboundTags)
	}
	n.BasePath = normalizeBasePath(n.BasePath)
	return nil
}

func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func normalizeTagList(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func nodeHTTPClient(n *model.Node) (*http.Client, error) {
	tlsCfg := &tls.Config{}
	if n.Scheme == "https" && n.TlsVerifyMode != "verify" {
		tlsCfg.InsecureSkipVerify = true
		if n.TlsVerifyMode == "pin" {
			want, err := decodeCertPin(n.PinnedCertSha256)
			if err != nil {
				return nil, err
			}
			tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return common.NewError("node presented no certificate")
				}
				sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
				if subtle.ConstantTimeCompare(sum[:], want) != 1 {
					return common.NewError("node certificate does not match pinned SHA-256")
				}
				return nil
			}
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ipAddr := range ips {
				if !n.AllowPrivateAddress && isPrivateIP(ipAddr.IP) {
					return nil, common.NewError("node address resolves to a private/local IP; enable Allow private for trusted internal nodes")
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipAddr.IP.String(), port))
				if err == nil {
					return conn, nil
				}
			}
			return nil, common.NewError("unable to connect to node")
		},
		TLSClientConfig: tlsCfg,
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second}, nil
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func decodeCertPin(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, common.NewError("certificate pin is empty")
	}
	if b, err := hex.DecodeString(strings.ReplaceAll(s, ":", "")); err == nil && len(b) == sha256.Size {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == sha256.Size {
			return b, nil
		}
	}
	return nil, common.NewError("certificate pin must be a SHA-256 hash (base64 or hex)")
}
