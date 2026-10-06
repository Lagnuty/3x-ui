package controller

import (
	"errors"
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

type OpenFluxController struct {
	BaseController
	openFluxService service.OpenFluxService
}

func NewOpenFluxController(g *gin.RouterGroup) *OpenFluxController {
	a := &OpenFluxController{}
	a.initRouter(g)
	return a
}

func (a *OpenFluxController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/install-info", a.installInfo)
	g.GET("/mobile-connections", a.mobileConnections)
	g.GET("/version", a.version)
	g.GET("/status/:id", a.status)
	g.GET("/unit/:id", a.unitPreview)
	g.POST("/add", a.add)
	g.POST("/install", a.install)
	g.POST("/update/:id", a.update)
	g.POST("/delete/:id", a.delete)
	g.POST("/apply/:id", a.apply)
	g.POST("/start/:id", a.start)
	g.POST("/stop/:id", a.stop)
	g.POST("/restart/:id", a.restart)
}

func mapOpenFluxErr(c *gin.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrOpenFluxNotFound):
		return errors.New(I18nWeb(c, "pages.openflux.errors.notFound"))
	case errors.Is(err, service.ErrOpenFluxNameRequired):
		return errors.New(I18nWeb(c, "pages.openflux.errors.nameRequired"))
	case errors.Is(err, service.ErrOpenFluxNameInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.nameInvalid"))
	case errors.Is(err, service.ErrOpenFluxServerIDInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.serverInvalid"))
	case errors.Is(err, service.ErrOpenFluxTransportInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.transportInvalid"))
	case errors.Is(err, service.ErrOpenFluxURLRequired):
		return errors.New(I18nWeb(c, "pages.openflux.errors.urlRequired"))
	case errors.Is(err, service.ErrOpenFluxURLInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.urlInvalid"))
	case errors.Is(err, service.ErrOpenFluxModeInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.modeInvalid"))
	case errors.Is(err, service.ErrOpenFluxCodecInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.codecInvalid"))
	case errors.Is(err, service.ErrOpenFluxDebugInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.debugInvalid"))
	case errors.Is(err, service.ErrOpenFluxKeyFileInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.keyFileInvalid"))
	case errors.Is(err, service.ErrOpenFluxUnsupportedOS):
		return errors.New(I18nWeb(c, "pages.openflux.errors.unsupportedOS"))
	case errors.Is(err, service.ErrOpenFluxBinaryUnavailable):
		return errors.New(I18nWeb(c, "pages.openflux.errors.binaryUnavailable"))
	case errors.Is(err, service.ErrOpenFluxSystemctlFailed):
		logger.Warning("openflux systemctl:", err)
		return errors.New(I18nWeb(c, "pages.openflux.errors.systemctl"))
	case errors.Is(err, service.ErrOpenFluxInstallFailed):
		logger.Warning("openflux install:", err)
		return errors.New(I18nWeb(c, "pages.openflux.errors.installFailed") + ": " + err.Error())
	case errors.Is(err, service.ErrOpenFluxRefInvalid):
		return errors.New(I18nWeb(c, "pages.openflux.errors.refInvalid"))
	default:
		return err
	}
}

func parseOpenFluxID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		jsonMsg(c, I18nWeb(c, "pages.openflux.errors.invalidId"), err)
		return 0, false
	}
	return id, true
}

func (a *OpenFluxController) list(c *gin.Context) {
	list, err := a.openFluxService.GetAll()
	jsonObj(c, list, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) mobileConnections(c *gin.Context) {
	list, err := a.openFluxService.MobileConnections()
	jsonObj(c, list, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) installInfo(c *gin.Context) {
	info, err := a.openFluxService.InstallInfo()
	jsonObj(c, info, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) version(c *gin.Context) {
	version, err := a.openFluxService.BinaryVersion()
	jsonObj(c, gin.H{"path": "/usr/local/bin/openflux", "version": version}, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) status(c *gin.Context) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	status, err := a.openFluxService.Status(id, 120)
	jsonObj(c, status, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) unitPreview(c *gin.Context) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	unit, err := a.openFluxService.UnitPreview(id)
	jsonObj(c, gin.H{"unit": unit}, mapOpenFluxErr(c, err))
}

func bindOpenFluxNode(c *gin.Context) (*model.OpenFluxNode, error) {
	var node model.OpenFluxNode
	if err := c.ShouldBind(&node); err != nil {
		return nil, err
	}
	return &node, nil
}

func (a *OpenFluxController) add(c *gin.Context) {
	node, err := bindOpenFluxNode(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.add"), err)
		return
	}
	err = a.openFluxService.Create(node)
	jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.add"), mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) install(c *gin.Context) {
	var req struct {
		Ref string `json:"ref" form:"ref"`
	}
	_ = c.ShouldBind(&req)
	result, err := a.openFluxService.InstallOrUpdate(req.Ref)
	jsonObj(c, result, mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) update(c *gin.Context) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	node, err := bindOpenFluxNode(c)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.update"), err)
		return
	}
	err = a.openFluxService.Update(id, node)
	jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.update"), mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) delete(c *gin.Context) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	err := a.openFluxService.Delete(id)
	jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.delete"), mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) apply(c *gin.Context) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	err := a.openFluxService.ApplyUnit(id)
	jsonMsg(c, I18nWeb(c, "pages.openflux.toasts.apply"), mapOpenFluxErr(c, err))
}

func (a *OpenFluxController) start(c *gin.Context) {
	a.control(c, "start")
}

func (a *OpenFluxController) stop(c *gin.Context) {
	a.control(c, "stop")
}

func (a *OpenFluxController) restart(c *gin.Context) {
	a.control(c, "restart")
}

func (a *OpenFluxController) control(c *gin.Context, action string) {
	id, ok := parseOpenFluxID(c)
	if !ok {
		return
	}
	err := a.openFluxService.Control(id, action)
	jsonMsg(c, I18nWeb(c, "pages.openflux.toasts."+action), mapOpenFluxErr(c, err))
}
