package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"

	"gorm.io/gorm"
)

func (s *ClientService) SyncInbound(tx *gorm.DB, inboundId int, clients []model.Client) error {
	if tx == nil {
		return nil
	}
	if err := tx.Where("inbound_id = ?", inboundId).Delete(&model.ClientInbound{}).Error; err != nil {
		return err
	}
	for _, client := range clients {
		if client.Email == "" {
			continue
		}
		record := client.ToRecord()
		var existing model.ClientRecord
		err := tx.Where("email = ?", client.Email).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(record).Error; err != nil {
				return err
			}
			existing = *record
		} else if err != nil {
			return err
		} else {
			if record.Group == "" {
				record.Group = existing.Group
			}
			if record.CreatedAt == 0 {
				record.CreatedAt = existing.CreatedAt
			}
			if record.Reverse == "" {
				record.Reverse = existing.Reverse
			}
			existing.SubID = record.SubID
			existing.UUID = record.UUID
			existing.Password = record.Password
			existing.Auth = record.Auth
			existing.Flow = record.Flow
			existing.Security = record.Security
			existing.Reverse = record.Reverse
			existing.LimitIP = record.LimitIP
			existing.TotalGB = record.TotalGB
			existing.ExpiryTime = record.ExpiryTime
			existing.Enable = record.Enable
			existing.TgID = record.TgID
			existing.Group = record.Group
			existing.Comment = record.Comment
			existing.Reset = record.Reset
			existing.CreatedAt = record.CreatedAt
			existing.UpdatedAt = record.UpdatedAt
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&model.ClientInbound{
			ClientId:     existing.Id,
			InboundId:    inboundId,
			FlowOverride: client.Flow,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func syncInboundClientsFromSettings(tx *gorm.DB, inbound *model.Inbound) error {
	var payload struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &payload); err != nil {
		return err
	}
	var clientService ClientService
	return clientService.SyncInbound(tx, inbound.Id, payload.Clients)
}

func (s *InboundService) SyncAllInboundClients() error {
	db := database.GetDB()
	var inbounds []model.Inbound
	if err := db.Find(&inbounds).Error; err != nil {
		return err
	}
	tx := db.Begin()
	for i := range inbounds {
		if err := syncInboundClientsFromSettings(tx, &inbounds[i]); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
