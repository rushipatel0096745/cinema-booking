package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"cinemabooking/internal/config"
	"cinemabooking/internal/domain"
	"cinemabooking/internal/pkg/mailer"
	repositories "cinemabooking/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrGoogleProvider     = errors.New("this account uses Google login")
)

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type AuthService struct {
	repo                  *repositories.UserRepository
	config                *config.Config
	emailVerificationRepo repositories.EmailVerificationRepository
	mailer                *mailer.Service
}

func NewAuthService(repo *repositories.UserRepository, cfg *config.Config, emailVerificationRepo repositories.EmailVerificationRepository, mailer *mailer.Service) *AuthService {
	return &AuthService{repo: repo, config: cfg, emailVerificationRepo: emailVerificationRepo, mailer: mailer}
}

// Register creates a new email/password user
func (s *AuthService) Register(ctx context.Context, req *domain.RegisterRequest) (*domain.AuthResponse, error) {
	// Check if email is taken
	_, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &domain.User{
		Email:        req.Email,
		Name:         req.Name,
		Role:         domain.RoleUser,
		PasswordHash: string(hash),
		Phone:        req.Phone,
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return s.issueTokenPair(ctx, user)
}

// Login authenticates an email/password user
func (s *AuthService) Login(ctx context.Context, req *domain.LoginRequest) (*domain.AuthResponse, error) {
	user, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Prevent Google-only users from using password login
	if user.GoogleID != "" && user.PasswordHash == "" {
		return nil, ErrGoogleProvider
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueTokenPair(ctx, user)
}

// HandleGoogleUser upserts a Google user and issues tokens
func (s *AuthService) HandleGoogleUser(ctx context.Context, googleUser *domain.User) (*domain.AuthResponse, error) {
	user, err := s.repo.UpsertGoogleUser(ctx, googleUser)
	if err != nil {
		return nil, fmt.Errorf("upserting google user: %w", err)
	}
	return s.issueTokenPair(ctx, user)
}

func (s *AuthService) GetUserProfile(ctx context.Context, userID string) (*domain.UserProfile, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &domain.UserProfile{
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		AvatarURL: user.AvatarURL,
	}, nil
}

// RefreshTokens validates a refresh token and issues a new pair (rotation)
func (s *AuthService) RefreshTokens(ctx context.Context, rawRefreshToken string) (*domain.AuthResponse, error) {
	tokenHash := hashToken(rawRefreshToken)

	rt, err := s.repo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if time.Now().After(rt.ExpiresAt) {
		_ = s.repo.DeleteRefreshToken(ctx, tokenHash)
		return nil, ErrInvalidToken
	}

	// Rotate: delete old, issue new
	_ = s.repo.DeleteRefreshToken(ctx, tokenHash)

	user, err := s.repo.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, user)
}

// Logout invalidates the refresh token
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	hash := hashToken(refreshToken)

	if err := s.repo.DeleteRefreshToken(ctx, hash); err != nil {
		return fmt.Errorf("deleting refresh token: %w", err)
	}

	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	if err := s.repo.DeleteAllRefreshTokens(ctx, userID); err != nil {
		return fmt.Errorf("deleting all refresh tokens: %w", err)
	}

	return nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req domain.UpdateProfileRequest) (*domain.User, error) {
	if req.Name == "" && req.Phone == "" {
		return nil, domain.NewAppError(400, "at least one field required")
	}

	user, err := s.repo.UpdateProfile(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("updating profile: %w", err)
	}

	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID string, req domain.ChangePasswordRequest) error {
	// confirm passwords match
	if req.NewPassword != req.ConfirmPassword {
		return domain.NewAppError(400, "passwords do not match")
	}

	// load user to verify current password
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Google OAuth users have no password
	if user.PasswordHash == "" {
		return domain.NewAppError(400, "account uses Google sign-in — password cannot be changed")
	}

	// verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return domain.NewAppError(401, "current password is incorrect")
	}

	// prevent reusing the same password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.NewPassword)); err == nil {
		return domain.NewAppError(400, "new password must be different from current password")
	}

	// hash new password
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.repo.UpdatePassword(ctx, userID, string(hashed))
}

// Updating email with email OTP based with redis
func (s *AuthService) UpdateEmail(ctx context.Context, userID string, newEmail string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.Email == newEmail {
		return domain.NewAppError(400, "new email must be different from current email")
	}

	existing, err := s.repo.FindByEmail(ctx, newEmail)
	if err == nil && existing != nil {
		return domain.NewAppError(409, "email already in use")
	}

	code := generateVerificationCode()

	if err := s.emailVerificationRepo.SetOTP(ctx, userID, newEmail, code, 15*time.Minute); err != nil {
		return err
	}

	return s.mailer.SendVerificationCode(ctx, newEmail, code)
}

func (s *AuthService) VerifyEmailChange(ctx context.Context, userID, code string) error {
	newEmail, storedCode, err := s.emailVerificationRepo.GetOTP(ctx, userID)
	if err != nil {
		return domain.NewAppError(400, err.Error())
	}

	if storedCode != code {
		return domain.NewAppError(400, "invalid verification code")
	}

	// Delete first — prevent reuse even if UpdateEmail fails
	if err := s.emailVerificationRepo.DeleteOTP(ctx, userID); err != nil {
		return err
	}

	return s.repo.UpdateEmail(ctx, userID, newEmail)
}

// ValidateAccessToken parses and validates a JWT access token
func (s *AuthService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// — private helpers —

func (s *AuthService) issueTokenPair(ctx context.Context, user *domain.User) (*domain.AuthResponse, error) {
	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, err
	}

	rawRefresh, err := s.generateRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		User:         *user,
	}, nil
}

func (s *AuthService) generateAccessToken(user *domain.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(s.config.JWTExpiry) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *AuthService) generateRefreshToken(ctx context.Context, userID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rawToken := base64.URLEncoding.EncodeToString(b)

	rt := &domain.RefreshToken{
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().AddDate(0, 0, s.config.RefreshExpiry), // setting expiry in date
	}
	if err := s.repo.SaveRefreshToken(ctx, rt); err != nil {
		return "", err
	}
	return rawToken, nil
}

// hashToken SHA-256 hashes a token before storing — never store raw refresh tokens
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateVerificationCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return fmt.Sprintf("%06d", n)
}
