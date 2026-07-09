// Package service provides business logic services for the 3x-ui web panel.
package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/gorm"
)

// InboundCRUD defines core inbound CRUD operations
type InboundCRUD interface {
	GetInbounds(userId int) ([]*model.Inbound, error)
	GetInboundsSlim(userId int) ([]*model.Inbound, error)
	GetInboundOptions(userId int) ([]InboundOption, error)
	GetAllInbounds() ([]*model.Inbound, error)
	GetInboundsByTrafficReset(period string) ([]*model.Inbound, error)
	AddInbound(inbound *model.Inbound) (*model.Inbound, bool, error)
	DelInbound(id int) (bool, error)
	DelInbounds(ids []int) (BulkDelInboundResult, bool, error)
	GetInbound(id int) (*model.Inbound, error)
	GetInboundDetail(id int) (*model.Inbound, error)
	SetInboundEnable(id int, enable bool) (bool, error)
	UpdateInbound(inbound *model.Inbound) (*model.Inbound, bool, error)
}

// InboundClients defines client management operations
type InboundClients interface {
	GetClients(inbound *model.Inbound) ([]model.Client, error)
	GetClientsBySubId(inboundId int, subId string) ([]model.Client, error)
	GetAllEmails() ([]string, error)
	GetAllEmailSubIDs() (map[string]string, error)
	EmailUsedByOtherInbounds(email string, exceptInboundId int) (bool, error)
	EmailsUsedByOtherInbounds(emails []string, exceptInboundId int) (map[string]bool, error)
	EmailsByInbound(inboundId int) ([]string, error)
	GetClientByEmail(email string) (*xray.ClientTraffic, *model.Client, error)
	GetClientInboundByTrafficID(trafficId int) (*xray.ClientTraffic, *model.Inbound, error)
	GetClientInboundByEmail(email string) (*xray.ClientTraffic, *model.Inbound, error)
}

// InboundTraffic defines traffic statistics operations
type InboundTraffic interface {
	EnrichClientStats(db *gorm.DB, inb []*model.Inbound)
	BackfillClientStats(db *gorm.DB, inb []*model.Inbound) [][]model.Client
	OverlayInboundsClientStats(db *gorm.DB, inbounds []*model.Inbound)
	AddClientStat(tx *gorm.DB, inboundId int, client *model.Client) error
	UpdateClientStat(tx *gorm.DB, email string, client *model.Client) error
	DelClientStat(tx *gorm.DB, email string) error
	DelClientStatsByEmails(tx *gorm.DB, emails []string) error
	ResetClientTrafficByEmail(clientEmail string) error
	ResetClientTraffic(id int, clientEmail string) (bool, error)
	GetClientTrafficByEmail(email string) (*xray.ClientTraffic, error)
}

// InboundStream defines stream/TLS protocol operations
type InboundStream interface {
	NormalizeStreamSettings(inbound *model.Inbound)
	NormalizeMtprotoSecret(inbound *model.Inbound)
	NormalizeMtprotoXrayPort(inbound *model.Inbound, oldSettings string) error
	GetInboundTags() (string, error)
	GetClientReverseTags() (string, error)
}

// InboundXray defines Xray synchronization operations
type InboundXray interface {
	BuildRuntimeInboundForAPI(tx *gorm.DB, inbound *model.Inbound) (*model.Inbound, error)
	SyncInboundXrayConfig(inbound *model.Inbound) (bool, error)
	SyncLocalInboundXrayConfig(inbound *model.Inbound, runtimeInbound *xray.InboundConfig) (bool, error)
	GetXrayAPI() *xray.XrayAPI
	GetPanelHost() string
	BuildRuntimeInbound(inbound *model.Inbound) (*xray.InboundConfig, error)
}

// InboundClientIPs defines client IP management operations
type InboundClientIPs interface {
	UpdateClientIPs(tx *gorm.DB, oldEmail string, newEmail string) error
	DelClientIPs(tx *gorm.DB, email string) error
	DelClientIPsByEmails(tx *gorm.DB, emails []string) error
}

// InboundNode defines node-specific operations
type InboundNode interface {
	RuntimeFor(ib *model.Inbound) (runtime.Runtime, error)
	NodePushPlan(ib *model.Inbound) (runtime.Runtime, bool, bool, error)
	DelClientIPsByEmails(tx *gorm.DB, emails []string) error
	DelClientStatsByEmails(tx *gorm.DB, emails []string) error
}

// InboundRuntime defines runtime operations
type InboundRuntime interface {
	ReconcileInbound(inbound *model.Inbound) error
	AddTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (bool, bool, error)
	GetOnlineClients() []string
	GetOnlineClientsByGuid() map[string][]string
	AnyNodePending(inboundIds []int) bool
}

// InboundSearch defines search operations
type InboundSearch interface {
	SearchInbounds(query string) ([]*model.Inbound, error)
}

// InboundHelpers defines internal helper methods
type InboundHelpers interface {
	AnnotateLocalOriginGuid(inbounds []*model.Inbound)
	AnnotateFallbackParents(db *gorm.DB, inbounds []*model.Inbound)
	UpdateClientTraffics(tx *gorm.DB, oldInbound *model.Inbound, newInbound *model.Inbound) error
}

// InboundServiceInterface combines all interfaces for backward compatibility
type InboundServiceInterface interface {
	InboundCRUD
	InboundClients
	InboundTraffic
	InboundStream
	InboundXray
	InboundClientIPs
	InboundNode
	InboundRuntime
	InboundSearch
	InboundHelpers
}