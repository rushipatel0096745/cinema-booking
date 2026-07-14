package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type EmailVerificationRepository interface {
	SetOTP(ctx context.Context, userID, newEmail, code string, ttl time.Duration) error
	GetOTP(ctx context.Context, userID string) (newEmail, code string, err error)
	DeleteOTP(ctx context.Context, userID string) error
}

type emailVerificationRepository struct {
	redis *redis.Client
}

func NewEmailVerificationRepository(redis *redis.Client) EmailVerificationRepository {
	return &emailVerificationRepository{redis: redis}
}

func (r *emailVerificationRepository) key(userID string) string {
	return fmt.Sprintf("email_change:%s", userID)
}

func (r *emailVerificationRepository) SetOTP(ctx context.Context, userID, newEmail, code string, ttl time.Duration) error {
	payload := fmt.Sprintf("%s|%s", newEmail, code)
	return r.redis.Set(ctx, r.key(userID), payload, ttl).Err()
}

func (r *emailVerificationRepository) GetOTP(ctx context.Context, userID string) (string, string, error) {
	val, err := r.redis.Get(ctx, r.key(userID)).Result()
	if err == redis.Nil {
		return "", "", fmt.Errorf("no pending email change or code expired")
	}
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed verification data")
	}

	return parts[0], parts[1], nil // newEmail, code
}

func (r *emailVerificationRepository) DeleteOTP(ctx context.Context, userID string) error {
	return r.redis.Del(ctx, r.key(userID)).Err()
}
