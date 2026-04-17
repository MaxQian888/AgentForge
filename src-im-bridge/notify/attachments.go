package notify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

// resolveAttachments materializes each AttachmentRequest into a
// core.Attachment whose ContentRef points at a file on disk (for base64 or
// staged payloads) or at the original URL. If staging isn't configured, only
// URL-based attachments are accepted.
func (r *Receiver) resolveAttachments(requests []AttachmentRequest) ([]core.Attachment, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	out := make([]core.Attachment, 0, len(requests))
	for i, req := range requests {
		att := core.Attachment{
			ID:        strings.TrimSpace(req.ID),
			Kind:      coerceAttachmentKind(req.Kind, req.MimeType),
			MimeType:  strings.TrimSpace(req.MimeType),
			Filename:  strings.TrimSpace(req.Filename),
			SizeBytes: req.SizeBytes,
			Metadata:  req.Metadata,
		}

		switch {
		case strings.TrimSpace(req.StagedID) != "":
			path, ok := r.staging.Lookup(strings.TrimSpace(req.StagedID))
			if !ok {
				return nil, fmt.Errorf("attachment[%d]: staged id %q not found", i, req.StagedID)
			}
			att.ContentRef = path
		case strings.TrimSpace(req.ContentRef) != "":
			att.ContentRef = strings.TrimSpace(req.ContentRef)
		case strings.TrimSpace(req.URL) != "":
			att.ContentRef = strings.TrimSpace(req.URL)
		case strings.TrimSpace(req.DataBase64) != "":
			if r.staging == nil {
				return nil, fmt.Errorf("attachment[%d]: base64 payload rejected — staging disabled", i)
			}
			raw, err := base64.StdEncoding.DecodeString(req.DataBase64)
			if err != nil {
				return nil, fmt.Errorf("attachment[%d]: base64 decode: %w", i, err)
			}
			id, path, size, sErr := r.staging.Stage(att.Filename, bytes.NewReader(raw), int64(len(raw)))
			if sErr != nil {
				return nil, fmt.Errorf("attachment[%d]: staging: %w", i, sErr)
			}
			if att.ID == "" {
				att.ID = id
			}
			att.ContentRef = path
			if att.SizeBytes == 0 {
				att.SizeBytes = size
			}
		default:
			return nil, fmt.Errorf("attachment[%d]: must provide one of staged_id, content_ref, url, or data_base64", i)
		}
		if att.ID == "" {
			att.ID = strings.TrimSpace(req.StagedID)
		}
		out = append(out, att)
	}
	return out, nil
}

func coerceAttachmentKind(raw, mimeType string) core.AttachmentKind {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "image":
		return core.AttachmentKindImage
	case "logs":
		return core.AttachmentKindLogs
	case "patch":
		return core.AttachmentKindPatch
	case "report":
		return core.AttachmentKindReport
	case "audio":
		return core.AttachmentKindAudio
	case "video":
		return core.AttachmentKindVideo
	case "file", "":
		// fall through to mime-based inference
	default:
		return core.AttachmentKind(strings.TrimSpace(raw))
	}
	m := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(m, "image/"):
		return core.AttachmentKindImage
	case strings.HasPrefix(m, "audio/"):
		return core.AttachmentKindAudio
	case strings.HasPrefix(m, "video/"):
		return core.AttachmentKindVideo
	case m == "text/x-patch" || m == "text/x-diff":
		return core.AttachmentKindPatch
	case m == "text/x-log" || strings.Contains(m, "ndjson"):
		return core.AttachmentKindLogs
	}
	return core.AttachmentKindFile
}

// handleUploadAttachment stages an incoming multipart upload and returns a
// handle the caller can use in a subsequent /im/send call. Operators wire
// the backend to call this endpoint before /im/send when they want the bridge
// to manage the file's lifecycle.
func (r *Receiver) handleUploadAttachment(w http.ResponseWriter, req *http.Request) {
	if r.staging == nil {
		http.Error(w, "attachment staging disabled", http.StatusServiceUnavailable)
		return
	}
	start := r.now()
	contentType := req.Header.Get("Content-Type")
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		http.Error(w, "invalid content type", http.StatusBadRequest)
		return
	}

	var (
		id       string
		path     string
		size     int64
		kind     core.AttachmentKind
		filename string
		mimeType string
	)

	if strings.HasPrefix(mt, "multipart/") {
		if err := req.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, fmt.Sprintf("parse multipart: %v", err), http.StatusBadRequest)
			return
		}
		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("missing file part: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()
		filename = header.Filename
		mimeType = header.Header.Get("Content-Type")
		kind = coerceAttachmentKind(req.FormValue("kind"), mimeType)
		id, path, size, err = r.staging.Stage(filename, file, header.Size)
		if err != nil {
			http.Error(w, fmt.Sprintf("stage: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		// Treat as raw body upload; headers carry metadata.
		filename = strings.TrimSpace(req.Header.Get("X-Attachment-Filename"))
		mimeType = strings.TrimSpace(req.Header.Get("X-Attachment-Mime"))
		kind = coerceAttachmentKind(req.Header.Get("X-Attachment-Kind"), mimeType)
		declared, _ := strconv.ParseInt(strings.TrimSpace(req.Header.Get("Content-Length")), 10, 64)
		id, path, size, err = r.staging.Stage(filename, req.Body, declared)
		if err != nil {
			http.Error(w, fmt.Sprintf("stage: %v", err), http.StatusInternalServerError)
			return
		}
	}

	log.WithFields(log.Fields{
		"component": "notify.attachments",
		"id":        id,
		"size":      size,
		"kind":      string(kind),
	}).Debug("staged attachment")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"path":       path,
		"size_bytes": size,
		"kind":       string(kind),
		"mime_type":  mimeType,
		"filename":   filename,
		"latency_ms": r.now().Sub(start).Milliseconds(),
	})
}

// handleReactionIngress accepts a reaction event payload directly from
// operator tooling. Provider-side adapters call this endpoint when they
// observe a reaction on a message they previously delivered.
func (r *Receiver) handleReactionIngress(w http.ResponseWriter, req *http.Request) {
	bodyBytes, ok := r.verifyAndRememberDelivery(w, req, "/im/reactions")
	if !ok {
		return
	}
	var ev ReactionEvent
	if err := json.Unmarshal(bodyBytes, &ev); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(ev.Platform) == "" {
		ev.Platform = r.metadata.Source
	}
	if ev.ReactedAt.IsZero() {
		ev.ReactedAt = r.now().UTC()
	}
	if ev.EmojiCode == "" && ev.RawEmoji != "" {
		ev.EmojiCode = core.ResolveReactionCode(ev.Platform, ev.RawEmoji)
	}
	if r.reactionSink == nil {
		http.Error(w, "reaction sink not configured", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.reactionSink.RecordReaction(ctx, ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

// dispatchReaction forwards an observed reaction event to the receiver's
// reaction sink. Called by provider adapters that build a ReactionEvent from
// inbound platform callbacks directly in-process.
func (r *Receiver) dispatchReaction(ctx context.Context, event ReactionEvent) error {
	if r == nil || r.reactionSink == nil {
		return errors.New("reaction sink not configured")
	}
	return r.reactionSink.RecordReaction(ctx, event)
}
