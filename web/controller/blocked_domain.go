package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"x-ui/database/model"
	"x-ui/web/service"
)

type BlockedDomainController struct {
	service *service.BlockedDomainService
}

func NewBlockedDomainController(g *gin.RouterGroup) *BlockedDomainController {
	ctrl := &BlockedDomainController{service: &service.BlockedDomainService{}}
	r := g.Group("/blocked-domains")
	r.GET("/", ctrl.List)
	r.POST("/", ctrl.Create)
	r.PUT("/:id", ctrl.Update)
	r.DELETE("/:id", ctrl.Delete)
	return ctrl
}

func (ctrl *BlockedDomainController) List(c *gin.Context) {
	domains, err := ctrl.service.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": domains})
}

func (ctrl *BlockedDomainController) Create(c *gin.Context) {
	var domain model.BlockedDomain
	if err := c.ShouldBindJSON(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	if err := ctrl.service.Create(&domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": domain})
}

func (ctrl *BlockedDomainController) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "invalid id"})
		return
	}
	var domain model.BlockedDomain
	if err := c.ShouldBindJSON(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	domain.Id = id
	if err := ctrl.service.Update(&domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": domain})
}

func (ctrl *BlockedDomainController) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": "invalid id"})
		return
	}
	if err := ctrl.service.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
} 