package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

type ClientWithAttachments struct {
	model.ClientRecord
	InboundIds []int               `json:"inboundIds"`
	Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
}

func (c ClientWithAttachments) MarshalJSON() ([]byte, error) {
	rec, err := json.Marshal(c.ClientRecord)
	if err != nil {
		return nil, err
	}
	extras := struct {
		InboundIds []int               `json:"inboundIds"`
		Traffic    *xray.ClientTraffic `json:"traffic,omitempty"`
	}{InboundIds: c.InboundIds, Traffic: c.Traffic}
	extra, err := json.Marshal(extras)
	if err != nil {
		return nil, err
	}
	if len(rec) < 2 || rec[len(rec)-1] != '}' || len(extra) <= 2 {
		return rec, nil
	}
	out := make([]byte, 0, len(rec)+len(extra))
	out = append(out, rec[:len(rec)-1]...)
	if len(rec) > 2 {
		out = append(out, ',')
	}
	out = append(out, extra[1:]...)
	return out, nil
}

func (s *ClientService) List() ([]ClientWithAttachments, error) {
	db := database.GetDB()
	records := []model.ClientRecord{}
	if err := db.Model(model.ClientRecord{}).Order("email asc").Find(&records).Error; err != nil {
		return nil, err
	}

	rows := make([]ClientWithAttachments, 0, len(records))
	for _, record := range records {
		inboundIds, err := s.GetInboundIdsForRecord(record.Id)
		if err != nil {
			return nil, err
		}
		var traffic xray.ClientTraffic
		var trafficPtr *xray.ClientTraffic
		if err := db.Model(xray.ClientTraffic{}).Where("email = ?", record.Email).First(&traffic).Error; err == nil {
			traffic.UUID = record.UUID
			traffic.SubId = record.SubID
			traffic.Enable = record.Enable
			trafficPtr = &traffic
		}
		rows = append(rows, ClientWithAttachments{
			ClientRecord: record,
			InboundIds:   inboundIds,
			Traffic:      trafficPtr,
		})
	}
	return rows, nil
}

func (s *ClientService) GetRecordByEmail(email string) (*model.ClientRecord, error) {
	record := &model.ClientRecord{}
	err := database.GetDB().Model(model.ClientRecord{}).Where("email = ?", email).First(record).Error
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *ClientService) GetInboundIdsForRecord(clientId int) ([]int, error) {
	var ids []int
	err := database.GetDB().
		Model(model.ClientInbound{}).
		Where("client_id = ?", clientId).
		Order("inbound_id asc").
		Pluck("inbound_id", &ids).Error
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []int{}
	}
	return ids, nil
}
