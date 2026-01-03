package service

import (
	"errors"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserAdminService struct {
	DB *gorm.DB
}

func NewUserAdminService() *UserAdminService {
	return &UserAdminService{DB: database.GetDB()}
}

type UserDTO struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func toDTO(u *model.User) UserDTO {
	return UserDTO{Id: u.Id, Username: u.Username, Role: u.Role}
}

func (s *UserAdminService) ListUsers() ([]UserDTO, error) {
	var users []model.User
	if err := s.DB.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]UserDTO, 0, len(users))
	for i := range users {
		out = append(out, toDTO(&users[i]))
	}
	return out, nil
}

func (s *UserAdminService) CreateUser(username, rawPassword, role string) (UserDTO, error) {
	if username == "" || rawPassword == "" {
		return UserDTO{}, errors.New("username and password required")
	}
	if role == "" {
		role = "reader"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), 12)
	if err != nil {
		return UserDTO{}, err
	}
	u := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := s.DB.Create(u).Error; err != nil {
		return UserDTO{}, err
	}
	return toDTO(u), nil
}

func (s *UserAdminService) UpdateUserRole(id int, newRole string) (UserDTO, error) {
	var u model.User
	if err := s.DB.First(&u, id).Error; err != nil {
		return UserDTO{}, err
	}
	if newRole == "" {
		return UserDTO{}, errors.New("role required")
	}
	u.Role = newRole
	if err := s.DB.Save(&u).Error; err != nil {
		return UserDTO{}, err
	}
	return toDTO(&u), nil
}

func (s *UserAdminService) ResetPassword(id int, newPassword string) error {
	if newPassword == "" {
		return errors.New("password required")
	}
	var u model.User
	if err := s.DB.First(&u, id).Error; err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.DB.Save(&u).Error
}

func (s *UserAdminService) DeleteUser(id int) error {
	return s.DB.Delete(&model.User{}, id).Error
}
