package service

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

type BridgeService struct {
	settingService     SettingService
	outboundSubService OutboundSubscriptionService
}

func (s *BridgeService) List() ([]*model.BridgeRoute, error) {
	var rows []*model.BridgeRoute
	err := database.GetDB().Model(model.BridgeRoute{}).Order("id asc").Find(&rows).Error
	return rows, err
}

func (s *BridgeService) Get(id int) (*model.BridgeRoute, error) {
	row := &model.BridgeRoute{}
	err := database.GetDB().Model(model.BridgeRoute{}).Where("id = ?", id).First(row).Error
	return row, err
}

func (s *BridgeService) Create(row *model.BridgeRoute) error {
	if err := s.normalize(row); err != nil {
		return err
	}
	return database.GetDB().Create(row).Error
}

func (s *BridgeService) Update(id int, row *model.BridgeRoute) error {
	if err := s.normalize(row); err != nil {
		return err
	}
	updates := map[string]any{
		"name":         row.Name,
		"remark":       row.Remark,
		"enable":       row.Enable,
		"outbound_tag": row.OutboundTag,
	}
	if b, err := json.Marshal(row.ClientEmails); err == nil {
		updates["client_emails"] = string(b)
	}
	return database.GetDB().Model(model.BridgeRoute{}).Where("id = ?", id).Updates(updates).Error
}

func (s *BridgeService) Delete(id int) error {
	return database.GetDB().Delete(&model.BridgeRoute{}, id).Error
}

func (s *BridgeService) SetEnable(id int, enable bool) error {
	return database.GetDB().Model(model.BridgeRoute{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *BridgeService) EnabledRoutes() ([]*model.BridgeRoute, error) {
	var rows []*model.BridgeRoute
	err := database.GetDB().
		Model(model.BridgeRoute{}).
		Where("enable = ? AND outbound_tag <> ''", true).
		Order("id asc").
		Find(&rows).Error
	return rows, err
}

func (s *BridgeService) OutboundTags() ([]string, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}
	cfg := &xray.Config{}
	if err := json.Unmarshal([]byte(templateConfig), cfg); err != nil {
		return nil, err
	}
	tags := map[string]struct{}{}
	addTags := func(rows []any) {
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			tag, _ := row["tag"].(string)
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags[tag] = struct{}{}
			}
		}
	}
	var manual []any
	if len(cfg.OutboundConfigs) > 0 {
		if err := json.Unmarshal(cfg.OutboundConfigs, &manual); err != nil {
			return nil, err
		}
	}
	if len(cfg.RouterConfig) > 0 {
		var routing map[string]any
		if err := json.Unmarshal(cfg.RouterConfig, &routing); err != nil {
			return nil, err
		}
		if balancers, ok := routing["balancers"].([]any); ok {
			for _, raw := range balancers {
				row, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				tag, _ := row["tag"].(string)
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags[tag] = struct{}{}
				}
			}
		}
	}
	prepend, appendList, err := s.outboundSubService.ActiveOutboundsSplit()
	if err != nil {
		return nil, err
	}
	addTags(prepend)
	addTags(manual)
	addTags(appendList)

	result := make([]string, 0, len(tags))
	for tag := range tags {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result, nil
}

func (s *BridgeService) normalize(row *model.BridgeRoute) error {
	row.Name = strings.TrimSpace(row.Name)
	row.Remark = strings.TrimSpace(row.Remark)
	row.OutboundTag = strings.TrimSpace(row.OutboundTag)
	if row.Name == "" {
		return common.NewError("bridge name is required")
	}
	if row.OutboundTag == "" {
		return common.NewError("outbound tag is required")
	}
	row.ClientEmails = normalizeEmailList(row.ClientEmails)
	if len(row.ClientEmails) == 0 {
		return common.NewError("select at least one client")
	}
	return nil
}

func normalizeEmailList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
