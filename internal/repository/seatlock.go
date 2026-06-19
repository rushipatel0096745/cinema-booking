package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SeatLockRepository interface {
	LockSeats(ctx context.Context, showtimeID string, seatIDs []string, userID string, ttl time.Duration) error
	ReleaseSeats(ctx context.Context, showtimeID string, seatIDs []string) error
	IsLocked(ctx context.Context, showtimeID string, seatID string) (bool, error)
	GetLockOwner(ctx context.Context, showtimeID string, seatID string) (string, error)
}

type seatLockRepository struct {
	redis *redis.Client
}

func NewSeatLockRepository(redis *redis.Client) SeatLockRepository {
	return &seatLockRepository{redis: redis}
}

func (r *seatLockRepository) key(showtimeID, seatID string) string {
	return fmt.Sprintf("seatlock:%s:%s", showtimeID, seatID)
}

func (r *seatLockRepository) LockSeats(ctx context.Context, showtimeID string, seatIDs []string, userID string, ttl time.Duration) error {
	pipe := r.redis.Pipeline()

	for _, seatID := range seatIDs {
		pipe.SetNX(ctx, r.key(showtimeID, seatID), userID, ttl)
	}

	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	for _, cmd := range cmds {
		if boolCmd, ok := cmd.(*redis.BoolCmd); ok {
			okVal, _ := boolCmd.Result()
			if !okVal {
				return fmt.Errorf("one or more seats already locked")
			}
		}
	}
	return nil
}

func (r *seatLockRepository) ReleaseSeats(ctx context.Context, showtimeID string, seatIDs []string) error {
	pipe := r.redis.Pipeline()
	for _, seatID := range seatIDs {
		pipe.Del(ctx, r.key(showtimeID, seatID))
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *seatLockRepository) IsLocked(ctx context.Context, showtimeID string, seatID string) (bool, error) {
	n, err := r.redis.Exists(ctx, r.key(showtimeID, seatID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *seatLockRepository) GetLockOwner(ctx context.Context, showtimeID string, seatID string) (string, error) {
	val, err := r.redis.Get(ctx, r.key(showtimeID, seatID)).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}
