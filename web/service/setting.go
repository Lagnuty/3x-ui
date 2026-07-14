package service

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/netproxy"
	"github.com/mhsanaei/3x-ui/v2/util/random"
	"github.com/mhsanaei/3x-ui/v2/util/reflect_util"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

//go:embed config.json
var xrayTemplateConfig string

var defaultValueMap = map[string]string{
	"xrayTemplateConfig":          xrayTemplateConfig,
	"webListen":                   "",
	"webDomain":                   "",
	"webPort":                     "2053",
	"webCertFile":                 "",
	"webKeyFile":                  "",
	"secret":                      random.Seq(32),
	"webBasePath":                 "/",
	"sessionMaxAge":               "360",
	"pageSize":                    "25",
	"expireDiff":                  "0",
	"trafficDiff":                 "0",
	"remarkModel":                 "-ieo",
	"timeLocation":                "Local",
	"tgBotEnable":                 "false",
	"tgBotToken":                  "",
	"tgBotProxy":                  "",
	"tgBotAPIServer":              "",
	"tgBotChatId":                 "",
	"tgRunTime":                   "@daily",
	"tgBotBackup":                 "false",
	"tgBotLoginNotify":            "true",
	"tgCpu":                       "80",
	"tgLang":                      "en-US",
	"twoFactorEnable":             "false",
	"twoFactorToken":              "",
	"subEnable":                   "true",
	"subJsonEnable":               "false",
	"subTitle":                    "",
	"subSupportUrl":               "",
	"subProfileUrl":               "",
	"subAnnounce":                 "",
	"subEnableRouting":            "true",
	"subRoutingRules":             "",
	"subListen":                   "",
	"subPort":                     "2096",
	"subPath":                     "/sub/",
	"subDomain":                   "",
	"subCertFile":                 "",
	"subKeyFile":                  "",
	"subUpdates":                  "12",
	"subEncrypt":                  "true",
	"subShowInfo":                 "true",
	"subURI":                      "",
	"subJsonPath":                 "/json/",
	"subJsonURI":                  "",
	"subJsonFragment":             "",
	"subJsonNoises":               "",
	"subJsonMux":                  "",
	"subJsonRules":                "",
	"datepicker":                  "gregorian",
	"warp":                        "",
	"externalTrafficInformEnable": "false",
	"externalTrafficInformURI":    "",
	"xrayOutboundTestUrl":         "https://www.google.com/generate_204",
	"panelOutbound":               "",

	// LDAP defaults
	"ldapEnable":            "false",
	"ldapHost":              "",
	"ldapPort":              "389",
	"ldapUseTLS":            "false",
	"ldapBindDN":            "",
	"ldapPassword":          "",
	"ldapBaseDN":            "",
	"ldapUserFilter":        "(objectClass=person)",
	"ldapUserAttr":          "mail",
	"ldapVlessField":        "vless_enabled",
	"ldapSyncCron":          "@every 1m",
	"ldapFlagField":         "",
	"ldapTruthyValues":      "true,1,yes,on",
	"ldapInvertFlag":        "false",
	"ldapInboundTags":       "",
	"ldapAutoCreate":        "false",
	"ldapAutoDelete":        "false",
	"ldapDefaultTotalGB":    "0",
	"ldapDefaultExpiryDays": "0",
	"ldapDefaultLimitIP":    "0",
}

// SettingService provides business logic for application settings management.
// It handles configuration storage, retrieval, and validation for all system settings.
type SettingService struct{}

