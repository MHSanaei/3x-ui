package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/entity"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

type CustomGeoController struct {
	BaseController
	customGeoService *service.CustomGeoService
}

func NewCustomGeoController(g *gin.RouterGroup, customGeo *service.CustomGeoService) *CustomGeoController {
	a := &CustomGeoController{customGeoService: customGeo}
	a.initRouter(g)
	return a
}

func (a *CustomGeoController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/aliases", a.aliases)
	g.POST("/add", a.add)
	g.POST("/update/:id", a.update)
	g.POST("/delete/:id", a.delete)
	g.POST("/download/:id", a.download)
	g.POST("/update-all", a.updateAll)
}

func mapCustomGeoErr(c *gin.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, service.ErrCustomGeoInvalidType):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrInvalidType"))
	case errors.Is(err, service.ErrCustomGeoAliasRequired):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasRequired"))
	case errors.Is(err, service.ErrCustomGeoAliasPattern):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasPattern"))
	case errors.Is(err, service.ErrCustomGeoAliasReserved):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrAliasReserved"))
	case errors.Is(err, service.ErrCustomGeoURLRequired):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlRequired"))
	case errors.Is(err, service.ErrCustomGeoInvalidURL):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrInvalidUrl"))
	case errors.Is(err, service.ErrCustomGeoURLScheme):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlScheme"))
	case errors.Is(err, service.ErrCustomGeoURLHost):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlHost"))
	case errors.Is(err, service.ErrCustomGeoDuplicateAlias):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDuplicateAlias"))
	case errors.Is(err, service.ErrCustomGeoNotFound):
		return errors.New(I18nWeb(c, "pages.index.customGeoErrNotFound"))
	case errors.Is(err, service.ErrCustomGeoDownload):
		logger.Warning("custom geo download:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDownload"))
	case errors.Is(err, service.ErrCustomGeoSSRFBlocked):
		logger.Warning("custom geo SSRF blocked:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrUrlHost"))
	case errors.Is(err, service.ErrCustomGeoPathTraversal):
		logger.Warning("custom geo path traversal blocked:", err)
		return errors.New(I18nWeb(c, "pages.index.customGeoErrDownload"))
	default:
		return err
	}
}

func (a *CustomGeoController) list(c *gin.Context) {
	list, err := a.customGeoService.GetAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastList"), mapCustomGeoErr(c, err))
		return
	}
	jsonObj(c, list, nil)
}

func (a *CustomGeoController) aliases(c *gin.Context) {
	out, err := a.customGeoService.GetAliasesForUI()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoAliasesError"), mapCustomGeoErr(c, err))
		return
	}
	jsonObj(c, out, nil)
}

type customGeoForm struct {
	Type  string `json:"type" form:"type"`
	Alias string `json:"alias" form:"alias"`
	Url   string `json:"url" form:"url"`
}

func (a *CustomGeoController) add(c *gin.Context) {
	var form customGeoForm
	if err := c.ShouldBind(&form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastAdd"), err)
		return
	}
	r := &model.CustomGeoResource{
		Type:  form.Type,
		Alias: form.Alias,
		Url:   form.Url,
	}
	err := a.customGeoService.Create(r)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastAdd"), mapCustomGeoErr(c, err))
}

func parseCustomGeoID(c *gin.Context, idStr string) (int, bool) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoInvalidId"), err)
		return 0, false
	}
	if id <= 0 {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoInvalidId"), errors.New(""))
		return 0, false
	}
	return id, true
}

func (a *CustomGeoController) update(c *gin.Context) {
	id, ok := parseCustomGeoID(c, c.Param("id"))
	if !ok {
		return
	}
	var form customGeoForm
	if bindErr := c.ShouldBind(&form); bindErr != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdate"), bindErr)
		return
	}
	r := &model.CustomGeoResource{
		Type:  form.Type,
		Alias: form.Alias,
		Url:   form.Url,
	}
	err := a.customGeoService.Update(id, r)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdate"), mapCustomGeoErr(c, err))
}

func (a *CustomGeoController) delete(c *gin.Context) {
	id, ok := parseCustomGeoID(c, c.Param("id"))
	if !ok {
		return
	}
	name, err := a.customGeoService.Delete(id)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastDelete", "fileName=="+name), mapCustomGeoErr(c, err))
}

func (a *CustomGeoController) download(c *gin.Context) {
	id, ok := parseCustomGeoID(c, c.Param("id"))
	if !ok {
		return
	}
	name, err := a.customGeoService.TriggerUpdate(id)
	jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastDownload", "fileName=="+name), mapCustomGeoErr(c, err))
}

func (a *CustomGeoController) updateAll(c *gin.Context) {
	res, err := a.customGeoService.TriggerUpdateAll()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.index.customGeoToastUpdateAll"), mapCustomGeoErr(c, err))
		return
	}
	if len(res.Failed) > 0 {
		c.JSON(http.StatusOK, entity.Msg{
			Success: false,
			Msg:     I18nWeb(c, "pages.index.customGeoErrUpdateAllIncomplete"),
			Obj:     res,
		})
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.index.customGeoToastUpdateAll"), res, nil)
}
