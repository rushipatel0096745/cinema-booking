package domain

import "time"

// Role constants
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User represents an application user (both regular and admin).
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Phone        string    `json:"phone,omitempty"`
	PasswordHash string    `json:"-"` // never serialised
	GoogleID     string   `json:"-"`
	AvatarURL    string   `json:"avatar_url,omitempty"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// IsAdmin returns true when the user holds the admin role.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// PublicProfile returns a copy safe to expose in API responses.
func (u *User) PublicProfile() UserProfile {
	avatarURL := ""
	if u.AvatarURL != "" {
		avatarURL = u.AvatarURL
	}
	return UserProfile{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: avatarURL,
		Role:      u.Role,
	}
}

// UserProfile is the public-facing subset of User.
type UserProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role"`
}

// RefreshToken stores a hashed refresh token tied to a user.
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired reports whether the token has passed its expiry time.
func (r *RefreshToken) IsExpired() bool { return time.Now().After(r.ExpiresAt) }

// ──────────────────────────────────────────────
// Request / Response DTOs
// ──────────────────────────────────────────────

type RegisterRequest struct {
	Name     string `json:"name"     binding:"required,min=2,max=100"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Phone    string `json:"phone"    binding:"omitempty,e164"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name"  binding:"omitempty,min=2,max=100"`
	Phone string `json:"phone" binding:"omitempty,e164"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
