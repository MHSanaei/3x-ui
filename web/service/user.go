package service

import (
	"errors"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"

	"gorm.io/gorm"
)

type UserService struct{}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string, secret string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ? and password = ? and login_secret = ?", username, password, secret).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}
	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"username": username, "password": password}).
		Error
}

func (s *UserService) UpdateUserSecret(id int, secret string) error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Update("login_secret", secret).
		Error
}

func (s *UserService) RemoveUserSecret() error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("1 = 1").
		Update("login_secret", "").
		Error
}

func (s *UserService) GetUserSecret(id int) *model.User {
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).
		Where("id = ?", id).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	return user
}

func (s *UserService) CheckSecretExistence() (bool, error) {
	db := database.GetDB()

	var count int64
	err := db.Model(model.User{}).
		Where("login_secret IS NOT NULL").
		Count(&count).
		Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = password
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = password
	return db.Save(user).Error
}
