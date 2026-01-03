package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/web/middleware"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

type UserAdminController struct {
	svc *service.UserAdminService
}

func NewUserAdminController(api *gin.RouterGroup) *UserAdminController {
	c := &UserAdminController{svc: service.NewUserAdminService()}

	admin := api.Group("/admin")
	admin.Use(middleware.AuthRequired(), middleware.RequireRole("admin"))
	{
		admin.GET("/users", c.list)
		admin.POST("/users", c.create)
		admin.PATCH("/users/:id/role", c.updateRole)
		admin.PATCH("/users/:id/password", c.resetPassword)
		admin.DELETE("/users/:id", c.delete)
		admin.GET("/healthz", func(ctx *gin.Context) { ctx.JSON(200, gin.H{"ok": true}) })
	}

	// кто угодно авторизованный может посмотреть свой профиль
	me := api.Group("/me")
	me.Use(middleware.AuthRequired())
	{
		me.GET("", c.me)
	}

	return c
}

func (c *UserAdminController) list(ctx *gin.Context) {
	users, err := c.svc.ListUsers()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, users)
}

type createReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

func (c *UserAdminController) create(ctx *gin.Context) {
	var req createReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	u, err := c.svc.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, u)
}

type roleReq struct {
	Role string `json:"role" binding:"required"`
}

func (c *UserAdminController) updateRole(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	var req roleReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	u, err := c.svc.UpdateUserRole(id, req.Role)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, u)
}

type pwReq struct {
	Password string `json:"password" binding:"required"`
}

func (c *UserAdminController) resetPassword(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	var req pwReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	if err := c.svc.ResetPassword(id, req.Password); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

func (c *UserAdminController) delete(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	if err := c.svc.DeleteUser(id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

func (c *UserAdminController) me(ctx *gin.Context) {
	uidVal, _ := ctx.Get("user_id")
	roleVal, _ := ctx.Get("role")
	ctx.JSON(http.StatusOK, gin.H{"id": uidVal, "role": roleVal})
}
