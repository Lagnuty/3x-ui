package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

const (
	openFluxBinaryPath = "/usr/local/bin/openflux"
	openFluxRepoURL    = "https://github.com/p1neappleXpress/OpenFlux.git"
	openFluxUnitDir    = "/etc/systemd/system"
	openFluxUnitPrefix = "openflux-"
	openFluxUnitSuffix = ".service"
)

var (
	openFluxNamePattern   = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\s._-]{0,63}$`)
	openFluxServerPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{0,64}$`)
	openFluxSafeArgChars  = regexp.MustCompile(`^[^\x00-\x1f\x7f]+$`)
	openFluxRefPattern    = regexp.MustCompile(`^[A-Za-z0-9._/\-]{1,128}$`)

	ErrOpenFluxNotFound          = errors.New("openflux_not_found")
	ErrOpenFluxNameRequired      = errors.New("openflux_name_required")
	ErrOpenFluxNameInvalid       = errors.New("openflux_name_invalid")
	ErrOpenFluxServerIDInvalid   = errors.New("openflux_server_id_invalid")
	ErrOpenFluxTransportInvalid  = errors.New("openflux_transport_invalid")
	ErrOpenFluxURLRequired       = errors.New("openflux_url_required")
	ErrOpenFluxURLInvalid        = errors.New("openflux_url_invalid")
	ErrOpenFluxModeInvalid       = errors.New("openflux_mode_invalid")
	ErrOpenFluxCodecInvalid      = errors.New("openflux_codec_invalid")
	ErrOpenFluxDebugInvalid      = errors.New("openflux_debug_invalid")
	ErrOpenFluxKeyFileInvalid    = errors.New("openflux_key_file_invalid")
	ErrOpenFluxUnsupportedOS     = errors.New("openflux_unsupported_os")
	ErrOpenFluxSystemctlFailed   = errors.New("openflux_systemctl_failed")
	ErrOpenFluxBinaryUnavailable = errors.New("openflux_binary_unavailable")
	ErrOpenFluxInstallFailed     = errors.New("openflux_install_failed")
	ErrOpenFluxRefInvalid        = errors.New("openflux_ref_invalid")
)

type OpenFluxService struct{}

type OpenFluxStatus struct {
	UnitName      string `json:"unitName"`
	Active        bool   `json:"active"`
	State         string `json:"state"`
	LastRestartAt string `json:"lastRestartAt"`
	Logs          string `json:"logs"`
	Version       string `json:"version"`
	Error         string `json:"error,omitempty"`
}

type OpenFluxMobileConnection struct {
	Name         string `json:"name"`
	Server       string `json:"server"`
	Module       string `json:"module"`
	Protocol     string `json:"protocol"`
	Transport    string `json:"transport"`
	Carrier      string `json:"carrier"`
	ModuleConfig string `json:"module_config"`
	Enabled      bool   `json:"enabled"`
}

