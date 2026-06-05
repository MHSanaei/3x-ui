package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/web/middleware"
	"github.com/mhsanaei/3x-ui/v3/web/service"
	"github.com/mhsanaei/3x-ui/v3/web/session"

	"github.com/gin-gonic/gin"
)

// AdminController exposes the admin-only RBAC + wallet management API under
// /panel/api/admin. Every route is gated by middleware.RequireAdmin, so a
// non-admin session (or none) can never reach user management, balance
// adjustments or the global transaction log.
type AdminController struct {
	userService   service.UserService
	walletService service.WalletService
}

func NewAdminController(g *gin.RouterGroup) *AdminController {
	a := &AdminController{}
	a.initRouter(g)
	return a
}

func (a *AdminController) initRouter(g *gin.RouterGroup) {
	admin := g.Group("/admin")
	admin.Use(middleware.RequireAdmin())

	admin.GET("/users", a.listUsers)
	admin.POST("/users", a.createUser)
	admin.POST("/users/:id", a.updateUser)
	admin.POST("/users/:id/del", a.deleteUser)
	admin.POST("/users/:id/balance", a.adjustBalance)
	admin.GET("/transactions", a.listTransactions)
}

type adminUserForm struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Balance  int64  `json:"balance"`
}

type balanceAdjustForm struct {
	Op          string `json:"op"`     // add | deduct | set
	Amount      int64  `json:"amount"` // for set: target balance; otherwise delta
	Description string `json:"description"`
}

func (a *AdminController) listUsers(c *gin.Context) {
	users, err := a.userService.ListUsers()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "fail"), err)
		return
	}
	jsonObj(c, users, nil)
}

func (a *AdminController) createUser(c *gin.Context) {
	var form adminUserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user, err := a.userService.AdminCreateUser(service.AdminUserInput{
		Username: form.Username,
		Password: form.Password,
		FullName: form.FullName,
		Phone:    form.Phone,
		Email:    form.Email,
		Role:     form.Role,
		Balance:  form.Balance,
	})
	if err != nil {
		if msg := adminUserErrorMessage(c, err); msg != "" {
			pureJsonMsg(c, http.StatusOK, false, msg)
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, user, nil)
}

func (a *AdminController) updateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form adminUserForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	user, err := a.userService.AdminUpdateUser(id, service.AdminUserInput{
		Username: form.Username,
		Password: form.Password,
		FullName: form.FullName,
		Phone:    form.Phone,
		Email:    form.Email,
		Role:     form.Role,
	})
	if err != nil {
		if msg := adminUserErrorMessage(c, err); msg != "" {
			pureJsonMsg(c, http.StatusOK, false, msg)
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, user, nil)
}

func (a *AdminController) deleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if self := session.GetLoginUser(c); self != nil && self.Id == id {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.users.toasts.cannotDeleteSelf"))
		return
	}
	if err := a.userService.DeleteUser(id); err != nil {
		if msg := adminUserErrorMessage(c, err); msg != "" {
			pureJsonMsg(c, http.StatusOK, false, msg)
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.users.toasts.userDeleted"), nil)
}

func (a *AdminController) adjustBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var form balanceAdjustForm
	if err := c.ShouldBindJSON(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if form.Amount < 0 {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.users.toasts.invalidAmount"))
		return
	}
	desc := form.Description
	if desc == "" {
		desc = "admin adjustment"
	}
	switch form.Op {
	case "add":
		_, err = a.walletService.Credit(id, form.Amount, desc)
	case "deduct":
		_, err = a.walletService.Debit(id, form.Amount, desc)
	case "set":
		_, err = a.walletService.SetBalance(id, form.Amount, desc)
	default:
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.users.toasts.invalidOp"))
		return
	}
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.clients.toasts.insufficientBalance"))
			return
		}
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	balance, err := a.walletService.GetBalance(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "fail"), err)
		return
	}
	jsonObj(c, gin.H{"balance": balance}, nil)
}

func (a *AdminController) listTransactions(c *gin.Context) {
	var userId *int
	if raw := c.Query("userId"); raw != "" {
		if id, err := strconv.Atoi(raw); err == nil {
			userId = &id
		}
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))
	rows, err := a.walletService.ListTransactions(userId, limit, offset)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "fail"), err)
		return
	}
	jsonObj(c, rows, nil)
}

// adminUserErrorMessage maps known user-service sentinels to localized
// messages. Returns "" for unknown errors so the caller falls back to a
// generic handler.
func adminUserErrorMessage(c *gin.Context, err error) string {
	switch {
	case errors.Is(err, service.ErrUsernameTaken):
		return I18nWeb(c, "pages.register.toasts.usernameTaken")
	case errors.Is(err, service.ErrEmailTaken):
		return I18nWeb(c, "pages.register.toasts.emailTaken")
	case errors.Is(err, service.ErrInvalidUsername):
		return I18nWeb(c, "pages.register.toasts.invalidUsername")
	case errors.Is(err, service.ErrInvalidEmail):
		return I18nWeb(c, "pages.register.toasts.invalidEmail")
	case errors.Is(err, service.ErrWeakPassword):
		return I18nWeb(c, "pages.register.toasts.weakPassword")
	case errors.Is(err, service.ErrLastAdmin):
		return I18nWeb(c, "pages.users.toasts.lastAdmin")
	default:
		return ""
	}
}
