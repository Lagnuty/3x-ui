package controller

import (
	"encoding/json"
	"fmt"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// XraySettingController handles Xray configuration and settings operations.
type XraySettingController struct {
	XraySettingService service.XraySettingService
	SettingService     service.SettingService
	InboundService     service.InboundService
	OutboundService    service.OutboundService
	XrayService        service.XrayService
	WarpService        service.WarpService
	NordService        service.NordService
}

// NewXraySettingController creates a new XraySettingController and initializes its routes.
func NewXraySettingController(g *gin.RouterGroup) *XraySettingController {
	a := &XraySettingController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for Xray settings management.
func (a *XraySettingController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xray")
	g.GET("/getDefaultJsonConfig", a.getDefaultXrayConfig)
	g.GET("/getOutboundsTraffic", a.getOutboundsTraffic)
	g.GET("/getXrayResult", a.getXrayResult)

	g.POST("/", a.getXraySetting)
	g.POST("/warp/:action", a.warp)
	g.POST("/nord/:action", a.nord)
	g.POST("/update", a.updateSetting)
	g.POST("/resetOutboundsTraffic", a.resetOutboundsTraffic)
	g.POST("/testOutbound", a.testOutbound)
	g.POST("/migrateLegacyReverse", a.migrateLegacyReverse)
}

// getXraySetting retrieves the Xray configuration template, inbound tags, and outbound test URL.
func (a *XraySettingController) getXraySetting(c *gin.Context) {
	xraySetting, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	// Older versions of this handler embedded the raw DB value as
	// `xraySetting` in the response without checking if the value
	// already had that wrapper shape. When the frontend saved it
	// back through the textarea verbatim, the wrapper got persisted
	// and every subsequent save nested another layer, which is what
	// eventually produced the blank Xray Settings page in #4059.
	// Strip any such wrapper here, and heal the DB if we found one so
	// the next read is O(1) instead of climbing the same pile again.
	if unwrapped := service.UnwrapXrayTemplateConfig(xraySetting); unwrapped != xraySetting {
		if saveErr := a.XraySettingService.SaveXraySetting(unwrapped); saveErr == nil {
			xraySetting = unwrapped
		} else {
			// Don't fail the read — just serve the unwrapped value
			// and leave the DB healing for a later save.
			xraySetting = unwrapped
		}
	}
	inboundTags, err := a.InboundService.GetInboundTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	clientReverseTags, err := a.InboundService.GetClientReverseTags()
	if err != nil {
		clientReverseTags = "[]"
	}
	outboundTestUrl, _ := a.SettingService.GetXrayOutboundTestUrl()
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	vlessReverseCandidates := make([]map[string]any, 0)
	if inbounds, candidatesErr := a.InboundService.GetAllInbounds(); candidatesErr == nil {
		for _, inbound := range inbounds {
			if inbound.Protocol != model.VLESS {
				continue
			}
			clients, _ := a.InboundService.GetClients(inbound)
			for _, client := range clients {
				vlessReverseCandidates = append(vlessReverseCandidates, map[string]any{
					"inboundId":     inbound.Id,
					"inboundRemark": inbound.Remark,
					"clientId":      client.ID,
					"clientEmail":   client.Email,
				})
			}
		}
	}
	xrayResponse := map[string]any{
		"xraySetting":            json.RawMessage(xraySetting),
		"inboundTags":            json.RawMessage(inboundTags),
		"clientReverseTags":      json.RawMessage(clientReverseTags),
		"outboundTestUrl":        outboundTestUrl,
		"vlessReverseCandidates": vlessReverseCandidates,
	}
	result, err := json.Marshal(xrayResponse)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, string(result), nil)
}

type legacyReverseBridgeMapping struct {
	LegacyTag   string `json:"legacyTag"`
	OutboundTag string `json:"outboundTag"`
}

type legacyReversePortalMapping struct {
	LegacyTag string `json:"legacyTag"`
	InboundID int    `json:"inboundId"`
	ClientID  string `json:"clientId"`
}

func removeMappedLegacyEntries(entries []any, mapped map[string]bool) []any {
	remaining := make([]any, 0, len(entries))
	for _, entry := range entries {
		entryMap, _ := entry.(map[string]any)
		tag, _ := entryMap["tag"].(string)
		if !mapped[tag] {
			remaining = append(remaining, entry)
		}
	}
	return remaining
}

// migrateLegacyReverse maps deprecated top-level reverse bridges/portals to
// VLESS reverse accounts. The legacy tags are retained so existing routing
// rules keep working.
func (a *XraySettingController) migrateLegacyReverse(c *gin.Context) {
	var request struct {
		Bridges []legacyReverseBridgeMapping `json:"bridges"`
		Portals []legacyReversePortalMapping `json:"portals"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		jsonMsg(c, "Invalid legacy reverse migration", err)
		return
	}

	raw, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, "Failed to load Xray template", err)
		return
	}
	raw = service.UnwrapXrayTemplateConfig(raw)
	var config map[string]any
	if err = json.Unmarshal([]byte(raw), &config); err != nil {
		jsonMsg(c, "Invalid Xray template", err)
		return
	}
	legacy, ok := config["reverse"].(map[string]any)
	if !ok {
		jsonMsg(c, "No legacy reverse configuration found", fmt.Errorf("reverse section is missing"))
		return
	}
	bridges, _ := legacy["bridges"].([]any)
	portals, _ := legacy["portals"].([]any)
	if len(request.Bridges) != len(bridges) || len(request.Portals) != len(portals) {
		jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("every legacy bridge and portal must be mapped"))
		return
	}
	legacyBridgeTags := map[string]bool{}
	legacyPortalTags := map[string]bool{}
	for _, entry := range bridges {
		entryMap, _ := entry.(map[string]any)
		tag, _ := entryMap["tag"].(string)
		legacyBridgeTags[tag] = true
	}
	for _, entry := range portals {
		entryMap, _ := entry.(map[string]any)
		tag, _ := entryMap["tag"].(string)
		legacyPortalTags[tag] = true
	}

	mappedBridges := map[string]bool{}
	selectedOutbounds := map[string]bool{}
	outbounds, _ := config["outbounds"].([]any)
	for _, mapping := range request.Bridges {
		if !legacyBridgeTags[mapping.LegacyTag] || mappedBridges[mapping.LegacyTag] {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("invalid or duplicate bridge tag %q", mapping.LegacyTag))
			return
		}
		if selectedOutbounds[mapping.OutboundTag] {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("VLESS outbound %q is mapped more than once", mapping.OutboundTag))
			return
		}
		selectedOutbounds[mapping.OutboundTag] = true
		found := false
		for _, rawOutbound := range outbounds {
			outbound, _ := rawOutbound.(map[string]any)
			if outbound["tag"] != mapping.OutboundTag || outbound["protocol"] != "vless" {
				continue
			}
			settings, _ := outbound["settings"].(map[string]any)
			if settings == nil {
				settings = map[string]any{}
				outbound["settings"] = settings
			}
			settings["reverse"] = map[string]any{"tag": mapping.LegacyTag}
			mappedBridges[mapping.LegacyTag] = true
			found = true
			break
		}
		if !found {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("VLESS outbound %q was not found", mapping.OutboundTag))
			return
		}
	}

	mappedPortals := map[string]bool{}
	selectedClients := map[string]bool{}
	for _, mapping := range request.Portals {
		if !legacyPortalTags[mapping.LegacyTag] || mappedPortals[mapping.LegacyTag] {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("invalid or duplicate portal tag %q", mapping.LegacyTag))
			return
		}
		clientKey := fmt.Sprintf("%d/%s", mapping.InboundID, mapping.ClientID)
		if selectedClients[clientKey] {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("VLESS client %q is mapped more than once", mapping.ClientID))
			return
		}
		selectedClients[clientKey] = true
		inbound, getErr := a.InboundService.GetInbound(mapping.InboundID)
		if getErr != nil || inbound.Protocol != model.VLESS {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("VLESS inbound %d was not found", mapping.InboundID))
			return
		}
		clients, getErr := a.InboundService.GetClients(inbound)
		if getErr != nil {
			jsonMsg(c, "Legacy reverse migration failed", getErr)
			return
		}
		var selected *model.Client
		for i := range clients {
			if clients[i].ID == mapping.ClientID {
				client := clients[i]
				client.Reverse = &model.ClientReverse{Tag: mapping.LegacyTag}
				selected = &client
				break
			}
		}
		if selected == nil {
			jsonMsg(c, "Legacy reverse migration failed", fmt.Errorf("client %q was not found", mapping.ClientID))
			return
		}
		settings, _ := json.Marshal(map[string]any{"clients": []model.Client{*selected}})
		update := &model.Inbound{Id: inbound.Id, Settings: string(settings)}
		needRestart, updateErr := a.InboundService.UpdateInboundClient(update, mapping.ClientID)
		if updateErr != nil {
			jsonMsg(c, "Legacy reverse migration failed", updateErr)
			return
		}
		if needRestart {
			a.XrayService.SetToNeedRestart()
		}
		mappedPortals[mapping.LegacyTag] = true
	}

	legacy["bridges"] = removeMappedLegacyEntries(bridges, mappedBridges)
	legacy["portals"] = removeMappedLegacyEntries(portals, mappedPortals)
	if len(legacy["bridges"].([]any)) == 0 && len(legacy["portals"].([]any)) == 0 {
		delete(config, "reverse")
	}
	updated, err := json.MarshalIndent(config, "", "  ")
	if err == nil {
		err = a.XraySettingService.SaveXraySetting(string(updated))
	}
	if err != nil {
		jsonMsg(c, "Failed to save migrated Xray template", err)
		return
	}
	a.XrayService.SetToNeedRestart()
	jsonMsg(c, "Legacy reverse migrated to VLESS reverse; restart Xray to apply it", nil)
}

// updateSetting updates the Xray configuration settings.
func (a *XraySettingController) updateSetting(c *gin.Context) {
	xraySetting := c.PostForm("xraySetting")
	if err := a.XraySettingService.SaveXraySetting(xraySetting); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), err)
		return
	}
	outboundTestUrl := c.PostForm("outboundTestUrl")
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	_ = a.SettingService.SetXrayOutboundTestUrl(outboundTestUrl)
	jsonMsg(c, I18nWeb(c, "pages.settings.toasts.modifySettings"), nil)
}

// getDefaultXrayConfig retrieves the default Xray configuration.
func (a *XraySettingController) getDefaultXrayConfig(c *gin.Context) {
	defaultJsonConfig, err := a.SettingService.GetDefaultXrayConfig()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, defaultJsonConfig, nil)
}

// getXrayResult retrieves the current Xray service result.
func (a *XraySettingController) getXrayResult(c *gin.Context) {
	jsonObj(c, a.XrayService.GetXrayResult(), nil)
}

// warp handles Warp-related operations based on the action parameter.
func (a *XraySettingController) warp(c *gin.Context) {
	action := c.Param("action")
	var resp string
	var err error
	switch action {
	case "data":
		resp, err = a.WarpService.GetWarpData()
	case "del":
		err = a.WarpService.DelWarpData()
	case "config":
		resp, err = a.WarpService.GetWarpConfig()
	case "reg":
		skey := c.PostForm("privateKey")
		pkey := c.PostForm("publicKey")
		resp, err = a.WarpService.RegWarp(skey, pkey)
	case "license":
		license := c.PostForm("license")
		resp, err = a.WarpService.SetWarpLicense(license)
	}

	jsonObj(c, resp, err)
}

// nord handles NordVPN-related operations based on the action parameter.
func (a *XraySettingController) nord(c *gin.Context) {
	action := c.Param("action")
	var resp string
	var err error
	switch action {
	case "countries":
		resp, err = a.NordService.GetCountries()
	case "servers":
		countryId := c.PostForm("countryId")
		resp, err = a.NordService.GetServers(countryId)
	case "reg":
		token := c.PostForm("token")
		resp, err = a.NordService.GetCredentials(token)
	case "setKey":
		key := c.PostForm("key")
		resp, err = a.NordService.SetKey(key)
	case "data":
		resp, err = a.NordService.GetNordData()
	case "del":
		err = a.NordService.DelNordData()
	}

	jsonObj(c, resp, err)
}

// getOutboundsTraffic retrieves the traffic statistics for outbounds.
func (a *XraySettingController) getOutboundsTraffic(c *gin.Context) {
	outboundsTraffic, err := a.OutboundService.GetOutboundsTraffic()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getOutboundTrafficError"), err)
		return
	}
	jsonObj(c, outboundsTraffic, nil)
}

// resetOutboundsTraffic resets the traffic statistics for the specified outbound tag.
func (a *XraySettingController) resetOutboundsTraffic(c *gin.Context) {
	tag := c.PostForm("tag")
	err := a.OutboundService.ResetOutboundTraffic(tag)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.resetOutboundTrafficError"), err)
		return
	}
	jsonObj(c, "", nil)
}

// testOutbound tests an outbound configuration and returns the delay/response time.
// Optional form "allOutbounds": JSON array of all outbounds; used to resolve sockopt.dialerProxy dependencies.
func (a *XraySettingController) testOutbound(c *gin.Context) {
	outboundJSON := c.PostForm("outbound")
	allOutboundsJSON := c.PostForm("allOutbounds")

	if outboundJSON == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("outbound parameter is required"))
		return
	}

	// Load the test URL from server settings to prevent SSRF via user-controlled URLs
	testURL, _ := a.SettingService.GetXrayOutboundTestUrl()

	result, err := a.OutboundService.TestOutbound(outboundJSON, testURL, allOutboundsJSON)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}

	jsonObj(c, result, nil)
}
