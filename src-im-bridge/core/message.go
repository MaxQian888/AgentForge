package core

import "time"

// Message represents an incoming IM message from any platform.
type Message struct {
	Platform   string
	SessionKey string // format: platform:chatID:userID
	UserID     string
	UserName   string
	ChatID     string
	ChatName   string
	Content    string
	ReplyCtx   any       // platform-specific reply context
	Timestamp  time.Time
	IsGroup    bool // group chat or DM
}
