package qianchuan

import (
	"os"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
)

// Register installs Qianchuan into reg using env-driven config:
//
//	QIANCHUAN_HOST       (default: ad.oceanengine.com)
//	QIANCHUAN_APP_ID     (required for OAuth)
//	QIANCHUAN_APP_SECRET (required for OAuth)
func Register(reg *adsplatform.Registry) {
	reg.Register("qianchuan", func() adsplatform.Provider {
		host := os.Getenv("QIANCHUAN_HOST")
		return NewProvider(NewClient(Options{
			Host:      host,
			AppID:     os.Getenv("QIANCHUAN_APP_ID"),
			AppSecret: os.Getenv("QIANCHUAN_APP_SECRET"),
		}))
	})
}
