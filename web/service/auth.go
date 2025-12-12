package service

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/gorm"
)

type AuthService struct {
	DB        *gorm.DB
	JWTSecret []byte
}

func NewAuthService() *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-me"
	}
	return &AuthService{
		DB:        database.GetDB(),
		JWTSecret: []byte(secret),
	}
}

// Регистрация (используем существующую модель: Username + PasswordHash + Role)
func (s *AuthService) Register(username, rawPassword, role string) error {
	if role == "" {
		role = "reader"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), 12)
	if err != nil {
		return err
	}
	u := &model.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
	}
	return s.DB.Create(u).Error
}

func (s *AuthService) Login(username, rawPassword string) (string, *model.User, error) {
	var u model.User
	if err := s.DB.Where("username = ?", username).First(&u).Error; err != nil {
		if database.IsNotFound(err) {
			return "", nil, errors.New("user not found")
		}
		return "", nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(rawPassword)); err != nil {
		return "", nil, errors.New("invalid password")
	}

	claims := jwt.MapClaims{
		"id":       u.Id,
		"username": u.Username,
		"role":     u.Role,
		"exp":      time.Now().Add(72 * time.Hour).Unix(),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.JWTSecret)
	return tok, &u, err
}
