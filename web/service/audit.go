package service

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"

	"github.com/gin-gonic/gin"
)

const maxAuditDetailLen = 512

type AuditService struct{}

func (s *AuditService) Record(entry *model.AuditLog) error {
	if entry == nil {
		return nil
	}
	if len(entry.Detail) > maxAuditDetailLen {
		entry.Detail = entry.Detail[:maxAuditDetailLen]
	}
	return database.GetDB().Create(entry).Error
}

func (s *AuditService) RecordRequest(c *gin.Context, action string, detail string) error {
	if c == nil {
		return nil
	}
	user, _ := c.Get("audit_user")
	loginUser, _ := user.(*model.User)
	if loginUser == nil {
		return nil
	}

	source, _ := c.Get("auth_source")
	sourceStr, _ := source.(string)
	if sourceStr == "" {
		sourceStr = "panel"
	}
	tokenId := 0
	if tokenIdAny, ok := c.Get("api_token_id"); ok {
		if v, ok := tokenIdAny.(int); ok {
			tokenId = v
		}
	}
	if action == "" {
		action = strings.TrimPrefix(c.FullPath(), "/")
		if action == "" {
			action = c.Request.URL.Path
		}
	}

	return s.Record(&model.AuditLog{
		UserId:    loginUser.Id,
		Username:  loginUser.Username,
		Role:      loginUser.NormalizedRole(),
		Source:    sourceStr,
		TokenId:   tokenId,
		Action:    action,
		Method:    c.Request.Method,
		Path:      c.Request.URL.Path,
		Status:    c.Writer.Status(),
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Detail:    detail,
	})
}

func (s *AuditService) List(limit int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows := []model.AuditLog{}
	if err := database.GetDB().
		Model(model.AuditLog{}).
		Order("id desc").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
