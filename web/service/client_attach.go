package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

type BulkClientResult struct {
	Affected int      `json:"affected"`
	Skipped  []string `json:"skipped"`
}

func (s *ClientService) AttachByEmail(inboundSvc *InboundService, email string, inboundIds []int) (bool, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return false, common.NewError("client email is required")
	}
	if len(inboundIds) == 0 {
		return false, common.NewError("at least one inbound is required")
	}

	record, err := s.GetRecordByEmail(email)
	if err != nil {
		return false, err
	}
	client := record.ToClient()
	now := time.Now().Unix() * 1000
	if client.CreatedAt == 0 {
		client.CreatedAt = now
	}
	client.UpdatedAt = now

	db := database.GetDB()
	tx := db.Begin()
	for _, inboundId := range inboundIds {
		inbound, err := inboundSvc.GetInbound(inboundId)
		if err != nil {
			tx.Rollback()
			return false, err
		}
		if err := validateClientForInbound(client, inbound); err != nil {
			tx.Rollback()
			return false, err
		}
		added, err := addClientToInboundSettings(tx, inbound, client)
		if err != nil {
			tx.Rollback()
			return false, err
		}
		if !added {
			continue
		}
		var trafficCount int64
		if err := tx.Model(xray.ClientTraffic{}).Where("email = ?", email).Count(&trafficCount).Error; err != nil {
			tx.Rollback()
			return false, err
		}
		if trafficCount == 0 {
			if err := inboundSvc.AddClientStat(tx, inbound.Id, client); err != nil {
				tx.Rollback()
				return false, err
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *ClientService) DetachByEmailMany(inboundSvc *InboundService, email string, inboundIds []int) (bool, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return false, common.NewError("client email is required")
	}
	if len(inboundIds) == 0 {
		return false, common.NewError("at least one inbound is required")
	}

	db := database.GetDB()
	tx := db.Begin()
	for _, inboundId := range inboundIds {
		inbound, err := inboundSvc.GetInbound(inboundId)
		if err != nil {
			tx.Rollback()
			return false, err
		}
		if err := removeClientFromInboundSettings(tx, inbound, email); err != nil {
			tx.Rollback()
			return false, err
		}
	}

	var remaining int64
	if err := tx.Model(model.ClientInbound{}).Where("client_id IN (SELECT id FROM clients WHERE email = ?)", email).Count(&remaining).Error; err != nil {
		tx.Rollback()
		return false, err
	}
	if remaining == 0 {
		if err := inboundSvc.DelClientStat(tx, email); err != nil {
			tx.Rollback()
			return false, err
		}
		if err := inboundSvc.DelClientIPs(tx, email); err != nil {
			tx.Rollback()
			return false, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *ClientService) BulkAttach(inboundSvc *InboundService, emails []string, inboundIds []int) (*BulkClientResult, bool, error) {
	result := &BulkClientResult{Skipped: []string{}}
	needRestart := false
	for _, email := range emails {
		nr, err := s.AttachByEmail(inboundSvc, email, inboundIds)
		if err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		result.Affected++
		if nr {
			needRestart = true
		}
	}
	return result, needRestart, nil
}

func (s *ClientService) BulkDetach(inboundSvc *InboundService, emails []string, inboundIds []int) (*BulkClientResult, bool, error) {
	result := &BulkClientResult{Skipped: []string{}}
	needRestart := false
	for _, email := range emails {
		nr, err := s.DetachByEmailMany(inboundSvc, email, inboundIds)
		if err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		result.Affected++
		if nr {
			needRestart = true
		}
	}
	return result, needRestart, nil
}

func (s *ClientService) BulkResetTraffic(inboundSvc *InboundService, emails []string) (int, error) {
	affected := 0
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		if err := inboundSvc.ResetClientTrafficByEmail(email); err != nil {
			return affected, err
		}
		affected++
	}
	return affected, nil
}

func validateClientForInbound(client *model.Client, inbound *model.Inbound) error {
	switch inbound.Protocol {
	case "trojan":
		if client.Password == "" {
			return common.NewError("empty client password for trojan inbound")
		}
	case "shadowsocks":
		if client.Email == "" {
			return common.NewError("empty client email for shadowsocks inbound")
		}
	default:
		if client.ID == "" {
			return common.NewError("empty client id")
		}
	}
	return nil
}

func addClientToInboundSettings(tx *gorm.DB, inbound *model.Inbound, client *model.Client) (bool, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false, err
	}
	rawClients, _ := settings["clients"].([]any)
	for _, raw := range rawClients {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if rowEmail, _ := row["email"].(string); rowEmail == client.Email {
			return false, nil
		}
	}
	rawClients = append(rawClients, client)
	settings["clients"] = rawClients
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(b)
	if err := tx.Save(inbound).Error; err != nil {
		return false, err
	}
	if err := syncInboundClientsFromSettings(tx, inbound); err != nil {
		return false, err
	}
	return true, nil
}

func removeClientFromInboundSettings(tx *gorm.DB, inbound *model.Inbound, email string) error {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return common.NewError("invalid clients format in inbound settings")
	}
	next := make([]any, 0, len(rawClients))
	found := false
	for _, raw := range rawClients {
		row, ok := raw.(map[string]any)
		if !ok {
			next = append(next, raw)
			continue
		}
		if rowEmail, _ := row["email"].(string); rowEmail == email {
			found = true
			continue
		}
		next = append(next, raw)
	}
	if !found {
		return common.NewError("client not found in inbound:", email)
	}
	if len(next) == 0 {
		return common.NewError("no client remained in Inbound")
	}
	settings["clients"] = next
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	inbound.Settings = string(b)
	if err := tx.Save(inbound).Error; err != nil {
		return err
	}
	return syncInboundClientsFromSettings(tx, inbound)
}