func (s *SettingService) GetDefaultJSONConfig() (any, error) {
	var jsonData any
	err := json.Unmarshal([]byte(xrayTemplateConfig), &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func (s *SettingService) GetAllSetting() (*entity.AllSetting, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Not("key = ?", "xrayTemplateConfig").Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := &entity.AllSetting{}
	t := reflect.TypeFor[entity.AllSetting]()
	v := reflect.ValueOf(allSetting).Elem()
	fields := reflect_util.GetFields(t)

	setSetting := func(key, value string) (err error) {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				err = errors.New(fmt.Sprint(panicErr))
			}
		}()

		var found bool
		var field reflect.StructField
		for _, f := range fields {
			if f.Tag.Get("json") == key {
				field = f
				found = true
				break
			}
		}

		if !found {
			// Some settings are automatically generated, no need to return to the front end to modify the user
			return nil
		}

		fieldV := v.FieldByName(field.Name)
		switch t := fieldV.Interface().(type) {
		case int:
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			fieldV.SetInt(n)
		case string:
			fieldV.SetString(value)
		case bool:
			fieldV.SetBool(value == "true")
		default:
			return common.NewErrorf("unknown field %v type %v", key, t)
		}
		return
	}

	keyMap := map[string]bool{}
	for _, setting := range settings {
		err := setSetting(setting.Key, setting.Value)
		if err != nil {
			return nil, err
		}
		keyMap[setting.Key] = true
	}

	for key, value := range defaultValueMap {
		if keyMap[key] {
			continue
		}
		err := setSetting(key, value)
		if err != nil {
			return nil, err
		}
	}

	return allSetting, nil
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	err := db.Where("1 = 1").Delete(model.Setting{}).Error
	if err != nil {
		return err
	}
	return db.Model(model.User{}).
		Where("1 = 1").Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}

// ApplyEnvironmentOverrides persists Docker-friendly environment overrides
// before servers bind their listeners. This keeps the old UI and database as
// the source of truth while allowing first boot configuration from compose.
func (s *SettingService) ApplyEnvironmentOverrides() error {
	overrides := map[string]string{
		"XUI_WEB_LISTEN":      "webListen",
		"XUI_WEB_PORT":        "webPort",
		"XUI_WEB_BASE_PATH":   "webBasePath",
		"XUI_SUB_ENABLE":      "subEnable",
		"XUI_SUB_LISTEN":      "subListen",
		"XUI_SUB_PORT":        "subPort",
		"XUI_SUB_PATH":        "subPath",
		"XUI_SUB_JSON_PATH":   "subJsonPath",
		"XUI_SUB_DOMAIN":      "subDomain",
		"XUI_SUB_URI":         "subURI",
		"XUI_SUB_JSON_URI":    "subJsonURI",
		"XUI_SUB_JSON_ENABLE": "subJsonEnable",
	}
	for envName, key := range overrides {
		value, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if err := validateEnvironmentOverride(key, value); err != nil {
			return common.NewErrorf("%s invalid: %v", envName, err)
		}
		if err := s.saveSetting(key, value); err != nil {
			return err
		}
		logger.Infof("Applied %s to setting %s", envName, key)
	}
	if err := s.applyXrayTemplateEnvironmentOverrides(); err != nil {
		return err
	}
	return nil
}

func (s *SettingService) applyXrayTemplateEnvironmentOverrides() error {
	apiPort, hasAPI := os.LookupEnv("XUI_XRAY_API_PORT")
	metricsPort, hasMetrics := os.LookupEnv("XUI_XRAY_METRICS_PORT")
	if !hasAPI && !hasMetrics {
		return nil
	}

	templateConfig, err := s.GetXrayConfigTemplate()
	if err != nil {
		return err
	}
	cfg := map[string]any{}
	if err := json.Unmarshal([]byte(templateConfig), &cfg); err != nil {
		return err
	}

	changed := false
	if hasAPI {
		port, err := parseEnvironmentPort("XUI_XRAY_API_PORT", apiPort)
		if err != nil {
			return err
		}
		if inbounds, ok := cfg["inbounds"].([]any); ok {
			for _, item := range inbounds {
				inbound, ok := item.(map[string]any)
				if !ok || inbound["tag"] != "api" {
					continue
				}
				inbound["port"] = float64(port)
				changed = true
				logger.Infof("Applied XUI_XRAY_API_PORT to Xray API inbound: %d", port)
				break
			}
		}
	}
	if hasMetrics {
		port, err := parseEnvironmentPort("XUI_XRAY_METRICS_PORT", metricsPort)
		if err != nil {
			return err
		}
		metrics, ok := cfg["metrics"].(map[string]any)
		if !ok {
			metrics = map[string]any{}
			cfg["metrics"] = metrics
		}
		metrics["listen"] = fmt.Sprintf("127.0.0.1:%d", port)
		changed = true
		logger.Infof("Applied XUI_XRAY_METRICS_PORT to Xray metrics listen: %d", port)
	}
	if !changed {
		return nil
	}

	updated, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return s.saveSetting("xrayTemplateConfig", string(updated))
}

