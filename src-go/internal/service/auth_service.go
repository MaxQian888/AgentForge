// Package service implements business logic for authentication and user management.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/config"
	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository defines the interface for user persistence operations.
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error
}

// CacheRepository defines the interface for token caching operations.
type CacheRepository interface {
	SetRefreshToken(ctx context.Context, userID, token string, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, userID string) (string, error)
	DeleteRefreshToken(ctx context.Context, userID string) error
	BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}

// Claims holds custom JWT claims for access and refresh tokens.
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

type AuthService struct {
	userRepo  UserRepository
	cacheRepo CacheRepository
	cfg       *config.Config
}

func NewAuthService(userRepo UserRepository, cacheRepo CacheRepository, cfg *config.Config) *AuthService {
	return &AuthService{userRepo: userRepo, cacheRepo: cacheRepo, cfg: cfg}
}

func authAuditFields(ctx context.Context, action string) log.Fields {
	fields := log.Fields{
		"action": action,
	}
	if traceID := applog.TraceID(ctx); traceID != "" {
		fields["trace_id"] = traceID
	}
	meta := applog.GetRequestMetadata(ctx)
	if meta.RequestID != "" {
		fields["request_id"] = meta.RequestID
	}
	if meta.RemoteIP != "" {
		fields["remote_ip"] = meta.RemoteIP
	}
	if meta.UserAgent != "" {
		fields["user_agent"] = meta.UserAgent
	}
	return fields
}

func logAuthAudit(ctx context.Context, action, outcome string, extra log.Fields) {
	fields := authAuditFields(ctx, action)
	fields["outcome"] = outcome
	for key, value := range extra {
		fields[key] = value
	}
	log.WithFields(fields).Info("auth audit")
}

// Register creates a new user and returns auth tokens.
func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error) {
	// Check if email already exists
	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		logAuthAudit(ctx, "register", "failure", log.Fields{"email": req.Email, "reason": "email_exists"})
		return nil, ErrEmailAlreadyExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		ID:       uuid.New(),
		Email:    req.Email,
		Password: string(hash),
		Name:     req.Name,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		logAuthAudit(ctx, "register", "failure", log.Fields{"email": req.Email, "reason": "create_user_failed"})
		return nil, fmt.Errorf("create user: %w", err)
	}

	resp, err := s.issueTokens(ctx, user)
	if err != nil {
		logAuthAudit(ctx, "register", "failure", log.Fields{"email": req.Email, "reason": "issue_tokens_failed"})
		return nil, err
	}
	logAuthAudit(ctx, "register", "success", log.Fields{"user_id": user.ID.String(), "email": user.Email})
	return resp, nil
}

// Login validates credentials and returns auth tokens.
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logAuthAudit(ctx, "login", "failure", log.Fields{"email": req.Email, "reason": "invalid_credentials"})
			return nil, ErrInvalidCredentials
		}
		logAuthAudit(ctx, "login", "failure", log.Fields{"email": req.Email, "reason": "load_user_failed"})
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		logAuthAudit(ctx, "login", "failure", log.Fields{"email": req.Email, "reason": "invalid_credentials"})
		return nil, ErrInvalidCredentials
	}

	resp, err := s.issueTokens(ctx, user)
	if err != nil {
		logAuthAudit(ctx, "login", "failure", log.Fields{"email": req.Email, "reason": "issue_tokens_failed"})
		return nil, err
	}
	logAuthAudit(ctx, "login", "success", log.Fields{"user_id": user.ID.String(), "email": user.Email})
	return resp, nil
}

// Refresh validates a refresh token and rotates both tokens.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.AuthResponse, error) {
	// Parse and validate the refresh token (we sign it as a JWT too)
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(refreshToken, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"reason": "invalid_refresh_token"})
		return nil, ErrInvalidToken
	}

	// Verify stored refresh token matches
	stored, err := s.cacheRepo.GetRefreshToken(ctx, claims.UserID)
	if errors.Is(err, repository.ErrCacheUnavailable) {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "refresh_cache_unavailable"})
		return nil, fmt.Errorf("load refresh token: %w", err)
	}
	if err != nil || stored != refreshToken {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "refresh_token_mismatch"})
		return nil, ErrInvalidToken
	}

	// Load user
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "invalid_subject"})
		return nil, ErrInvalidToken
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "user_not_found"})
			return nil, ErrInvalidToken
		}
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "load_user_failed"})
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Delete old refresh token before issuing new ones
	if err := s.cacheRepo.DeleteRefreshToken(ctx, claims.UserID); err != nil {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "delete_refresh_token_failed"})
		return nil, fmt.Errorf("delete refresh token: %w", err)
	}

	resp, err := s.issueTokens(ctx, user)
	if err != nil {
		logAuthAudit(ctx, "refresh", "failure", log.Fields{"user_id": claims.UserID, "reason": "issue_tokens_failed"})
		return nil, err
	}
	logAuthAudit(ctx, "refresh", "success", log.Fields{"user_id": user.ID.String(), "email": user.Email})
	return resp, nil
}

