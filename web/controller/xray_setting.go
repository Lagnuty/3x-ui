package controller

import (
	"encoding/json"
	"strconv"

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
	OutboundSubService service.OutboundSubscriptionService
	XrayService        service.XrayService
	WarpService        service.WarpService
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
	g.POST("/update", a.updateSetting)
	g.POST("/resetOutboundsTraffic", a.resetOutboundsTraffic)
	g.POST("/testOutbound", a.testOutbound)
	g.GET("/outboundSubs", a.listOutboundSubs)
	g.POST("/outboundSubs/create", a.createOutboundSub)
	g.POST("/outboundSubs/update/:id", a.updateOutboundSub)
	g.POST("/outboundSubs/delete/:id", a.deleteOutboundSub)
	g.POST("/outboundSubs/refresh/:id", a.refreshOutboundSub)
	g.POST("/outboundSubs/move/:id", a.moveOutboundSub)
}

// getXraySetting retrieves the Xray configuration template, inbound tags, and outbound test URL.
func (a *XraySettingController) getXraySetting(c *gin.Context) {
	xraySetting, err := a.SettingService.GetXrayConfigTemplate()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	inboundTags, err := a.InboundService.GetInboundTags()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	outboundTestUrl, _ := a.SettingService.GetXrayOutboundTestUrl()
	if outboundTestUrl == "" {
		outboundTestUrl = "https://www.google.com/generate_204"
	}
	panelOutbound, _ := a.SettingService.GetPanelOutbound()
	xrayResponse := map[string]interface{}{
		"xraySetting":     json.RawMessage(xraySetting),
		"inboundTags":     json.RawMessage(inboundTags),
		"outboundTestUrl": outboundTestUrl,
		"panelOutbound":   panelOutbound,
	}
	result, err := json.Marshal(xrayResponse)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.settings.toasts.getSettings"), err)
		return
	}
	jsonObj(c, string(result), nil)
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
	_ = a.SettingService.SetPanelOutbound(c.PostForm("panelOutbound"))
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

type outboundSubForm struct {
	Remark         string `json:"remark" form:"remark"`
	Url            string `json:"url" form:"url"`
	TagPrefix      string `json:"tagPrefix" form:"tagPrefix"`
	Enabled        bool   `json:"enabled" form:"enabled"`
	UpdateInterval int    `json:"updateInterval" form:"updateInterval"`
	AllowPrivate   bool   `json:"allowPrivate" form:"allowPrivate"`
	Prepend        bool   `json:"prepend" form:"prepend"`
}

type outboundSubMoveForm struct {
	Dir string `json:"dir" form:"dir"`
}

func (a *XraySettingController) listOutboundSubs(c *gin.Context) {
	list, err := a.OutboundSubService.List()
	jsonObj(c, list, err)
}

func (a *XraySettingController) createOutboundSub(c *gin.Context) {
	form := &outboundSubForm{Enabled: true, UpdateInterval: 600}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "Failed to create outbound subscription", err)
		return
	}
	sub, err := a.OutboundSubService.Create(form.Remark, form.Url, form.TagPrefix, form.Enabled, form.UpdateInterval, form.AllowPrivate, form.Prepend)
	jsonObj(c, sub, err)
}

func (a *XraySettingController) updateOutboundSub(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	form := &outboundSubForm{Enabled: true, UpdateInterval: 600}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, "Failed to update outbound subscription", err)
		return
	}
	err = a.OutboundSubService.Update(id, form.Remark, form.Url, form.TagPrefix, form.Enabled, form.UpdateInterval, form.AllowPrivate, form.Prepend)
	jsonObj(c, "", err)
}

func (a *XraySettingController) deleteOutboundSub(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	err = a.OutboundSubService.Delete(id)
	if err == nil {
		a.XrayService.SetToNeedRestart()
	}
	jsonObj(c, "", err)
}

func (a *XraySettingController) refreshOutboundSub(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	outbounds, err := a.OutboundSubService.Refresh(id)
	if err == nil {
		a.XrayService.SetToNeedRestart()
	}
	jsonObj(c, outbounds, err)
}

func (a *XraySettingController) moveOutboundSub(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "Invalid id", err)
		return
	}
	form := &outboundSubMoveForm{}
	_ = c.ShouldBind(form)
	up := form.Dir == "up"
	err = a.OutboundSubService.Move(id, up)
	if err == nil {
		a.XrayService.SetToNeedRestart()
	}
	jsonObj(c, "", err)
}
