// Package controller provides HTTP request handlers and controllers for the 3x-ui web management panel.
// It handles routing, authentication, and API endpoints for managing Xray inbounds, settings, and more.
package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/locale"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// BaseController provides common functionality for all controllers, including authentication checks.
type BaseController struct{}

// checkLogin is a middleware that verifies user authentication and handles unauthorized access.
func (a *BaseController) checkLogin(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		freshUser, err := (&service.UserService{}).GetById(user.Id)
		if err == nil && freshUser.CanUsePanel() {
			session.SetLoginUser(c, freshUser)
			c.Set("audit_user", freshUser)
			c.Next()
			return
		}
		session.ClearSession(c)
	}
	if isAjax(c) {
		pureJsonMsg(c, http.StatusUnauthorized, false, I18nWeb(c, "pages.login.loginAgain"))
	} else {
		c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
	}
	c.Abort()
}

func (a *BaseController) auditActions(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		c.Next()
		return
	}
	c.Next()
	_ = (&service.AuditService{}).RecordRequest(c, "", "")
}

// I18nWeb retrieves an internationalized message for the web interface based on the current locale.
func I18nWeb(c *gin.Context, name string, params ...string) string {
	anyfunc, funcExists := c.Get("I18n")
	if !funcExists {
		logger.Warning("I18n function not exists in gin context!")
		return ""
	}
	i18nFunc, _ := anyfunc.(func(i18nType locale.I18nType, key string, keyParams ...string) string)
	msg := i18nFunc(locale.Web, name, params...)
	return msg
}