// Logout blacklists the access token JTI and removes the refresh token.
func (s *AuthService) Logout(ctx context.Context, userID, jti string, accessTokenTTL time.Duration) error {
	// Add JTI to blacklist
	if err := s.cacheRepo.BlacklistToken(ctx, jti, accessTokenTTL); err != nil {
		logAuthAudit(ctx, "logout", "failure", log.Fields{"user_id": userID, "reason": "blacklist_failed"})
		return fmt.Errorf("blacklist token: %w", err)
	}
	// Remove refresh token
	if err := s.cacheRepo.DeleteRefreshToken(ctx, userID); err != nil {
		logAuthAudit(ctx, "logout", "failure", log.Fields{"user_id": userID, "reason": "delete_refresh_token_failed"})
		return fmt.Errorf("delete refresh token: %w", err)
	}
	logAuthAudit(ctx, "logout", "success", log.Fields{"user_id": userID, "jti": jti})
	return nil
}

// GetCurrentUser loads the authoritative user profile for the authenticated subject.
func (s *AuthService) GetCurrentUser(ctx context.Context, userID string) (model.UserDTO, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return model.UserDTO{}, ErrInvalidToken
	}

	user, err := s.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.UserDTO{}, ErrInvalidToken
		}
		return model.UserDTO{}, fmt.Errorf("get user: %w", err)
	}

	return user.ToDTO(), nil
}

// issueTokens creates and stores access + refresh tokens for a user.
func (s *AuthService) issueTokens(ctx context.Context, user *model.User) (*model.AuthResponse, error) {
	now := time.Now()
	userID := user.ID.String()

	// Access token
	accessJTI := uuid.New().String()
	accessClaims := &Claims{
		UserID: userID,
		Email:  user.Email,
		JTI:    accessJTI,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTAccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token (also a JWT for self-contained validation)
	refreshJTI := uuid.New().String()
	refreshClaims := &Claims{
		UserID: userID,
		Email:  user.Email,
		JTI:    refreshJTI,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTRefreshTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	// Store refresh token in Redis
	if err := s.cacheRepo.SetRefreshToken(ctx, userID, refreshToken, s.cfg.JWTRefreshTTL); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user.ToDTO(),
	}, nil
}

// UpdateProfile updates the user's display name and returns the updated profile.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req *model.UpdateUserRequest) (model.UserDTO, error) {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return model.UserDTO{}, ErrInvalidToken
	}
	if err := s.userRepo.UpdateName(ctx, parsedID, req.Name); err != nil {
		return model.UserDTO{}, fmt.Errorf("update profile: %w", err)
	}
	user, err := s.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		return model.UserDTO{}, fmt.Errorf("reload user: %w", err)
	}
	return user.ToDTO(), nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, userID string, currentPassword, newPassword, currentJTI string, accessTokenTTL time.Duration) error {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "invalid_token"})
		return ErrInvalidToken
	}
	user, err := s.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "user_not_found"})
			return ErrInvalidToken
		}
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "load_user_failed"})
		return fmt.Errorf("get user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "incorrect_current_password"})
		return ErrCurrentPasswordIncorrect
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "hash_password_failed"})
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, parsedID, string(hash)); err != nil {
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "update_password_failed"})
		return fmt.Errorf("update password: %w", err)
	}
	if strings.TrimSpace(currentJTI) != "" && accessTokenTTL > 0 {
		if err := s.cacheRepo.BlacklistToken(ctx, currentJTI, accessTokenTTL); err != nil {
			logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "blacklist_current_token_failed"})
			return fmt.Errorf("blacklist current token: %w", err)
		}
	}
	if err := s.cacheRepo.DeleteRefreshToken(ctx, userID); err != nil {
		logAuthAudit(ctx, "change_password", "failure", log.Fields{"user_id": userID, "reason": "delete_refresh_token_failed"})
		return fmt.Errorf("delete refresh token: %w", err)
	}
	logAuthAudit(ctx, "change_password", "success", log.Fields{"user_id": userID, "jti": currentJTI})
	return nil
}

// Sentinel errors
var (
	ErrEmailAlreadyExists       = errors.New("email already exists")
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrInvalidToken             = errors.New("invalid or expired token")
	ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
)
