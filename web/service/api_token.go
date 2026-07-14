package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

const apiTokenLength = 48

type ApiTokenView struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
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
			Id:        token.Id,
			Name:      token.Name,
			Enabled:   token.Enabled,
			CreatedAt: token.CreatedAt,
		})
	}
	return views, nil
}

func (s *ApiTokenService) Create(name string) (*ApiTokenView, error) {
	if name == "" {
		return nil, errors.New("name can not be empty")
	}

	plain, err := generateAPIToken()
	if err != nil {
		return nil, err
	}

	row := &model.ApiToken{
		Name:    name,
		Token:   hashTokenSHA256(plain),
		Enabled: true,
	}
	if err := database.GetDB().Create(row).Error; err != nil {
		return nil, err
	}

	return &ApiTokenView{
		Id:        row.Id,
		Name:      row.Name,
		Token:     plain,
		Enabled:   row.Enabled,
		CreatedAt: row.CreatedAt,
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

func (s *ApiTokenService) Match(presented string) bool {
	if presented == "" {
		return false
	}

	hashed := hashTokenSHA256(presented)
	rows := []model.ApiToken{}
	if err := database.GetDB().
		Model(model.ApiToken{}).
		Where("enabled = ?", true).
		Find(&rows).Error; err != nil {
		return false
	}

	for _, row := range rows {
		if subtle.ConstantTimeCompare([]byte(row.Token), []byte(hashed)) == 1 {
			return true
		}
	}
	return false
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
