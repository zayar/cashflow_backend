package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

type SignedUpload struct {
	UploadURL string            `json:"uploadUrl"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ObjectKey string            `json:"objectKey"`
	AccessURL string            `json:"accessUrl"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type serviceAccountJSON struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
}

func SignUpload(ctx context.Context, objectKey, contentType string, expires time.Duration) (*SignedUpload, error) {
	if GetStorageProvider() != StorageProviderGCS {
		return nil, fmt.Errorf("storage provider %q is not supported for signed uploads", GetStorageProvider())
	}

	bucket := strings.TrimSpace(os.Getenv("GCS_BUCKET"))
	if bucket == "" {
		return nil, errors.New("GCS_BUCKET is required")
	}

	opts := &storage.SignedURLOptions{
		Scheme:      storage.SigningSchemeV4,
		Method:      "PUT",
		Expires:     time.Now().Add(expires),
		ContentType: contentType,
	}

	accessID, privateKey, ok, err := loadSignerFromEnv()
	if err != nil {
		return nil, err
	}
	if ok {
		opts.GoogleAccessID = accessID
		opts.PrivateKey = privateKey
	} else {
		email, signBytes, err := iamSigner(ctx)
		if err != nil {
			return nil, err
		}
		opts.GoogleAccessID = email
		opts.SignBytes = signBytes
	}

	signedURL, err := storage.SignedURL(bucket, objectKey, opts)
	if err != nil {
		return nil, err
	}

	return &SignedUpload{
		UploadURL: signedURL,
		Method:    opts.Method,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		ObjectKey: objectKey,
		AccessURL: BuildObjectAccessURL(objectKey),
		ExpiresAt: opts.Expires,
	}, nil
}

func loadSignerFromEnv() (string, []byte, bool, error) {
	credJSON := strings.TrimSpace(os.Getenv("GCS_CREDENTIALS_JSON"))
	if credJSON != "" {
		var key serviceAccountJSON
		if err := json.Unmarshal([]byte(credJSON), &key); err != nil {
			return "", nil, false, fmt.Errorf("invalid GCS_CREDENTIALS_JSON: %w", err)
		}
		if key.ClientEmail == "" || key.PrivateKey == "" {
			return "", nil, false, errors.New("GCS_CREDENTIALS_JSON missing client_email or private_key")
		}
		return key.ClientEmail, normalizePrivateKey(key.PrivateKey), true, nil
	}

	email := strings.TrimSpace(os.Getenv("GCS_SIGNER_EMAIL"))
	privateKey := strings.TrimSpace(os.Getenv("GCS_SIGNER_PRIVATE_KEY"))
	if email == "" || privateKey == "" {
		return "", nil, false, nil
	}
	return email, normalizePrivateKey(privateKey), true, nil
}

func normalizePrivateKey(key string) []byte {
	key = strings.ReplaceAll(key, "\\n", "\n")
	return []byte(key)
}

func iamSigner(ctx context.Context) (string, func([]byte) ([]byte, error), error) {
	email := strings.TrimSpace(os.Getenv("GCS_SIGNER_EMAIL"))
	if email == "" {
		if metadata.OnGCE() {
			defaultEmail, err := metadata.Email("default")
			if err != nil {
				return "", nil, fmt.Errorf("failed to get default service account email: %w", err)
			}
			email = defaultEmail
		}
	}
	if email == "" {
		return "", nil, errors.New("GCS_SIGNER_EMAIL is required when no private key is provided")
	}

	creds, err := google.FindDefaultCredentials(ctx, iamcredentials.CloudPlatformScope)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load ADC credentials: %w", err)
	}
	svc, err := iamcredentials.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create iamcredentials service: %w", err)
	}

	resource := fmt.Sprintf("projects/-/serviceAccounts/%s", email)
	signBytes := func(data []byte) ([]byte, error) {
		req := &iamcredentials.SignBlobRequest{
			Payload: base64.StdEncoding.EncodeToString(data),
		}
		resp, err := svc.Projects.ServiceAccounts.SignBlob(resource, req).Do()
		if err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(resp.SignedBlob)
	}

	return email, signBytes, nil
}
