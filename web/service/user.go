package service

import (
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	ldaputil "github.com/mhsanaei/3x-ui/v2/util/ldap"
	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

// UserService provides business logic for user management and authentication.
// It handles user creation, login, password management, and 2FA operations.
type UserService struct {
	settingService SettingService
}

type UserView struct {
	Id        int    `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func userToView(user model.User) UserView {
	role := user.Role
	if role == "" {
		role = model.UserRoleAdmin
	}
	return UserView{
		Id:        user.Id,
		Username:  user.Username,
		Role:      role,
		Enabled:   user.Enabled,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func normalizeUserRole(role string) (string, error) {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		role = model.UserRoleAdmin
	}
	switch role {
	case model.UserRoleAdmin, model.UserRoleAPI:
		return role, nil
	default:
		return "", errors.New("unsupported user role")
	}
}

// GetFirstUser retrieves the first user from the database.
// This is typically used for initial setup or when there's only one admin user.
func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	if user.Role == "" {
		user.Role = model.UserRoleAdmin
	}
	return user, nil
}

func (s *UserService) GetById(id int) (*model.User, error) {
	db := database.GetDB()
	user := &model.User{}
	if err := db.Model(model.User{}).Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}
	if user.Role == "" {
		user.Role = model.UserRoleAdmin
	}
	return user, nil
}

func (s *UserService) ListUsers() ([]UserView, error) {
	db := database.GetDB()
	users := []model.User{}
	if err := db.Model(model.User{}).Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	views := make([]UserView, 0, len(users))
	for _, user := range users {
		views = append(views, userToView(user))
	}
	return views, nil
}

func (s *UserService) CheckUser(username string, password string, twoFactorCode string) *model.User {
	db := database.GetDB()

	user := &model.User{}

	err := db.Model(model.User{}).
		Where("username = ?", username).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}
	if user.Role == "" {
		user.Role = model.UserRoleAdmin
	}
	if !user.CanUsePanel() {
		return nil
	}

	// If LDAP enabled and local password check fails, attempt LDAP auth
	if !crypto.CheckPasswordHash(user.Password, password) {
		ldapEnabled, _ := s.settingService.GetLdapEnable()
		if !ldapEnabled {
			return nil
		}

		host, _ := s.settingService.GetLdapHost()
		port, _ := s.settingService.GetLdapPort()
		useTLS, _ := s.settingService.GetLdapUseTLS()
		bindDN, _ := s.settingService.GetLdapBindDN()
		ldapPass, _ := s.settingService.GetLdapPassword()
		baseDN, _ := s.settingService.GetLdapBaseDN()
		userFilter, _ := s.settingService.GetLdapUserFilter()
		userAttr, _ := s.settingService.GetLdapUserAttr()

		cfg := ldaputil.Config{
			Host:       host,
			Port:       port,
			UseTLS:     useTLS,
			BindDN:     bindDN,
			Password:   ldapPass,
			BaseDN:     baseDN,
			UserFilter: userFilter,
			UserAttr:   userAttr,
		}
		ok, err := ldaputil.AuthenticateUser(cfg, username, password)
		if err != nil || !ok {
			return nil
		}
		// On successful LDAP auth, continue 2FA checks below
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		logger.Warning("check two factor err:", err)
		return nil
	}

	if twoFactorEnable {
		twoFactorToken, err := s.settingService.GetTwoFactorToken()

		if err != nil {
			logger.Warning("check two factor token err:", err)
			return nil
		}

		if gotp.NewDefaultTOTP(twoFactorToken).Now() != twoFactorCode {
			return nil
		}
	}

	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)

	if err != nil {
		return err
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		return err
	}

	if twoFactorEnable {
		s.settingService.SetTwoFactorEnable(false)
		s.settingService.SetTwoFactorToken("")
	}

	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"username": username, "password": hashedPassword}).
		Error
}

func (s *UserService) CreateUser(username string, password string, role string, enabled bool) (*UserView, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("username can not be empty")
	}
	if password == "" {
		return nil, errors.New("password can not be empty")
	}
	role, err := normalizeUserRole(role)
	if err != nil {
		return nil, err
	}
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username: username,
		Password: hashedPassword,
		Role:     role,
		Enabled:  enabled,
	}
	if err := database.GetDB().Create(user).Error; err != nil {
		return nil, err
	}
	view := userToView(*user)
	return &view, nil
}

func (s *UserService) UpdateManagedUser(id int, username string, password string, role string, enabled bool) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username can not be empty")
	}
	role, err := normalizeUserRole(role)
	if err != nil {
		return err
	}
	user, err := s.GetById(id)
	if err != nil {
		return err
	}
	if user.IsAdmin() && (role != model.UserRoleAdmin || !enabled) {
		if err := s.ensureAnotherEnabledAdmin(id); err != nil {
			return err
		}
	}

	updates := map[string]any{
		"username": username,
		"role":     role,
		"enabled":  enabled,
	}
	if password != "" {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
		if err != nil {
			return err
		}
		updates["password"] = hashedPassword
	}
	return database.GetDB().Model(model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (s *UserService) DeleteUser(id int) error {
	user, err := s.GetById(id)
	if err != nil {
		return err
	}
	if user.IsAdmin() {
		if err := s.ensureAnotherEnabledAdmin(id); err != nil {
			return err
		}
	}
	return database.GetDB().Delete(&model.User{}, id).Error
}

func (s *UserService) ensureAnotherEnabledAdmin(excludeId int) error {
	var count int64
	if err := database.GetDB().Model(model.User{}).
		Where("id <> ? AND enabled = ? AND (role = ? OR role = '' OR role IS NULL)", excludeId, true, model.UserRoleAdmin).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("at least one enabled admin user is required")
	}
	return nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)

	if er != nil {
		return er
	}

	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = hashedPassword
		user.Role = model.UserRoleAdmin
		user.Enabled = true
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	user.Role = model.UserRoleAdmin
	user.Enabled = true
	return db.Save(user).Error
}
