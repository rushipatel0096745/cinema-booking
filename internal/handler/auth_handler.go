package handlers

import (
	"cinemabooking/internal/config"
	domain "cinemabooking/internal/domain"
	repositories "cinemabooking/internal/repository"
	services "cinemabooking/internal/service"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type AuthHandler struct {
	authService *services.AuthService
	oauthConfig *oauth2.Config
	userRepo    *repositories.UserRepository
}

func NewAuthHandler(authService *services.AuthService, cfg *config.Config, userRepo *repositories.UserRepository) *AuthHandler {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	return &AuthHandler{authService: authService, oauthConfig: oauthCfg, userRepo: userRepo}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(
			http.StatusBadRequest,
			domain.Fail[any](err.Error()),
		)
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrEmailTaken) {
			c.JSON(
				http.StatusConflict,
				domain.Fail[any]("email already registered"),
			)
			return
		}

		c.JSON(
			http.StatusInternalServerError,
			// domain.Fail[any]("registration failed"),
			domain.Fail[any](err.Error()),
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		domain.OK(resp),
	)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, domain.Fail[any]("invalid email or password"))
			return
		}
		if errors.Is(err, services.ErrGoogleProvider) {
			c.JSON(http.StatusUnauthorized, domain.Fail[any]("this account uses Google login"))
			return
		}

		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.OK(resp))
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	// state is a CSRF token — in production, store this in a short-lived server-side session or signed cookie
	state := "random-state-value" // TODO: generate & store per-request
	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	// TODO: validate state matches what you stored
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, domain.Fail[any]("missing code"))
		return
	}

	token, err := h.oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any]("failed to exchange code"))
		return
	}

	// Fetch user info from Google
	client := h.oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any]("failed to fetch user info"))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any]("failed to read user info"))
		return
	}

	var googleUser struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Picture  string `json:"picture"`
		Verified bool   `json:"verified_email"`
	}
	if err := json.Unmarshal(body, &googleUser); err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any]("failed to parse user info"))
		return
	}

	user := &domain.User{
		Email:     googleUser.Email,
		Name:      googleUser.Name,
		AvatarURL: googleUser.Picture,
		Role:      domain.RoleUser,
		GoogleID:  googleUser.ID,
		Phone:     "",
	}

	authResp, err := h.authService.HandleGoogleUser(c.Request.Context(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}

	// For mobile apps: redirect with tokens in query params (use a custom scheme)
	// For web: redirect to frontend with tokens
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf(
		"%s/auth/callback?access_token=%s&refresh_token=%s",
		"yourapp://", authResp.AccessToken, authResp.RefreshToken,
	))
	// OR just return JSON if this is a pure API:
	c.JSON(http.StatusOK, domain.OK(authResp))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req domain.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	resp, err := h.authService.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.Fail[any]("invalid or expired refresh token"))
		return
	}

	c.JSON(http.StatusOK, domain.OK(resp))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req domain.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := h.authService.LogoutAll(c.Request.Context(), userID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

func (h *AuthHandler) GetUserProfile(c *gin.Context) {
	userId := c.GetString("user_id")
	user, err := h.authService.GetUserProfile(c.Request.Context(), userId)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("user not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.OK(user))
}

// PUT /api/v1/users/profile
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req domain.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK(user.PublicProfile()))
}

// PUT /api/v1/users/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")

	var req domain.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID, req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

// PUT /api/v1/users/email
func (h *AuthHandler) UpdateEmail(c *gin.Context) {
	userID := c.GetString("user_id")

	var req domain.UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	if err := h.authService.UpdateEmail(c.Request.Context(), userID, req.Email); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

func (h *AuthHandler) VerifyEmailChange(c *gin.Context) {
	userID := c.GetString("user_id")

	var req domain.VerifyEmailChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.Fail[any](err.Error()))
		return
	}

	if err := h.authService.VerifyEmailChange(c.Request.Context(), userID, req.Code); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, domain.OK[any](nil))
}

func (h *AuthHandler) Me(c *gin.Context) {
	// UserID is set by the auth middleware
	userID := c.GetString("user_id")
	user, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			c.JSON(http.StatusNotFound, domain.Fail[any]("user not found"))
			return
		}
		c.JSON(http.StatusInternalServerError, domain.Fail[any](err.Error()))
		return
	}

	c.JSON(http.StatusOK, domain.OK(user))
}
