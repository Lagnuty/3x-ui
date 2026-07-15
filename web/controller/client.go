package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

type ClientController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewClientController(g *gin.RouterGroup) *ClientController {
	a := &ClientController{}
	a.initRouter(g)
	return a
}

func (a *ClientController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/get/:email", a.get)
	g.GET("/traffic/:email", a.getTrafficByEmail)
	g.POST("/:email/attach", a.attach)
	g.POST("/:email/detach", a.detach)
	g.POST("/bulkAttach", a.bulkAttach)
	g.POST("/bulkDetach", a.bulkDetach)
	g.POST("/bulkAdjust", a.bulkAdjust)
	g.POST("/bulkSpeedLimit", a.bulkSpeedLimit)
	g.POST("/bulkDel", a.bulkDelete)
	g.POST("/bulkResetTraffic", a.bulkResetTraffic)
	g.POST("/resetTraffic/:email", a.resetTrafficByEmail)
	g.POST("/updateTraffic/:email", a.updateTrafficByEmail)
	g.POST("/ips/:email", a.getIps)
	g.POST("/clearIps/:email", a.clearIps)
	g.POST("/onlines", a.onlines)
	g.POST("/lastOnline", a.lastOnline)
}

type attachDetachBody struct {
	InboundIds []int `json:"inboundIds"`
}

func (a *ClientController) attach(c *gin.Context) {
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.AttachByEmail(&a.inboundService, c.Param("email"), body.InboundIds)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), err)
}

func (a *ClientController) detach(c *gin.Context) {
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	needRestart, err := a.clientService.DetachByEmailMany(&a.inboundService, c.Param("email"), body.InboundIds)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), err)
}

type bulkAttachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

type bulkAdjustRequest struct {
	Emails   []string `json:"emails"`
	AddDays  int      `json:"addDays"`
	AddBytes int64    `json:"addBytes"`
}

type bulkSpeedLimitRequest struct {
	SpeedLimitUpMbps   int64 `json:"speedLimitUpMbps"`
	SpeedLimitDownMbps int64 `json:"speedLimitDownMbps"`
}

type bulkDeleteRequest struct {
	Emails      []string `json:"emails"`
	KeepTraffic bool     `json:"keepTraffic"`
}

func (a *ClientController) bulkAttach(c *gin.Context) {
	var req bulkAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkAttach(&a.inboundService, req.Emails, req.InboundIds)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonObj(c, result, err)
}

func (a *ClientController) bulkDetach(c *gin.Context) {
	var req bulkAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkDetach(&a.inboundService, req.Emails, req.InboundIds)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonObj(c, result, err)
}

func (a *ClientController) bulkAdjust(c *gin.Context) {
	var req bulkAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkAdjust(&a.inboundService, req.Emails, req.AddDays, req.AddBytes)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonObj(c, result, err)
}

func (a *ClientController) bulkSpeedLimit(c *gin.Context) {
	var req bulkSpeedLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkSetSpeedLimits(req.SpeedLimitUpMbps, req.SpeedLimitDownMbps)
	if err == nil && needRestart {
		if restartErr := a.xrayService.RestartXray(true); restartErr != nil {
			a.xrayService.SetToNeedRestart()
			jsonObj(c, result, restartErr)
			return
		}
	}
	jsonObj(c, result, err)
}

func (a *ClientController) bulkDelete(c *gin.Context) {
	var req bulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	result, needRestart, err := a.clientService.BulkDelete(&a.inboundService, req.Emails, req.KeepTraffic)
	if needRestart {
		a.xrayService.SetToNeedRestart()
	}
	jsonObj(c, result, err)
}

type bulkResetRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkResetTraffic(c *gin.Context) {
	var req bulkResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	affected, err := a.clientService.BulkResetTraffic(&a.inboundService, req.Emails)
	if err == nil && affected > 0 {
		a.xrayService.SetToNeedRestart()
	}
	jsonObj(c, gin.H{"affected": affected}, err)
}

func (a *ClientController) resetTrafficByEmail(c *gin.Context) {
	err := a.inboundService.ResetClientTrafficByEmail(c.Param("email"))
	if err == nil {
		a.xrayService.SetToNeedRestart()
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), err)
}

func (a *ClientController) list(c *gin.Context) {
	rows, err := a.clientService.List()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *ClientController) get(c *gin.Context) {
	email := c.Param("email")
	rec, err := a.clientService.GetRecordByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	inboundIds, err := a.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var usedTraffic int64
	if traffic, err := a.inboundService.GetClientTrafficByEmail(email); err == nil && traffic != nil {
		usedTraffic = traffic.Up + traffic.Down
	}
	jsonObj(c, gin.H{"client": rec, "inboundIds": inboundIds, "usedTraffic": usedTraffic}, nil)
}

func (a *ClientController) getTrafficByEmail(c *gin.Context) {
	traffic, err := a.inboundService.GetClientTrafficByEmail(c.Param("email"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, traffic, nil)
}

type trafficUpdateRequest struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func (a *ClientController) updateTrafficByEmail(c *gin.Context) {
	var req trafficUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	err := a.inboundService.UpdateClientTrafficByEmail(c.Param("email"), req.Upload, req.Download)
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), err)
}

func (a *ClientController) getIps(c *gin.Context) {
	ips, err := a.inboundService.GetInboundClientIps(c.Param("email"))
	if err != nil || ips == "" {
		jsonObj(c, "No IP Record", nil)
		return
	}
	type ipWithTimestamp struct {
		IP        string `json:"ip"`
		Timestamp int64  `json:"timestamp"`
	}
	var ipsWithTime []ipWithTimestamp
	if err := json.Unmarshal([]byte(ips), &ipsWithTime); err == nil && len(ipsWithTime) > 0 {
		formatted := make([]string, 0, len(ipsWithTime))
		for _, item := range ipsWithTime {
			if item.IP == "" {
				continue
			}
			if item.Timestamp > 0 {
				ts := time.Unix(item.Timestamp, 0).Local().Format("2006-01-02 15:04:05")
				formatted = append(formatted, fmt.Sprintf("%s (%s)", item.IP, ts))
				continue
			}
			formatted = append(formatted, item.IP)
		}
		jsonObj(c, formatted, nil)
		return
	}
	var oldIps []string
	if err := json.Unmarshal([]byte(ips), &oldIps); err == nil && len(oldIps) > 0 {
		jsonObj(c, oldIps, nil)
		return
	}
	jsonObj(c, ips, nil)
}

func (a *ClientController) clearIps(c *gin.Context) {
	err := a.inboundService.ClearClientIps(c.Param("email"))
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), err)
}

func (a *ClientController) onlines(c *gin.Context) {
	jsonObj(c, a.inboundService.GetOnlineClients(), nil)
}

func (a *ClientController) lastOnline(c *gin.Context) {
	data, err := a.inboundService.GetClientsLastOnline()
	jsonObj(c, data, err)
}
