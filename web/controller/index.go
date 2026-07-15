package controller

import (
	"net/http"
	"text/template"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LoginForm represents the login request structure.
type LoginForm struct {
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	TwoFactorCode string `json:"twoFactorCode" form:"twoFactorCode"`
}

// IndexController handles the main index and login-related routes.
type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	auditService   service.AuditService
	tgbot          service.Tgbot
}

// NewIndexController creates a new IndexController and initializes its routes.
func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for index, login, logout, and two-factor authentication.
func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.GET("/logout", a.logout)

	g.POST("/login", a.login)
	g.POST("/getTwoFactorEnable", a.getTwoFactorEnable)
}

// index handles the root route, redirecting logged-in users to the panel or showing the login page.
func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "panel/")
		return
	}
	html(c, "login.html", "pages.login.title", nil)
}

// login handles user authentication and session creation.
func (a *IndexController) login(c *gin.Context) {
	var form LoginForm

	if err := c.ShouldBind(&form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}

	remoteIP := getRemoteIp(c)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	safeUser := template.HTMLEscapeString(form.Username)
	if blockedUntil, ok := defaultLoginLimiter.allow(remoteIP, form.Username); !ok {
		logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, "too many failed attempts", blockedUntil.Format(time.RFC3339))
		a.tgbot.UserLoginNotify(safeUser, "", remoteIP, timeStr, 0)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	}

	user := a.userService.CheckUser(form.Username, form.Password, form.TwoFactorCode)

	if user == nil {
		if blockedUntil, blocked := defaultLoginLimiter.registerFailure(remoteIP, form.Username); blocked {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q, blocked_until=%s", safeUser, remoteIP, "invalid credentials", blockedUntil.Format(time.RFC3339))
		} else {
			logger.Warningf("failed login: username=%q, IP=%q, reason=%q", safeUser, remoteIP, "invalid credentials")
		}
		a.tgbot.UserLoginNotify(safeUser, "", remoteIP, timeStr, 0)
		_ = a.auditService.Record(&model.AuditLog{
			Username:  safeUser,
			Source:    "panel",
			Action:    "login_failed",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    http.StatusOK,
			IP:        remoteIP,
			UserAgent: c.Request.UserAgent(),
			Detail:    "invalid credentials or non-panel role",
		})
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	}

	defaultLoginLimiter.registerSuccess(remoteIP, form.Username)
	logger.Infof("%s logged in successfully, Ip Address: %s\n", safeUser, remoteIP)
	a.tgbot.UserLoginNotify(safeUser, ``, remoteIP, timeStr, 1)

	sessionMaxAge, err := a.settingService.GetSessionMaxAge()
	if err != nil {
		logger.Warning("Unable to get session's max age from DB")
	}

	session.SetMaxAge(c, sessionMaxAge*60)
	session.SetLoginUser(c, user)
	if err := sessions.Default(c).Save(); err != nil {
		logger.Warning("Unable to save session: ", err)
		return
	}

	logger.Infof("%s logged in successfully", safeUser)
	_ = a.auditService.Record(&model.AuditLog{
		UserId:    user.Id,
		Username:  user.Username,
		Role:      user.NormalizedRole(),
		Source:    "panel",
		Action:    "login_success",
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Status:    http.StatusOK,
		IP:        remoteIP,
		UserAgent: c.Request.UserAgent(),
	})
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)
}

// logout handles user logout by clearing the session and redirecting to the login page.
func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Infof("%s logged out successfully", user.Username)
		_ = a.auditService.Record(&model.AuditLog{
			UserId:    user.Id,
			Username:  user.Username,
			Role:      user.NormalizedRole(),
			Source:    "panel",
			Action:    "logout",
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Status:    http.StatusTemporaryRedirect,
			IP:        getRemoteIp(c),
			UserAgent: c.Request.UserAgent(),
		})
	}
	session.ClearSession(c)
	if err := sessions.Default(c).Save(); err != nil {
		logger.Warning("Unable to save session after clearing:", err)
	}
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}

// getTwoFactorEnable retrieves the current status of two-factor authentication.
func (a *IndexController) getTwoFactorEnable(c *gin.Context) {
	status, err := a.settingService.GetTwoFactorEnable()
	if err == nil {
		jsonObj(c, status, nil)
	}
}
