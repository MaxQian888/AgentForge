package repository

import (
	"context"

	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type PluginRegistryDBBinder interface {
	Save(ctx context.Context, record *model.PluginRecord) error
	GetByID(ctx context.Context, pluginID string) (*model.PluginRecord, error)
	List(ctx context.Context, filter model.PluginFilter) ([]*model.PluginRecord, error)
	Delete(ctx context.Context, pluginID string) error
	DB() *gorm.DB
	WithDB(db *gorm.DB) PluginRegistryDBBinder
}

type PluginInstanceDBBinder interface {
	UpsertCurrent(ctx context.Context, snapshot *model.PluginInstanceSnapshot) error
	GetCurrentByPluginID(ctx context.Context, pluginID string) (*model.PluginInstanceSnapshot, error)
	DeleteByPluginID(ctx context.Context, pluginID string) error
	WithDB(db *gorm.DB) PluginInstanceDBBinder
}

type PluginEventDBBinder interface {
	Append(ctx context.Context, event *model.PluginEventRecord) error
	ListByPluginID(ctx context.Context, pluginID string, limit int) ([]*model.PluginEventRecord, error)
	WithDB(db *gorm.DB) PluginEventDBBinder
}
