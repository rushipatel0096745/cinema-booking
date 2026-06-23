package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type Service struct {
	cld *cloudinary.Cloudinary
}

func New(cloudName, apiKey, apiSecret string) (*Service, error) {
	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return nil, fmt.Errorf("storage: cloudName, apiKey and apiSecret are required")
	}

	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("storage: initializing cloudinary client: %w", err)
	}

	return &Service{cld: cld}, nil
}

func (s *Service) UploadQR(ctx context.Context, bookingID string, data []byte) (string, error) {
	// Use same folder structure as your old R2 key: qr/<bookingID>
	publicID := fmt.Sprintf("qr/%s", bookingID)

	overwrite := true

	resp, err := s.cld.Upload.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{
		PublicID:     publicID,
		Overwrite:    &overwrite, // replace on re-upload for same booking
		ResourceType: "image",
		Format:       "png",
	})
	if err != nil {
		return "", fmt.Errorf("storage: uploading qr to cloudinary: %w", err)
	}
	if resp.Error.Message != "" {
		return "", fmt.Errorf("storage: cloudinary error: %s", resp.Error.Message)
	}

	return resp.SecureURL, nil
}
