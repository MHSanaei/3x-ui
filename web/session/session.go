package session

import (
	"encoding/gob"

	"x-ui/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const loginUser = "LOGIN_USER"

func init() {
	gob.Register(model.User{})
}

func SetLoginUser(c *gin.Context, user *model.User) error {
	s := sessions.Default(c)
	s.Set(loginUser, user)
	return s.Save()
}

func SetMaxAge(c *gin.Context, maxAge int) error {
	s := sessions.Default(c)
	s.Options(sessions.Options{
		Path:   "/",
		MaxAge: maxAge,
	})
	return s.Save()
}

func GetLoginUser(c *gin.Context) *model.User {
	s := sessions.Default(c)
	if obj := s.Get(loginUser); obj != nil {
		if user, ok := obj.(model.User); ok {
			return &user
		}
	}
	return nil
}

func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != nil
}

func ClearSession(c *gin.Context) error {
	s := sessions.Default(c)
	s.Clear()
	s.Options(sessions.Options{
		Path:   "/",
		MaxAge: -1,
	})
	if err := s.Save(); err != nil {
		return err
	}
	c.SetCookie("3x-ui", "", -1, "/", "", false, true)
	return nil
}
