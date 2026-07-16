package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

type BridgeController struct {
	bridgeService service.BridgeService
	xrayService   service.XrayService
}

func NewBridgeController(g *gin.RouterGroup) *BridgeController {
	a := &BridgeController{}
	a.initRouter(g)
	return a
}

func (a *BridgeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/options", a.options)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/del/:id", a.del)
	g.POST("/setEnable/:id", a.setEnable)
}

func (a *BridgeController) list(c *gin.Context) {
	rows, err := a.bridgeService.List()
	jsonObj(c, rows, err)
}

func (a *BridgeController) options(c *gin.Context) {
	tags, err := a.bridgeService.OutboundTags()
	jsonObj(c, gin.H{"outboundTags": tags}, err)
}

func (a *BridgeController) add(c *gin.Context) {
	row := &model.BridgeRoute{Enable: true}
	if err := c.ShouldBind(row); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.bridgeService.Create(row); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.restartXray(c, row)
}

func (a *BridgeController) update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	row := &model.BridgeRoute{}
	if err := c.ShouldBind(row); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.bridgeService.Update(id, row); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	a.restartXray(c, row)
}

func (a *BridgeController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	if err := a.bridgeService.Delete(id); err != nil {
		jsonMsg(c, "Bridge deleted", err)
		return
	}
	a.restartXray(c, gin.H{"id": id})
}

func (a *BridgeController) setEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	body := struct {
		Enable bool `json:"enable" form:"enable"`
	}{}
	if err := c.ShouldBind(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.bridgeService.SetEnable(id, body.Enable); err != nil {
		jsonMsg(c, "Bridge updated", err)
		return
	}
	a.restartXray(c, gin.H{"id": id, "enable": body.Enable})
}

func (a *BridgeController) restartXray(c *gin.Context, obj any) {
	if err := a.xrayService.RestartXray(true); err != nil {
		a.xrayService.SetToNeedRestart()
		jsonObj(c, obj, err)
		return
	}
	jsonObj(c, obj, nil)
}
