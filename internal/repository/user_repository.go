package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cinemabooking/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")
var ErrMovieInUse = errors.New("movie is in use")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) scanUser(row pgx.Row) (*domain.User, error) {
	user := &domain.User{}
	var phone, avatarURL, passwordHash, googleID pgtype.Text

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&phone,
		&avatarURL,
		&passwordHash,
		&googleID,
		&user.Role,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Phone = phone.String
	user.AvatarURL = avatarURL.String
	user.PasswordHash = passwordHash.String
	user.GoogleID = googleID.String

	return user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	var (
		phone        pgtype.Text
		avatarURL    pgtype.Text
		passwordHash pgtype.Text
		googleID     pgtype.Text
	)
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, phone, avatar_url, password_hash, google_id, role, created_at
         FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.Name, &phone, &avatarURL, &passwordHash, &googleID, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Phone = phone.String
	user.AvatarURL = avatarURL.String
	user.PasswordHash = passwordHash.String
	user.GoogleID = googleID.String

	return user, nil
}

func (r *UserRepository) FindByGoogleID(ctx context.Context, googleIDVal string) (*domain.User, error) {
	user := &domain.User{}
	var (
		phone        pgtype.Text
		avatarURL    pgtype.Text
		passwordHash pgtype.Text
		googleID     pgtype.Text
	)
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, phone, avatar_url, password_hash, google_id, role, created_at
         FROM users WHERE google_id = $1`, googleIDVal,
	).Scan(&user.ID, &user.Email, &user.Name, &phone, &avatarURL, &passwordHash, &googleID, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Phone = phone.String
	user.AvatarURL = avatarURL.String
	user.PasswordHash = passwordHash.String
	user.GoogleID = googleID.String

	return user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	user := &domain.User{}
	var (
		phone        pgtype.Text
		avatarURL    pgtype.Text
		passwordHash pgtype.Text
		googleID     pgtype.Text
	)
	err := r.db.QueryRow(ctx,
		`SELECT id, email, name, phone, avatar_url, password_hash, google_id, role, created_at
         FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.Name, &phone, &avatarURL, &passwordHash, &googleID, &user.Role, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Phone = phone.String
	user.AvatarURL = avatarURL.String
	user.PasswordHash = passwordHash.String
	user.GoogleID = googleID.String

	return user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.QueryRow(ctx,
		`INSERT INTO users (email, name, phone, avatar_url, password_hash, google_id, role)
         VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
         RETURNING id, created_at`,
		user.Email, user.Name, user.Phone, user.AvatarURL, user.PasswordHash, user.GoogleID, user.Role,
	).Scan(&user.ID, &user.CreatedAt)
}

func (r *UserRepository) UpdateProfile(ctx context.Context, userID string, req domain.UpdateProfileRequest) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `
        UPDATE users
        SET
            name   = CASE WHEN $1 != '' THEN $1 ELSE name END,
            phone  = CASE WHEN $2 != '' THEN $2 ELSE phone END
        WHERE id = $3
        RETURNING id, email, name, phone, avatar_url, password_hash, google_id, role, created_at
    `, req.Name, req.Phone, userID)

	return r.scanUser(row)
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID string, hashedPassword string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		hashedPassword, userID,
	)
	return err
}

func (r *UserRepository) UpdateEmail(ctx context.Context, userID string, email string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET email = $1 WHERE id = $2`,
		email, userID,
	)
	return err
}

// UpsertGoogleUser handles both new Google users and existing ones
// func (r *UserRepository) UpsertGoogleUser(ctx context.Context, user *domain.User) (*domain.User, error) {
// 	result := &domain.User{}
// 	err := r.db.QueryRow(ctx,
// 		`INSERT INTO users (email, name, phone, avatar_url, google_id, role)
//          VALUES ($1, $2, $3, $4, $5, $6)
//          ON CONFLICT (email) DO UPDATE SET
//              name = EXCLUDED.name,
//              avatar_url = EXCLUDED.avatar_url,
//              google_id = EXCLUDED.google_id,
//          RETURNING id, email, name, phone, avatar_url, password_hash, google_id, role, created_at`,
// 		user.Email, user.Name, user.Phone, user.AvatarURL, user.GoogleID, user.Role,
// 	).Scan(&result.ID, &result.Email, &result.Name, &result.Phone, &result.AvatarURL,
// 		&result.PasswordHash, &result.GoogleID, &result.Role, &result.CreatedAt)
// 	return result, err
// }

func (r *UserRepository) UpsertGoogleUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	result := &domain.User{}

	// nullable fields that pgx can't scan into *string directly
	var (
		passwordHash pgtype.Text
		phone        pgtype.Text
		googleID     pgtype.Text
		avatarURL    pgtype.Text
	)

	query := `
        INSERT INTO users (email, name, phone, avatar_url, google_id, role)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (email)
        DO UPDATE SET
            name       = COALESCE(NULLIF(EXCLUDED.name, ''), users.name),
            avatar_url = COALESCE(NULLIF(EXCLUDED.avatar_url, ''), users.avatar_url),
            google_id  = EXCLUDED.google_id
        RETURNING id, email, name, phone, avatar_url, password_hash, google_id, role, created_at
    `

	err := r.db.QueryRow(ctx, query,
		user.Email,
		user.Name,
		user.Phone,
		user.AvatarURL,
		user.GoogleID,
		user.Role,
	).Scan(
		&result.ID,
		&result.Email,
		&result.Name,
		&phone,
		&avatarURL,
		&passwordHash,
		&googleID,
		&result.Role,
		&result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upserting google user: %w", err)
	}

	// unwrap nullable fields — empty string if NULL
	result.PasswordHash = passwordHash.String
	result.Phone = phone.String
	result.AvatarURL = avatarURL.String
	result.GoogleID = googleID.String

	return result, nil
}

// Refresh token methods
func (r *UserRepository) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		rt.UserID, rt.TokenHash, rt.ExpiresAt,
	)
	return err
}

func (r *UserRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	rt := &domain.RefreshToken{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rt, nil
}

func (r *UserRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *UserRepository) DeleteAllRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}

func (r *UserRepository) CleanExpiredTokens(ctx context.Context) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < $1`, time.Now())
	return err
}
