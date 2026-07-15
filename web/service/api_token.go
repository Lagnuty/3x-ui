package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

const apiTokenLength = 48

type ApiTokenView struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Token      string `json:"token,omitempty"`
	UserId     int    `json:"userId"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
	LastUsedAt int64  `json:"lastUsedAt"`
}

type ApiTokenService struct{}

func (s *ApiTokenService) List() ([]ApiTokenView, error) {
	db := database.GetDB()
	tokens := []model.ApiToken{}
	if err := db.Model(model.ApiToken{}).Order("id desc").Find(&tokens).Error; err != nil {
		return nil, err
	}

	views := make([]ApiTokenView, 0, len(tokens))
	for _, token := range tokens {
		views = append(views, ApiTokenView{
			Id:         token.Id,
			Name:       token.Name,
			UserId:     token.UserId,
			Enabled:    token.Enabled,
			CreatedAt:  token.CreatedAt,
			UpdatedAt:  token.UpdatedAt,
			LastUsedAt: token.LastUsedAt,
		})
	}
	return views, nil
}

func (s *ApiTokenService) Create(name string, userId int) (*ApiTokenView, error) {
	if name == "" {
		return nil, errors.New("name can not be empty")
	}
	if userId > 0 {
		user := &model.User{}
		if err := database.GetDB().Model(model.User{}).Where("id = ? AND enabled = ?", userId, true).First(user).Error; err != nil {
			return nil, errors.New("enabled token user not found")
		}
	}

	plain, err := generateAPIToken()
	if err != nil {
		return nil, err
	}

	row := &model.ApiToken{
		Name:    name,
		Token:   hashTokenSHA256(plain),
		UserId:  userId,
		Enabled: true,
	}
	if err := database.GetDB().Create(row).Error; err != nil {
		return nil, err
	}

	return &ApiTokenView{
		Id:         row.Id,
		Name:       row.Name,
		Token:      plain,
		UserId:     row.UserId,
		Enabled:    row.Enabled,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		LastUsedAt: row.LastUsedAt,
	}, nil
}

func (s *ApiTokenService) Delete(id int) error {
	return database.GetDB().Delete(&model.ApiToken{}, id).Error
}

func (s *ApiTokenService) SetEnabled(id int, enabled bool) error {
	return database.GetDB().
		Model(model.ApiToken{}).
		Where("id = ?", id).
		Update("enabled", enabled).
		Error
}

func (s *ApiTokenService) Match(presented string) (*model.ApiToken, bool) {
	if presented == "" {
		return nil, false
	}

	hashed := hashTokenSHA256(presented)
	rows := []model.ApiToken{}
	if err := database.GetDB().
		Model(model.ApiToken{}).
		Where("enabled = ?", true).
		Find(&rows).Error; err != nil {
		return nil, false
	}

	for _, row := range rows {
		if subtle.ConstantTimeCompare([]byte(row.Token), []byte(hashed)) == 1 {
			database.GetDB().Model(model.ApiToken{}).
				Where("id = ?", row.Id).
				Update("last_used_at", time.Now().UnixMilli())
			return &row, true
		}
	}
	return nil, false
}

func generateAPIToken() (string, error) {
	buf := make([]byte, 36)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if len(token) > apiTokenLength {
		return token[:apiTokenLength], nil
	}
	return token, nil
}

func hashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
