package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/link"
)

type OutboundSubscriptionService struct{}

var defaultOutboundPrefixRe = regexp.MustCompile(`^sub(\d+)-$`)

func (s *OutboundSubscriptionService) List() ([]*model.OutboundSubscription, error) {
	var subs []*model.OutboundSubscription
	if err := database.GetDB().Model(&model.OutboundSubscription{}).Order("priority asc, id asc").Find(&subs).Error; err != nil {
		return nil, err
	}
	for _, sub := range subs {
		sub.OutboundCount = countStoredOutbounds(sub.LastFetchedOutbounds)
		sub.LastFetchedOutbounds = ""
		sub.LinkIdentities = ""
	}
	return subs, nil
}

func (s *OutboundSubscriptionService) Get(id int) (*model.OutboundSubscription, error) {
	var sub model.OutboundSubscription
	if err := database.GetDB().First(&sub, id).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *OutboundSubscriptionService) Create(remark, rawURL, tagPrefix string, enabled bool, updateInterval int, allowPrivate, prepend bool) (*model.OutboundSubscription, error) {
	cleanURL, err := sanitizeOutboundSubURL(rawURL, allowPrivate)
	if err != nil {
		return nil, common.NewError("invalid subscription URL:", err)
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	prefix := strings.TrimSpace(tagPrefix)
	if prefix == "" {
		prefix = s.nextDefaultSubPrefix(0)
	}
	var count int64
	_ = database.GetDB().Model(&model.OutboundSubscription{}).Count(&count).Error
	sub := &model.OutboundSubscription{
		Remark:         strings.TrimSpace(remark),
		Url:            cleanURL,
		Enabled:        enabled,
		AllowPrivate:   allowPrivate,
		TagPrefix:      prefix,
		UpdateInterval: updateInterval,
		Priority:       int(count),
		Prepend:        prepend,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *OutboundSubscriptionService) Update(id int, remark, rawURL, tagPrefix string, enabled bool, updateInterval int, allowPrivate, prepend bool) error {
	sub, err := s.Get(id)
	if err != nil {
		return err
	}
	cleanURL, err := sanitizeOutboundSubURL(rawURL, allowPrivate)
	if err != nil {
		return common.NewError("invalid subscription URL:", err)
	}
	if updateInterval <= 0 {
		updateInterval = 600
	}
	prefix := strings.TrimSpace(tagPrefix)
	if prefix == "" {
		prefix = s.nextDefaultSubPrefix(sub.Id)
	}
	sub.Remark = strings.TrimSpace(remark)
	sub.Url = cleanURL
	sub.Enabled = enabled
	sub.AllowPrivate = allowPrivate
	sub.TagPrefix = prefix
	sub.UpdateInterval = updateInterval
	sub.Prepend = prepend
	return database.GetDB().Save(sub).Error
}

func (s *OutboundSubscriptionService) Delete(id int) error {
	return database.GetDB().Delete(&model.OutboundSubscription{}, id).Error
}

func (s *OutboundSubscriptionService) Refresh(id int) ([]any, error) {
	sub, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	outbounds, err := fetchOutboundSubJSON(sub.Url, sub.TagPrefix)
	now := time.Now().Unix()
	if err != nil {
		sub.LastError = err.Error()
		_ = database.GetDB().Save(sub).Error
		return nil, err
	}
	raw, err := json.Marshal(outbounds)
	if err != nil {
		return nil, err
	}
	sub.LastFetchedOutbounds = string(raw)
	sub.LastUpdated = now
	sub.LastError = ""
	if err := database.GetDB().Save(sub).Error; err != nil {
		return nil, err
	}
	return outbounds, nil
}

func (s *OutboundSubscriptionService) RefreshAllEnabled() (int, error) {
	var subs []*model.OutboundSubscription
	if err := database.GetDB().Where("enabled = ?", true).Find(&subs).Error; err != nil {
		return 0, err
	}
	now := time.Now().Unix()
	refreshed := 0
	for _, sub := range subs {
		if sub.LastUpdated != 0 && sub.LastUpdated+int64(sub.UpdateInterval) > now {
			continue
		}
		if _, err := s.Refresh(sub.Id); err == nil {
			refreshed++
		}
	}
	return refreshed, nil
}

func (s *OutboundSubscriptionService) ActiveOutboundsSplit() (prepend []any, appendList []any, err error) {
	var subs []*model.OutboundSubscription
	if err := database.GetDB().
		Model(&model.OutboundSubscription{}).
		Where("enabled = ?", true).
		Order("priority asc, id asc").
		Find(&subs).Error; err != nil {
		return nil, nil, err
	}
	for _, sub := range subs {
		if strings.TrimSpace(sub.LastFetchedOutbounds) == "" {
			continue
		}
		var rows []any
		if err := json.Unmarshal([]byte(sub.LastFetchedOutbounds), &rows); err != nil {
			continue
		}
		if sub.Prepend {
			prepend = append(prepend, rows...)
			continue
		}
		appendList = append(appendList, rows...)
	}
	return prepend, appendList, nil
}

func (s *OutboundSubscriptionService) Move(id int, up bool) error {
	subs, err := s.List()
	if err != nil {
		return err
	}
	index := -1
	for i, sub := range subs {
		if sub.Id == id {
			index = i
			break
		}
	}
	if index == -1 {
		return common.NewError("subscription not found")
	}
	swap := index + 1
	if up {
		swap = index - 1
	}
	if swap < 0 || swap >= len(subs) {
		return nil
	}
	subs[index].Priority, subs[swap].Priority = subs[swap].Priority, subs[index].Priority
	db := database.GetDB()
	if err := db.Model(&model.OutboundSubscription{}).Where("id = ?", subs[index].Id).Update("priority", subs[index].Priority).Error; err != nil {
		return err
	}
	return db.Model(&model.OutboundSubscription{}).Where("id = ?", subs[swap].Id).Update("priority", subs[swap].Priority).Error
}

func (s *OutboundSubscriptionService) nextDefaultSubPrefix(excludeId int) string {
	var subs []*model.OutboundSubscription
	_ = database.GetDB().Find(&subs).Error
	used := map[int]bool{}
	for _, sub := range subs {
		if sub.Id == excludeId {
			continue
		}
		if sub.TagPrefix == "" {
			used[sub.Id] = true
			continue
		}
		if m := defaultOutboundPrefixRe.FindStringSubmatch(sub.TagPrefix); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				used[n] = true
			}
		}
	}
	n := 1
	for used[n] {
		n++
	}
	return fmt.Sprintf("sub%d-", n)
}