type OpenFluxInstallInfo struct {
	Path             string `json:"path"`
	Repo             string `json:"repo"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	LatestCommit     string `json:"latestCommit"`
	LatestRef        string `json:"latestRef"`
}

type OpenFluxInstallResult struct {
	Path          string `json:"path"`
	Repo          string `json:"repo"`
	Ref           string `json:"ref"`
	Commit        string `json:"commit"`
	Version       string `json:"version"`
	BackupPath    string `json:"backupPath,omitempty"`
	RestartedUnit int    `json:"restartedUnit"`
}

func (s *OpenFluxService) GetAll() ([]model.OpenFluxNode, error) {
	var list []model.OpenFluxNode
	err := database.GetDB().Order("id asc").Find(&list).Error
	return list, err
}

func (s *OpenFluxService) GetByID(id int) (*model.OpenFluxNode, error) {
	var node model.OpenFluxNode
	if err := database.GetDB().First(&node, id).Error; err != nil {
		if database.IsNotFound(err) {
			return nil, ErrOpenFluxNotFound
		}
		return nil, err
	}
	return &node, nil
}

func (s *OpenFluxService) Create(node *model.OpenFluxNode) error {
	if err := normalizeOpenFluxNode(node); err != nil {
		return err
	}
	if err := database.GetDB().Create(node).Error; err != nil {
		return err
	}
	return s.ApplyUnit(node.Id)
}

func (s *OpenFluxService) Update(id int, node *model.OpenFluxNode) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	node.Id = id
	if err := normalizeOpenFluxNode(node); err != nil {
		return err
	}
	updates := map[string]any{
		"name":                node.Name,
		"server_id":           node.ServerID,
		"transport":           node.Transport,
		"url":                 node.URL,
		"mode":                node.Mode,
		"codec":               node.Codec,
		"debug":               node.Debug,
		"enabled":             node.Enabled,
		"encryption_key_file": node.EncryptionKeyFile,
	}
	if err := database.GetDB().Model(&model.OpenFluxNode{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	return s.ApplyUnit(id)
}

func (s *OpenFluxService) Delete(id int) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	_ = s.Control(id, "stop")
	_ = removeOpenFluxUnit(id)
	return database.GetDB().Delete(&model.OpenFluxNode{}, id).Error
}

func normalizeOpenFluxNode(node *model.OpenFluxNode) error {
	node.Name = strings.TrimSpace(node.Name)
	node.ServerID = strings.TrimSpace(node.ServerID)
	node.Transport = strings.ToLower(strings.TrimSpace(node.Transport))
	node.URL = strings.TrimSpace(node.URL)
	node.Mode = strings.ToLower(strings.TrimSpace(node.Mode))
	node.Codec = strings.ToLower(strings.TrimSpace(node.Codec))
	node.EncryptionKeyFile = strings.TrimSpace(node.EncryptionKeyFile)
	if node.Name == "" {
		return ErrOpenFluxNameRequired
	}
	if !openFluxNamePattern.MatchString(node.Name) {
		return ErrOpenFluxNameInvalid
	}
	if !openFluxServerPattern.MatchString(node.ServerID) {
		return ErrOpenFluxServerIDInvalid
	}
	switch node.Transport {
	case "yandex", "vyandex", "boards":
	default:
		return ErrOpenFluxTransportInvalid
	}
	if node.URL == "" {
		return ErrOpenFluxURLRequired
	}
	parsed, err := url.Parse(node.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ErrOpenFluxURLInvalid
	}
	switch node.Transport {
	case "boards":
		if !strings.Contains(strings.ToLower(parsed.Host), "yandex.") || !strings.HasPrefix(parsed.Path, "/board") {
			return ErrOpenFluxURLInvalid
		}
	default:
		if !strings.Contains(strings.ToLower(parsed.Host), "yandex.") {
			return ErrOpenFluxURLInvalid
		}
	}
	if node.Mode == "" {
		node.Mode = "l4"
	}
	if node.Mode != "l4" && node.Mode != "l3" {
		return ErrOpenFluxModeInvalid
	}
	if node.Codec == "" {
		node.Codec = "batched"
	}
	if node.Codec != "batched" {
		return ErrOpenFluxCodecInvalid
	}
	if node.Debug < 0 || node.Debug > 3 {
		return ErrOpenFluxDebugInvalid
	}
	if node.EncryptionKeyFile != "" && (!filepath.IsAbs(node.EncryptionKeyFile) || strings.ContainsAny(node.EncryptionKeyFile, "\x00\r\n")) {
		return ErrOpenFluxKeyFileInvalid
	}
	if !openFluxSafeArgChars.MatchString(node.URL) || !openFluxSafeArgChars.MatchString(node.Name) {
		return ErrOpenFluxURLInvalid
	}
	return nil
}

func openFluxUnitName(id int) string {
	return fmt.Sprintf("%s%d%s", openFluxUnitPrefix, id, openFluxUnitSuffix)
}

func openFluxUnitPath(id int) string {
	return filepath.Join(openFluxUnitDir, openFluxUnitName(id))
}

func openFluxShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func openFluxUnitContent(node *model.OpenFluxNode) string {
	args := []string{
		"--role=exit",
		"--mode=" + node.Mode,
		"--transport=" + node.Transport,
		"--url=" + node.URL,
		"--codec=" + node.Codec,
		"--debug=" + strconv.Itoa(node.Debug),
	}
	if node.EncryptionKeyFile != "" {
		args = append(args, "--encryption-key-file="+node.EncryptionKeyFile)
	}
	quoted := make([]string, 0, len(args)+1)
	quoted = append(quoted, openFluxBinaryPath)
	for _, arg := range args {
		quoted = append(quoted, openFluxShellQuote(arg))
	}
	return fmt.Sprintf(`[Unit]
Description=L-VPN OpenFlux %d (%s)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=always
RestartSec=3
User=root

[Install]
WantedBy=multi-user.target
`, node.Id, node.Name, strings.Join(quoted, " "))
}

func requireOpenFluxLinux() error {
	if runtime.GOOS != "linux" {
		return ErrOpenFluxUnsupportedOS
	}
	return nil
}

func runOpenFluxCommand(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	return string(out), err
}

func runOpenFluxCommandIn(timeout time.Duration, dir string, env []string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	return string(out), err
}

func reloadSystemd() {
	_, _ = runOpenFluxCommand(20*time.Second, "systemctl", "daemon-reload")
}

func (s *OpenFluxService) ApplyUnit(id int) error {
	if err := requireOpenFluxLinux(); err != nil {
		return err
	}
	node, err := s.GetByID(id)
	if err != nil {
		return err
	}
	if err := os.WriteFile(openFluxUnitPath(id), []byte(openFluxUnitContent(node)), 0o644); err != nil {
		return err
	}
	reloadSystemd()
	if node.Enabled {
		if out, err := runOpenFluxCommand(30*time.Second, "systemctl", "enable", "--now", openFluxUnitName(id)); err != nil {
			return fmt.Errorf("%w: %s", ErrOpenFluxSystemctlFailed, strings.TrimSpace(out))
		}
		return nil
	}
	if out, err := runOpenFluxCommand(30*time.Second, "systemctl", "disable", "--now", openFluxUnitName(id)); err != nil {
		return fmt.Errorf("%w: %s", ErrOpenFluxSystemctlFailed, strings.TrimSpace(out))
	}
	return nil
}

func removeOpenFluxUnit(id int) error {
	if err := requireOpenFluxLinux(); err != nil {
		return err
	}
	path := openFluxUnitPath(id)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	reloadSystemd()
	return nil
}

func (s *OpenFluxService) Control(id int, action string) error {
	if err := requireOpenFluxLinux(); err != nil {
		return err
	}
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	if out, err := runOpenFluxCommand(30*time.Second, "systemctl", action, openFluxUnitName(id)); err != nil {
		return fmt.Errorf("%w: %s", ErrOpenFluxSystemctlFailed, strings.TrimSpace(out))
	}
	return nil
}

func (s *OpenFluxService) Status(id int, logLines int) (*OpenFluxStatus, error) {
	if _, err := s.GetByID(id); err != nil {
		return nil, err
	}
	status := &OpenFluxStatus{UnitName: openFluxUnitName(id)}
	version, _ := s.BinaryVersion()
	status.Version = version
	if err := requireOpenFluxLinux(); err != nil {
		status.State = "unsupported"
		status.Error = err.Error()
		return status, nil
	}
	out, err := runOpenFluxCommand(10*time.Second, "systemctl", "is-active", status.UnitName)
	status.State = strings.TrimSpace(out)
	status.Active = err == nil && status.State == "active"
	restart, _ := runOpenFluxCommand(10*time.Second, "systemctl", "show", status.UnitName, "-p", "ActiveEnterTimestamp", "--value")
	status.LastRestartAt = strings.TrimSpace(restart)
	if logLines <= 0 || logLines > 500 {
		logLines = 80
	}
	logs, _ := runOpenFluxCommand(15*time.Second, "journalctl", "-u", status.UnitName, "-n", strconv.Itoa(logLines), "--no-pager")
	status.Logs = maskOpenFluxSecrets(logs)
	return status, nil
}

func (s *OpenFluxService) BinaryVersion() (string, error) {
	if _, err := os.Stat(openFluxBinaryPath); err != nil {
		return "", ErrOpenFluxBinaryUnavailable
	}
	out, err := runOpenFluxCommand(10*time.Second, openFluxBinaryPath, "--version")
	if err != nil {
		out, err = runOpenFluxCommand(10*time.Second, openFluxBinaryPath, "version")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(maskOpenFluxSecrets(out)), nil
}

func (s *OpenFluxService) InstallInfo() (*OpenFluxInstallInfo, error) {
	info := &OpenFluxInstallInfo{
		Path:      openFluxBinaryPath,
		Repo:      openFluxRepoURL,
		LatestRef: "HEAD",
	}
	if version, err := s.BinaryVersion(); err == nil {
		info.Installed = true
		info.InstalledVersion = version
	}
	if out, err := runOpenFluxCommand(20*time.Second, "git", "ls-remote", openFluxRepoURL, "HEAD"); err == nil {
		fields := strings.Fields(out)
		if len(fields) > 0 {
			info.LatestCommit = fields[0]
		}
	}
	return info, nil
}

func normalizeOpenFluxRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "HEAD", nil
	}
	if !openFluxRefPattern.MatchString(ref) || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.HasPrefix(ref, "-") {
		return "", ErrOpenFluxRefInvalid
	}
	return ref, nil
}

func copyOpenFluxFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (s *OpenFluxService) InstallOrUpdate(ref string) (*OpenFluxInstallResult, error) {
	if err := requireOpenFluxLinux(); err != nil {
		return nil, err
	}
	ref, err := normalizeOpenFluxRef(ref)
	if err != nil {
		return nil, err
	}
	if _, err = exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("%w: git is not installed", ErrOpenFluxInstallFailed)
	}
	if _, err = exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("%w: go is not installed", ErrOpenFluxInstallFailed)
	}

	tmp, err := os.MkdirTemp("", "openflux-build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	srcDir := filepath.Join(tmp, "src")
	if out, err := runOpenFluxCommand(2*time.Minute, "git", "clone", "--depth=1", openFluxRepoURL, srcDir); err != nil {
		return nil, fmt.Errorf("%w: git clone failed: %s", ErrOpenFluxInstallFailed, strings.TrimSpace(out))
	}
	if ref != "HEAD" {
		if out, err := runOpenFluxCommandIn(2*time.Minute, srcDir, nil, "git", "fetch", "--depth=1", "origin", ref); err != nil {
			return nil, fmt.Errorf("%w: git fetch %s failed: %s", ErrOpenFluxInstallFailed, ref, strings.TrimSpace(out))
		}
		if out, err := runOpenFluxCommandIn(30*time.Second, srcDir, nil, "git", "checkout", "--detach", "FETCH_HEAD"); err != nil {
			return nil, fmt.Errorf("%w: git checkout %s failed: %s", ErrOpenFluxInstallFailed, ref, strings.TrimSpace(out))
		}
	}
	commitOut, _ := runOpenFluxCommandIn(10*time.Second, srcDir, nil, "git", "rev-parse", "--short=12", "HEAD")
	commit := strings.TrimSpace(commitOut)

	candidate := filepath.Join(tmp, "openflux-linux-amd64")
	env := []string{"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"}
	if out, err := runOpenFluxCommandIn(5*time.Minute, srcDir, env, "go", "build", "-ldflags=-s -w", "-trimpath", "-o", candidate, "."); err != nil {
		return nil, fmt.Errorf("%w: go build failed: %s", ErrOpenFluxInstallFailed, strings.TrimSpace(out))
	}
	if err = os.Chmod(candidate, 0o755); err != nil {
		return nil, err
	}
	versionOut, versionErr := runOpenFluxCommand(10*time.Second, candidate, "--version")
	if versionErr != nil {
		versionOut, versionErr = runOpenFluxCommand(10*time.Second, candidate, "version")
	}
	if versionErr != nil {
		return nil, fmt.Errorf("%w: candidate version check failed: %s", ErrOpenFluxInstallFailed, strings.TrimSpace(versionOut))
	}

	if err = os.MkdirAll(filepath.Dir(openFluxBinaryPath), 0o755); err != nil {
		return nil, err
	}
	var backupPath string
	if _, statErr := os.Stat(openFluxBinaryPath); statErr == nil {
		backupPath = fmt.Sprintf("%s.rollback-%d", openFluxBinaryPath, time.Now().UnixNano())
		if err = os.Rename(openFluxBinaryPath, backupPath); err != nil {
			return nil, fmt.Errorf("%w: backup failed: %v", ErrOpenFluxInstallFailed, err)
		}
	}

	rollback := func(updateErr error) (*OpenFluxInstallResult, error) {
		_ = os.Remove(openFluxBinaryPath)
		if backupPath != "" {
			if restoreErr := os.Rename(backupPath, openFluxBinaryPath); restoreErr != nil {
				return nil, fmt.Errorf("%v; rollback failed: %w", updateErr, restoreErr)
			}
		}
		nodes, _ := s.GetAll()
		for _, node := range nodes {
			if node.Enabled {
				_ = s.ApplyUnit(node.Id)
			}
		}
		return nil, fmt.Errorf("%v; previous OpenFlux binary restored", updateErr)
	}

	if err = copyOpenFluxFile(candidate, openFluxBinaryPath, 0o755); err != nil {
		return rollback(fmt.Errorf("%w: activate failed: %v", ErrOpenFluxInstallFailed, err))
	}
	if _, err = s.BinaryVersion(); err != nil {
		return rollback(fmt.Errorf("%w: installed binary failed version check: %v", ErrOpenFluxInstallFailed, err))
	}

	restarted := 0
	nodes, _ := s.GetAll()
	for _, node := range nodes {
		if !node.Enabled {
			continue
		}
		if err = s.ApplyUnit(node.Id); err != nil {
			return rollback(fmt.Errorf("%w: unit %d failed after update: %v", ErrOpenFluxInstallFailed, node.Id, err))
		}
		restarted++
	}
	if backupPath != "" {
		if err = os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: installed but backup cleanup failed: %v", ErrOpenFluxInstallFailed, err)
		}
		backupPath = ""
	}
	return &OpenFluxInstallResult{
		Path:          openFluxBinaryPath,
		Repo:          openFluxRepoURL,
		Ref:           ref,
		Commit:        commit,
		Version:       strings.TrimSpace(maskOpenFluxSecrets(versionOut)),
		BackupPath:    backupPath,
		RestartedUnit: restarted,
	}, nil
}

func maskOpenFluxSecrets(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--url="); idx >= 0 {
			tail := line[idx+6:]
			end := strings.IndexAny(tail, " \t'")
			if end < 0 {
				end = len(tail)
			}
			lines[i] = line[:idx+6] + "<secret-url>" + tail[end:]
		}
	}
	return strings.Join(lines, "\n")
}

func (s *OpenFluxService) MobileConnections() ([]OpenFluxMobileConnection, error) {
	list, err := s.GetAll()
	if err != nil {
		return nil, err
	}
	out := make([]OpenFluxMobileConnection, 0, len(list))
	for _, node := range list {
		if !node.Enabled {
			continue
		}
		cfg := map[string]any{
			"transport": node.Transport,
			"url":       node.URL,
			"codec":     node.Codec,
			"mode":      node.Mode,
			"debug":     node.Debug,
		}
		rawCfg, _ := json.Marshal(cfg)
		out = append(out, OpenFluxMobileConnection{
			Name:         node.Name,
			Server:       node.ServerID,
			Module:       "openflux_yandex_docs_cursor_ws",
			Protocol:     "openflux",
			Transport:    node.Transport,
			Carrier:      openFluxCarrier(node.Transport),
			ModuleConfig: string(rawCfg),
			Enabled:      node.Enabled,
		})
	}
	return out, nil
}

func openFluxCarrier(transport string) string {
	switch transport {
	case "boards":
		return "cursor_ws"
	case "vyandex":
		return "volga_ws"
	default:
		return "yandex_docs"
	}
}

func (s *OpenFluxService) UnitPreview(id int) (string, error) {
	node, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	return maskOpenFluxSecrets(openFluxUnitContent(node)), nil
}

func OpenFluxUnitHashForTest(node *model.OpenFluxNode) string {
	sum := sha256.Sum256([]byte(openFluxUnitContent(node)))
	return hex.EncodeToString(sum[:])
}