func parseEnvironmentPort(name string, value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		return 0, common.NewErrorf("%s must be 1..65535", name)
	}
	return port, nil
}

func validateEnvironmentOverride(key string, value string) error {
	switch key {
	case "webPort", "subPort":
		if _, err := parseEnvironmentPort(key, value); err != nil {
			return err
		}
	case "subEnable", "subJsonEnable":
		if _, err := strconv.ParseBool(value); err != nil {
			return common.NewError("boolean must be true or false")
		}
	case "webListen", "subListen":
		if value != "" && net.ParseIP(value) == nil {
			return common.NewError("listen address must be empty or a valid IP")
		}
	case "webBasePath", "subPath", "subJsonPath":
		if value == "" {
			return common.NewError("path cannot be empty")
		}
	}
	return nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getBool(key string) (bool, error) {
	str, err := s.getString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

func (s *SettingService) setBool(key string, value bool) error {
	return s.setString(key, strconv.FormatBool(value))
}

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}

func (s *SettingService) GetXrayConfigTemplate() (string, error) {
	return s.getString("xrayTemplateConfig")
}

func (s *SettingService) GetXrayOutboundTestUrl() (string, error) {
	return s.getString("xrayOutboundTestUrl")
}

func (s *SettingService) SetXrayOutboundTestUrl(url string) error {
	return s.setString("xrayOutboundTestUrl", url)
}
func (s *SettingService) GetPanelOutbound() (string, error) {
	return s.getString("panelOutbound")
}

func (s *SettingService) SetPanelOutbound(tag string) error {
	return s.setString("panelOutbound", strings.TrimSpace(tag))
}

func (s *SettingService) PanelEgressProxyURL() string {
	tag, err := s.GetPanelOutbound()
	if err != nil || strings.TrimSpace(tag) == "" {
		return ""
	}
	proc := XrayProcess()
	if proc == nil || !proc.IsRunning() {
		logger.Warning("panel outbound [", tag, "] is set but Xray is not running, using a direct connection")
		return ""
	}
	cfg := proc.GetConfig()
	if cfg == nil {
		return ""
	}
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag == PanelEgressInboundTag {
			return fmt.Sprintf("socks5://127.0.0.1:%d", cfg.InboundConfigs[i].Port)
		}
	}
	logger.Warning("panel outbound [", tag, "] is set but the egress bridge is not in the running config, using a direct connection")
	return ""
}

func (s *SettingService) NewProxiedHTTPClient(timeout time.Duration) *http.Client {
	client, err := netproxy.NewHTTPClient(s.PanelEgressProxyURL(), timeout)
	if err != nil {
		logger.Warning("invalid panel egress proxy, using direct connection:", err)
		return &http.Client{Timeout: timeout}
	}
	return client
}

func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

func (s *SettingService) SetListen(ip string) error {
	return s.setString("webListen", ip)
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) GetTgBotToken() (string, error) {
	return s.getString("tgBotToken")
}

func (s *SettingService) SetTgBotToken(token string) error {
	return s.setString("tgBotToken", token)
}

func (s *SettingService) GetTgBotProxy() (string, error) {
	return s.getString("tgBotProxy")
}

func (s *SettingService) SetTgBotProxy(token string) error {
	return s.setString("tgBotProxy", token)
}

func (s *SettingService) GetTgBotAPIServer() (string, error) {
	return s.getString("tgBotAPIServer")
}

func (s *SettingService) SetTgBotAPIServer(token string) error {
	return s.setString("tgBotAPIServer", token)
}

func (s *SettingService) GetTgBotChatId() (string, error) {
	return s.getString("tgBotChatId")
}

func (s *SettingService) SetTgBotChatId(chatIds string) error {
	return s.setString("tgBotChatId", chatIds)
}

func (s *SettingService) GetTgbotEnabled() (bool, error) {
	return s.getBool("tgBotEnable")
}

func (s *SettingService) SetTgbotEnabled(value bool) error {
	return s.setBool("tgBotEnable", value)
}

