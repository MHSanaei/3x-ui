package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"x-ui/database/model"
	"x-ui/database"
)

type BlockedDomainController struct {
}

func NewBlockedDomainController(g *gin.RouterGroup) *BlockedDomainController {
	ctrl := &BlockedDomainController{}
	r := g.Group("/blocked-domains")
	r.GET("/", ctrl.List)
	r.POST("/", ctrl.Create)
	r.PUT("/:id", ctrl.Update)
	r.DELETE("/:id", ctrl.Delete)
	return ctrl
}

func (ctrl *BlockedDomainController) List(c *gin.Context) {
	var domains []model.BlockedDomain
	err := database.GetDB().Find(&domains).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "obj": domains})
}

func (ctrl *BlockedDomainController) Create(c *gin.Context) {
	var domain model.BlockedDomain
	if err := c.ShouldBind(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	err := database.GetDB().Create(&domain).Error
	if err != nil {
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
	if err := c.ShouldBind(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "msg": err.Error()})
		return
	}
	domain.Id = id
	err = database.GetDB().Save(&domain).Error
	if err != nil {
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
	err = database.GetDB().Delete(&model.BlockedDomain{}, id).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
} 