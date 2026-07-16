package controller

import (
	"net/http"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController *InboundController
	serverController  *ServerController
	apiTokenService   service.ApiTokenService
	userService       service.UserService
	Tgbot             service.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

// checkAPIAuth is a middleware that returns 404 for unauthenticated API requests
// to hide the existence of API endpoints from unauthorized users
func (a *APIController) checkAPIAuth(c *gin.Context) {
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if apiToken, ok := a.apiTokenService.Match(token); ok {
			loginUser, err := a.userService.GetFirstUser()
			if apiToken.UserId > 0 {
				loginUser, err = a.userService.GetById(apiToken.UserId)
			}
			if err != nil || loginUser == nil || !loginUser.Enabled {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			session.SetLoginUser(c, loginUser)
			c.Set("audit_user", loginUser)
			c.Set("api_authed", true)
			c.Set("auth_source", "api")
			c.Set("api_token_id", apiToken.Id)
			c.Next()
			return
		}
	}

	loginUser := session.GetLoginUser(c)
	if loginUser == nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	freshUser, err := a.userService.GetById(loginUser.Id)
	if err != nil || !freshUser.CanUsePanel() {
		session.ClearSession(c)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	session.SetLoginUser(c, freshUser)
	c.Set("audit_user", freshUser)
	c.Next()
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)
	api.Use(a.auditActions)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = NewInboundController(inbounds)

	// Clients API
	clients := api.Group("/clients")
	NewClientController(clients)
	NewGroupController(clients)

	// Nodes API
	nodes := api.Group("/nodes")
	NewNodeController(nodes)

	// Client bridge routing API
	bridges := api.Group("/bridges")
	NewBridgeController(bridges)

	// Server API
	server := api.Group("/server")
	a.serverController = NewServerController(server)

	// Extra routes
	api.GET("/backuptotgbot", a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