func (s *SettingService) GetTgbotRuntime() (string, error) {
	return s.getString("tgRunTime")
}

func (s *SettingService) SetTgbotRuntime(time string) error {
	return s.setString("tgRunTime", time)
}

func (s *SettingService) GetTgBotBackup() (bool, error) {
	return s.getBool("tgBotBackup")
}

func (s *SettingService) GetTgBotLoginNotify() (bool, error) {
	return s.getBool("tgBotLoginNotify")
}

func (s *SettingService) GetTgCpu() (int, error) {
	return s.getInt("tgCpu")
}

func (s *SettingService) GetTgLang() (string, error) {
	return s.getString("tgLang")
}

func (s *SettingService) GetTwoFactorEnable() (bool, error) {
	return s.getBool("twoFactorEnable")
}

func (s *SettingService) SetTwoFactorEnable(value bool) error {
	return s.setBool("twoFactorEnable", value)
}

func (s *SettingService) GetTwoFactorToken() (string, error) {
	return s.getString("twoFactorToken")
}

func (s *SettingService) SetTwoFactorToken(value string) error {
	return s.setString("twoFactorToken", value)
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

func (s *SettingService) SetPort(port int) error {
	return s.setInt("webPort", port)
}

func (s *SettingService) SetCertFile(webCertFile string) error {
	return s.setString("webCertFile", webCertFile)
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) SetKeyFile(webKeyFile string) error {
	return s.setString("webKeyFile", webKeyFile)
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) GetExpireDiff() (int, error) {
	return s.getInt("expireDiff")
}

func (s *SettingService) GetTrafficDiff() (int, error) {
	return s.getInt("trafficDiff")
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetRemarkModel() (string, error) {
	return s.getString("remarkModel")
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		err := s.saveSetting("secret", secret)
		if err != nil {
			logger.Warning("save secret failed:", err)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) SetBasePath(basePath string) error {
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	return s.setString("webBasePath", basePath)
}

func (s *SettingService) GetBasePath() (string, error) {
	basePath, err := s.getString("webBasePath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	return basePath, nil
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	l, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(l)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", l, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) GetSubEnable() (bool, error) {
	return s.getBool("subEnable")
}

func (s *SettingService) GetSubJsonEnable() (bool, error) {
	return s.getBool("subJsonEnable")
}

func (s *SettingService) GetSubTitle() (string, error) {
	return s.getString("subTitle")
}

func (s *SettingService) GetSubSupportUrl() (string, error) {
	return s.getString("subSupportUrl")
}

func (s *SettingService) GetSubProfileUrl() (string, error) {
	return s.getString("subProfileUrl")
}

func (s *SettingService) GetSubAnnounce() (string, error) {
	return s.getString("subAnnounce")
}

func (s *SettingService) GetSubEnableRouting() (bool, error) {
	return s.getBool("subEnableRouting")
}

func (s *SettingService) GetSubRoutingRules() (string, error) {
	return s.getString("subRoutingRules")
}

func (s *SettingService) GetSubListen() (string, error) {
	return s.getString("subListen")
}

func (s *SettingService) GetSubPort() (int, error) {
	return s.getInt("subPort")
}

func (s *SettingService) GetSubPath() (string, error) {
	return s.getString("subPath")
}

func (s *SettingService) GetSubJsonPath() (string, error) {
	return s.getString("subJsonPath")
}

func (s *SettingService) GetSubDomain() (string, error) {
	return s.getString("subDomain")
}

func (s *SettingService) SetSubCertFile(subCertFile string) error {
	return s.setString("subCertFile", subCertFile)
}

func (s *SettingService) GetSubCertFile() (string, error) {
	return s.getString("subCertFile")
}

func (s *SettingService) SetSubKeyFile(subKeyFile string) error {
	return s.setString("subKeyFile", subKeyFile)
}

func (s *SettingService) GetSubKeyFile() (string, error) {
	return s.getString("subKeyFile")
}

func (s *SettingService) GetSubUpdates() (string, error) {
	return s.getString("subUpdates")
}

func (s *SettingService) GetSubEncrypt() (bool, error) {
	return s.getBool("subEncrypt")
}

func (s *SettingService) GetSubShowInfo() (bool, error) {
	return s.getBool("subShowInfo")
}

func (s *SettingService) GetPageSize() (int, error) {
	return s.getInt("pageSize")
}

func (s *SettingService) GetSubURI() (string, error) {
	return s.getString("subURI")
}

func (s *SettingService) GetSubJsonURI() (string, error) {
	return s.getString("subJsonURI")
}

func (s *SettingService) GetSubJsonFragment() (string, error) {
	return s.getString("subJsonFragment")
}

func (s *SettingService) GetSubJsonNoises() (string, error) {
	return s.getString("subJsonNoises")
}

func (s *SettingService) GetSubJsonMux() (string, error) {
	return s.getString("subJsonMux")
}

func (s *SettingService) GetSubJsonRules() (string, error) {
	return s.getString("subJsonRules")
}

func (s *SettingService) GetDatepicker() (string, error) {
	return s.getString("datepicker")
}

func (s *SettingService) GetWarp() (string, error) {
	return s.getString("warp")
}

func (s *SettingService) SetWarp(data string) error {
	return s.setString("warp", data)
}

func (s *SettingService) GetExternalTrafficInformEnable() (bool, error) {
	return s.getBool("externalTrafficInformEnable")
}

func (s *SettingService) SetExternalTrafficInformEnable(value bool) error {
	return s.setBool("externalTrafficInformEnable", value)
}

func (s *SettingService) GetExternalTrafficInformURI() (string, error) {
	return s.getString("externalTrafficInformURI")
}

func (s *SettingService) SetExternalTrafficInformURI(InformURI string) error {
	return s.setString("externalTrafficInformURI", InformURI)
}

func (s *SettingService) GetIpLimitEnable() (bool, error) {
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return false, err
	}
	return (accessLogPath != "none" && accessLogPath != ""), nil
}

// GetLdapEnable returns whether LDAP is enabled.
func (s *SettingService) GetLdapEnable() (bool, error) {
	return s.getBool("ldapEnable")
}

func (s *SettingService) GetLdapHost() (string, error) {
	return s.getString("ldapHost")
}

func (s *SettingService) GetLdapPort() (int, error) {
	return s.getInt("ldapPort")
}

func (s *SettingService) GetLdapUseTLS() (bool, error) {
	return s.getBool("ldapUseTLS")
}

func (s *SettingService) GetLdapBindDN() (string, error) {
	return s.getString("ldapBindDN")
}

func (s *SettingService) GetLdapPassword() (string, error) {
	return s.getString("ldapPassword")
}

func (s *SettingService) GetLdapBaseDN() (string, error) {
	return s.getString("ldapBaseDN")
}

func (s *SettingService) GetLdapUserFilter() (string, error) {
	return s.getString("ldapUserFilter")
}

func (s *SettingService) GetLdapUserAttr() (string, error) {
	return s.getString("ldapUserAttr")
}

func (s *SettingService) GetLdapVlessField() (string, error) {
	return s.getString("ldapVlessField")
}

func (s *SettingService) GetLdapSyncCron() (string, error) {
	return s.getString("ldapSyncCron")
}

func (s *SettingService) GetLdapFlagField() (string, error) {
	return s.getString("ldapFlagField")
}

func (s *SettingService) GetLdapTruthyValues() (string, error) {
	return s.getString("ldapTruthyValues")
}

func (s *SettingService) GetLdapInvertFlag() (bool, error) {
	return s.getBool("ldapInvertFlag")
}

func (s *SettingService) GetLdapInboundTags() (string, error) {
	return s.getString("ldapInboundTags")
}

func (s *SettingService) GetLdapAutoCreate() (bool, error) {
	return s.getBool("ldapAutoCreate")
}

func (s *SettingService) GetLdapAutoDelete() (bool, error) {
	return s.getBool("ldapAutoDelete")
}

func (s *SettingService) GetLdapDefaultTotalGB() (int, error) {
	return s.getInt("ldapDefaultTotalGB")
}

func (s *SettingService) GetLdapDefaultExpiryDays() (int, error) {
	return s.getInt("ldapDefaultExpiryDays")
}

func (s *SettingService) GetLdapDefaultLimitIP() (int, error) {
	return s.getInt("ldapDefaultLimitIP")
}

func (s *SettingService) UpdateAllSetting(allSetting *entity.AllSetting) error {
	if err := allSetting.CheckValid(); err != nil {
		return err
	}

	v := reflect.ValueOf(allSetting).Elem()
	t := reflect.TypeFor[entity.AllSetting]()
	fields := reflect_util.GetFields(t)
	errs := make([]error, 0)
	for _, field := range fields {
		key := field.Tag.Get("json")
		fieldV := v.FieldByName(field.Name)
		value := fmt.Sprint(fieldV.Interface())
		err := s.saveSetting(key, value)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return common.Combine(errs...)
}

func (s *SettingService) GetDefaultXrayConfig() (any, error) {
	var jsonData any
	err := json.Unmarshal([]byte(xrayTemplateConfig), &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

func extractHostname(host string) string {
	h, _, err := net.SplitHostPort(host)
	// Err is not nil means host does not contain port
	if err != nil {
		h = host
	}

	ip := net.ParseIP(h)
	// If it's not an IP, return as is
	if ip == nil {
		return h
	}

	// If it's an IPv4, return as is
	if ip.To4() != nil {
		return h
	}

	// IPv6 needs bracketing
	return "[" + h + "]"
}

func (s *SettingService) GetDefaultSettings(host string) (any, error) {
	type settingFunc func() (any, error)
	settings := map[string]settingFunc{
		"expireDiff":    func() (any, error) { return s.GetExpireDiff() },
		"trafficDiff":   func() (any, error) { return s.GetTrafficDiff() },
		"pageSize":      func() (any, error) { return s.GetPageSize() },
		"defaultCert":   func() (any, error) { return s.GetCertFile() },
		"defaultKey":    func() (any, error) { return s.GetKeyFile() },
		"tgBotEnable":   func() (any, error) { return s.GetTgbotEnabled() },
		"subEnable":     func() (any, error) { return s.GetSubEnable() },
		"subJsonEnable": func() (any, error) { return s.GetSubJsonEnable() },
		"subTitle":      func() (any, error) { return s.GetSubTitle() },
		"subURI":        func() (any, error) { return s.GetSubURI() },
		"subJsonURI":    func() (any, error) { return s.GetSubJsonURI() },
		"remarkModel":   func() (any, error) { return s.GetRemarkModel() },
		"datepicker":    func() (any, error) { return s.GetDatepicker() },
		"ipLimitEnable": func() (any, error) { return s.GetIpLimitEnable() },
	}

	result := make(map[string]any)

	for key, fn := range settings {
		value, err := fn()
		if err != nil {
			return "", err
		}
		result[key] = value
	}

	subEnable := result["subEnable"].(bool)
	subJsonEnable := false
	if v, ok := result["subJsonEnable"]; ok {
		if b, ok2 := v.(bool); ok2 {
			subJsonEnable = b
		}
	}
	if (subEnable && result["subURI"].(string) == "") || (subJsonEnable && result["subJsonURI"].(string) == "") {
		subURI := ""
		subTitle, _ := s.GetSubTitle()
		subPort, _ := s.GetSubPort()
		subPath, _ := s.GetSubPath()
		subJsonPath, _ := s.GetSubJsonPath()
		subDomain, _ := s.GetSubDomain()
		subKeyFile, _ := s.GetSubKeyFile()
		subCertFile, _ := s.GetSubCertFile()
		subTLS := false
		if subKeyFile != "" && subCertFile != "" {
			subTLS = true
		}
		if subDomain == "" {
			subDomain = extractHostname(host)
		}
		if subTLS {
			subURI = "https://"
		} else {
			subURI = "http://"
		}
		if (subPort == 443 && subTLS) || (subPort == 80 && !subTLS) {
			subURI += subDomain
		} else {
			subURI += fmt.Sprintf("%s:%d", subDomain, subPort)
		}
		if subEnable && result["subURI"].(string) == "" {
			result["subURI"] = subURI + subPath
		}
		if result["subTitle"].(string) == "" {
			result["subTitle"] = subTitle
		}
		if subJsonEnable && result["subJsonURI"].(string) == "" {
			result["subJsonURI"] = subURI + subJsonPath
		}
	}

	return result, nil
}
