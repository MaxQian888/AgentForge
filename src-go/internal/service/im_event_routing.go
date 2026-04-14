package service

import (
	"context"

	"github.com/react-go-quick-starter/server/internal/model"
)

type IMEventChannelResolver interface {
	ResolveChannelsForEvent(ctx context.Context, eventType string, platform string, channelID string) ([]*model.IMChannel, error)
}

