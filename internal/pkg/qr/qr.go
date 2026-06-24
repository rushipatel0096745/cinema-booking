package qr

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	// "time"

	goqr "github.com/skip2/go-qrcode"
)

type QrService struct {
	secret string // QR_SECRET env var
}

func NewQrService(secret string) *QrService {
	return &QrService{secret: secret}
}

type Payload struct {
	BookingID  string `json:"bid"`
	UserID     string `json:"uid"`
	ShowtimeID string `json:"sid"`
	ExpiresAt  int64  `json:"exp"`
	Signature  string `json:"sig"`
}

func (s *QrService) Generate(bookingID, userID, showtimeID string, expiresAt int64) ([]byte, error) {
	sig := s.sign(bookingID, userID, showtimeID, expiresAt)
	slog.Info("qr-debug", "bookingID", bookingID, "userID", userID, "showtimeID", showtimeID, "expiresAt", expiresAt, "signature", sig)
	// fmt.Println("signature", sig)
	payload := Payload{
		BookingID:  bookingID,
		UserID:     userID,
		ShowtimeID: showtimeID,
		ExpiresAt:  expiresAt,
		Signature:  sig,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshalling qr payload: %w", err)
	}

	// generate 512x512 PNG
	png, err := goqr.Encode(string(data), goqr.Medium, 512)
	if err != nil {
		return nil, fmt.Errorf("encoding qr: %w", err)
	}

	return png, nil
}

func (s *QrService) Verify(payload Payload) bool {
	slog.Info("verify qr endpoint hit..........")
	if time.Now().Unix() > payload.ExpiresAt {
		return false
	}
	expected := s.sign(payload.BookingID, payload.UserID, payload.ShowtimeID, payload.ExpiresAt)

	slog.Info(
		"verify-debug",
		"bookingID", payload.BookingID,
		"userID", payload.UserID,
		"showtimeID", payload.ShowtimeID,
		"expiresAt", payload.ExpiresAt,
		"provided", payload.Signature,
		"expected", expected,
	)

	return hmac.Equal([]byte(payload.Signature), []byte(expected))
}

func (s *QrService) sign(bookingID, userID, showtimeID string, expiresAt int64) string {
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(fmt.Sprintf("%s:%s:%s:%d", bookingID, userID, showtimeID, expiresAt)))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateReader returns an io.Reader for upload
func (s *QrService) GenerateReader(bookingID, userID, showtimeID string, expiresAt int64) (*bytes.Reader, error) {
	png, err := s.Generate(bookingID, userID, showtimeID, expiresAt)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(png), nil
}
