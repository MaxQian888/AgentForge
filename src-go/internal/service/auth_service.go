// Package service implements business logic for authentication and user management.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/config"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

// Register creates a new user and returns auth tokens.
func (s *AuthService) Register(ctx context.Context, req *model.RegisterRequest) (*model.AuthResponse, error) {
	// Check if email already exists
	existing, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
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
		return nil, fmt.Errorf("create user: %w", err)
	}

	return s.issueTokens(ctx, user)
}

// Login validates credentials and returns auth tokens.
func (s *AuthService) Login(ctx context.Context, req *model.LoginRequest) (*model.AuthResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokens(ctx, user)
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
		return nil, ErrInvalidToken
	}

	// Verify stored refresh token matches
	stored, err := s.cacheRepo.GetRefreshToken(ctx, claims.UserID)
	if errors.Is(err, repository.ErrCacheUnavailable) {
		return nil, fmt.Errorf("load refresh token: %w", err)
	}
	if err != nil || stored != refreshToken {
		return nil, ErrInvalidToken
	}

	// Load user
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Delete old refresh token before issuing new ones
	if err := s.cacheRepo.DeleteRefreshToken(ctx, claims.UserID); err != nil {
		return nil, fmt.Errorf("delete refresh token: %w", err)
	}

	return s.issueTokens(ctx, user)
}

// Logout blacklists the access token JTI and removes the refresh token.
func (s *AuthService) Logout(ctx context.Context, userID, jti string, accessTokenTTL time.Duration) error {
	// Add JTI to blacklist
	if err := s.cacheRepo.BlacklistToken(ctx, jti, accessTokenTTL); err != nil {
		return fmt.Errorf("blacklist token: %w", err)
	}
	// Remove refresh token
	if err := s.cacheRepo.DeleteRefreshToken(ctx, userID); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
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
func (s *AuthService) ChangePassword(ctx context.Context, userID string, currentPassword, newPassword string) error {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return ErrInvalidToken
	}
	user, err := s.userRepo.GetByID(ctx, parsedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidToken
		}
		return fmt.Errorf("get user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return ErrCurrentPasswordIncorrect
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, parsedID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

// Sentinel errors
var (
	ErrEmailAlreadyExists       = errors.New("email already exists")
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrInvalidToken             = errors.New("invalid or expired token")
	ErrCurrentPasswordIncorrect = errors.New("current password is incorrect")
)