func countStoredOutbounds(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	var arr []any
	if json.Unmarshal([]byte(raw), &arr) != nil {
		return 0
	}
	return len(arr)
}

func fetchOutboundSubJSON(rawURL, tagPrefix string) ([]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "3x-ui/2.8.11+ outbound-subscription")
	client := (&SettingService{}).NewProxiedHTTPClient(20 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, common.NewError("subscription fetch failed:", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	outbounds, err := parseOutboundSubJSON(body)
	if err != nil {
		return nil, err
	}
	prefixOutboundTags(outbounds, strings.TrimSpace(tagPrefix))
	return outbounds, nil
}

func parseOutboundSubJSON(body []byte) ([]any, error) {
	var arr []any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return parseOutboundShareLinks(body)
	}
	raw, ok := obj["outbounds"].([]any)
	if !ok {
		return parseOutboundShareLinks(body)
	}
	return raw, nil
}

func parseOutboundShareLinks(body []byte) ([]any, error) {
	parsed, _, err := link.ParseSubscriptionBody(body)
	if err != nil {
		return nil, common.NewError("subscription must be JSON outbounds or supported share links")
	}
	out := make([]any, 0, len(parsed))
	for _, row := range parsed {
		out = append(out, map[string]any(row))
	}
	if len(out) == 0 {
		return nil, common.NewError("subscription contains no supported outbound links")
	}
	return out, nil
}

func prefixOutboundTags(outbounds []any, prefix string) {
	if prefix == "" {
		return
	}
	for i, raw := range outbounds {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := row["tag"].(string)
		if tag == "" {
			tag = fmt.Sprintf("outbound-%d", i+1)
		}
		if !strings.HasPrefix(tag, prefix) {
			row["tag"] = prefix + tag
		}
	}
}

func sanitizeOutboundSubURL(rawURL string, allowPrivate bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", common.NewError("only http and https are allowed")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", common.NewError("missing host")
	}
	if allowPrivate {
		return parsed.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return "", common.NewError("private subscription host is not allowed")
		}
	}
	return parsed.String(), nil
}
