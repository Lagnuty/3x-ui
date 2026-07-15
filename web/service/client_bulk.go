package service

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/gorm"
)

func (s *ClientService) BulkDelete(inboundSvc *InboundService, emails []string, keepTraffic bool) (*BulkClientResult, bool, error) {
	result := &BulkClientResult{Skipped: []string{}}
	needRestart := false
	for _, email := range emails {
		record, err := s.GetRecordByEmail(email)
		if err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		inboundIds, err := s.GetInboundIdsForRecord(record.Id)
		if err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		if len(inboundIds) == 0 {
			if err := s.deleteOrphanClientRecord(inboundSvc, record, keepTraffic); err != nil {
				result.Skipped = append(result.Skipped, email)
				continue
			}
			result.Affected++
			continue
		}
		if _, err := s.deleteClientEverywhere(inboundSvc, email, inboundIds, keepTraffic); err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		result.Affected++
		needRestart = true
	}
	return result, needRestart, nil
}

func (s *ClientService) deleteOrphanClientRecord(inboundSvc *InboundService, record *model.ClientRecord, keepTraffic bool) error {
	db := database.GetDB()
	tx := db.Begin()
	if err := tx.Where("client_id = ?", record.Id).Delete(&model.ClientInbound{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if !keepTraffic {
		if err := inboundSvc.DelClientStat(tx, record.Email); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := inboundSvc.DelClientIPs(tx, record.Email); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(record).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *ClientService) BulkAdjust(inboundSvc *InboundService, emails []string, addDays int, addBytes int64) (*BulkClientResult, bool, error) {
	result := &BulkClientResult{Skipped: []string{}}
	nowMs := time.Now().Unix() * 1000
	for _, email := range emails {
		if err := s.adjustClientEverywhere(email, addDays, addBytes, nowMs); err != nil {
			result.Skipped = append(result.Skipped, email)
			continue
		}
		result.Affected++
	}
	return result, result.Affected > 0, nil
}

func (s *ClientService) BulkSetSpeedLimits(upMbps int64, downMbps int64) (*BulkClientResult, bool, error) {
	result := &BulkClientResult{Skipped: []string{}}
	if upMbps < 0 {
		upMbps = 0
	}
	if downMbps < 0 {
		downMbps = 0
	}

	db := database.GetDB()
	tx := db.Begin()

	var records []model.ClientRecord
	if err := tx.Model(model.ClientRecord{}).Find(&records).Error; err != nil {
		tx.Rollback()
		return result, false, err
	}

	nowMs := time.Now().Unix() * 1000
	for i := range records {
		records[i].SpeedLimitUpMbps = upMbps
		records[i].SpeedLimitDownMbps = downMbps
		records[i].UpdatedAt = nowMs
		if err := tx.Save(&records[i]).Error; err != nil {
			tx.Rollback()
			return result, false, err
		}
		result.Affected++
	}

	var inbounds []model.Inbound
	if err := tx.Find(&inbounds).Error; err != nil {
		tx.Rollback()
		return result, false, err
	}

	recordsByEmail := make(map[string]*model.ClientRecord, len(records))
	for i := range records {
		recordsByEmail[records[i].Email] = &records[i]
	}
	for i := range inbounds {
		changed, err := patchClientSpeedLimitsInSettings(&inbounds[i], recordsByEmail)
		if err != nil {
			tx.Rollback()
			return result, false, err
		}
		if !changed {
			continue
		}
		if err := tx.Save(&inbounds[i]).Error; err != nil {
			tx.Rollback()
			return result, false, err
		}
		if err := syncInboundClientsFromSettings(tx, &inbounds[i]); err != nil {
			tx.Rollback()
			return result, false, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return result, false, err
	}
	return result, result.Affected > 0, nil
}

func (s *ClientService) deleteClientEverywhere(inboundSvc *InboundService, email string, inboundIds []int, keepTraffic bool) (bool, error) {
	db := database.GetDB()
	tx := db.Begin()
	for _, inboundId := range inboundIds {
		inbound, err := inboundSvc.GetInbound(inboundId)
		if err != nil {
			tx.Rollback()
			return false, err
		}
		if err := removeClientFromInboundSettingsAllowEmpty(tx, inbound, email); err != nil {
			tx.Rollback()
			return false, err
		}
	}
	if !keepTraffic {
		if err := inboundSvc.DelClientStat(tx, email); err != nil {
			tx.Rollback()
			return false, err
		}
	}
	if err := inboundSvc.DelClientIPs(tx, email); err != nil {
		tx.Rollback()
		return false, err
	}
	if err := tx.Where("email = ?", email).Delete(&model.ClientRecord{}).Error; err != nil {
		tx.Rollback()
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *ClientService) adjustClientEverywhere(email string, addDays int, addBytes int64, nowMs int64) error {
	db := database.GetDB()
	tx := db.Begin()

	var record model.ClientRecord
	if err := tx.Where("email = ?", email).First(&record).Error; err != nil {
		tx.Rollback()
		return err
	}
	if addBytes != 0 {
		record.TotalGB += addBytes
		if record.TotalGB < 0 {
			record.TotalGB = 0
		}
	}
	if addDays != 0 {
		delta := int64(addDays) * 24 * 60 * 60 * 1000
		if record.ExpiryTime <= 0 || record.ExpiryTime < nowMs {
			record.ExpiryTime = nowMs + delta
		} else {
			record.ExpiryTime += delta
		}
		if record.ExpiryTime < 0 {
			record.ExpiryTime = 0
		}
	}
	record.UpdatedAt = nowMs
	if err := tx.Save(&record).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]any{"total": record.TotalGB, "expiry_time": record.ExpiryTime}).Error; err != nil {
		tx.Rollback()
		return err
	}

	var inbounds []model.Inbound
	if err := tx.Find(&inbounds).Error; err != nil {
		tx.Rollback()
		return err
	}
	for i := range inbounds {
		changed, err := patchClientRecordInSettings(&inbounds[i], &record)
		if err != nil {
			tx.Rollback()
			return err
		}
		if changed {
			if err := tx.Save(&inbounds[i]).Error; err != nil {
				tx.Rollback()
				return err
			}
			if err := syncInboundClientsFromSettings(tx, &inbounds[i]); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}

func removeClientFromInboundSettingsAllowEmpty(tx *gorm.DB, inbound *model.Inbound, email string) error {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return err
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return nil
	}
	next := make([]any, 0, len(rawClients))
	for _, raw := range rawClients {
		row, ok := raw.(map[string]any)
		if !ok {
			next = append(next, raw)
			continue
		}
		if rowEmail, _ := row["email"].(string); rowEmail == email {
			continue
		}
		next = append(next, raw)
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

func patchClientRecordInSettings(inbound *model.Inbound, record *model.ClientRecord) (bool, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false, err
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return false, nil
	}
	changed := false
	for _, raw := range rawClients {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if rowEmail, _ := row["email"].(string); rowEmail != record.Email {
			continue
		}
		row["totalGB"] = record.TotalGB
		row["expiryTime"] = record.ExpiryTime
		row["speedLimitUpMbps"] = record.SpeedLimitUpMbps
		row["speedLimitDownMbps"] = record.SpeedLimitDownMbps
		row["updated_at"] = record.UpdatedAt
		changed = true
	}
	if !changed {
		return false, nil
	}
	settings["clients"] = rawClients
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(b)
	return true, nil
}

func patchClientSpeedLimitsInSettings(inbound *model.Inbound, recordsByEmail map[string]*model.ClientRecord) (bool, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return false, err
	}
	rawClients, ok := settings["clients"].([]any)
	if !ok {
		return false, nil
	}
	changed := false
	for _, raw := range rawClients {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rowEmail, _ := row["email"].(string)
		record := recordsByEmail[rowEmail]
		if record == nil {
			continue
		}
		row["speedLimitUpMbps"] = record.SpeedLimitUpMbps
		row["speedLimitDownMbps"] = record.SpeedLimitDownMbps
		row["updated_at"] = record.UpdatedAt
		changed = true
	}
	if !changed {
		return false, nil
	}
	settings["clients"] = rawClients
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	inbound.Settings = string(b)
	return true, nil
}
