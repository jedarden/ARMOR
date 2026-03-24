// Package b2keys provides B2 application key management via the B2 native API.
package b2keys

import (
	"context"
	"time"

	"github.com/kurin/blazer/b2"
)

// KeyInfo contains information about a B2 application key.
type KeyInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Expires      string   `json:"expires,omitempty"`
	Secret       string   `json:"secret,omitempty"` // Only populated on create
}

// CreateKeyRequest is the request body for creating a new B2 application key.
type CreateKeyRequest struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Prefix       string   `json:"prefix,omitempty"`
	Duration     string   `json:"duration,omitempty"` // e.g., "1h", "24h", "168h" (7 days)
}

// ListKeysResponse is the response for listing B2 application keys.
type ListKeysResponse struct {
	Keys       []KeyInfo `json:"keys"`
	NextCursor string    `json:"nextCursor,omitempty"`
}

// Client wraps the B2 client for key management operations.
type Client struct {
	client *b2.Client
}

// NewClient creates a new B2 key management client.
func NewClient(ctx context.Context, accountID, applicationKey string) (*Client, error) {
	client, err := b2.NewClient(ctx, accountID, applicationKey)
	if err != nil {
		return nil, err
	}
	return &Client{client: client}, nil
}

// ListKeys lists B2 application keys.
func (c *Client) ListKeys(ctx context.Context, count int, cursor string) (*ListKeysResponse, error) {
	if count <= 0 {
		count = 100
	}

	keys, nextCursor, err := c.client.ListKeys(ctx, count, cursor)
	if err != nil {
		return nil, err
	}

	result := &ListKeysResponse{
		NextCursor: nextCursor,
		Keys:       make([]KeyInfo, len(keys)),
	}

	for i, k := range keys {
		result.Keys[i] = KeyInfo{
			ID:           k.ID(),
			Name:         k.Name(),
			Capabilities: k.Capabilities(),
		}
		if !k.Expires().IsZero() {
			result.Keys[i].Expires = k.Expires().Format(time.RFC3339)
		}
	}

	return result, nil
}

// CreateKey creates a new B2 application key.
func (c *Client) CreateKey(ctx context.Context, req *CreateKeyRequest) (*KeyInfo, error) {
	var opts []b2.KeyOption

	// Add capabilities
	if len(req.Capabilities) > 0 {
		opts = append(opts, b2.Capabilities(req.Capabilities...))
	}

	// Add prefix restriction
	if req.Prefix != "" {
		opts = append(opts, b2.Prefix(req.Prefix))
	}

	// Add lifetime/duration
	if req.Duration != "" {
		if d, err := time.ParseDuration(req.Duration); err == nil {
			opts = append(opts, b2.Lifetime(d))
		}
	}

	key, err := c.client.CreateKey(ctx, req.Name, opts...)
	if err != nil {
		return nil, err
	}

	info := &KeyInfo{
		ID:           key.ID(),
		Name:         key.Name(),
		Capabilities: key.Capabilities(),
		Secret:       key.Secret(),
	}

	if !key.Expires().IsZero() {
		info.Expires = key.Expires().Format(time.RFC3339)
	}

	return info, nil
}

// DeleteKey deletes a B2 application key by ID.
// Note: The blazer library requires listing keys to get the Key object for deletion.
func (c *Client) DeleteKey(ctx context.Context, keyID string) error {
	// List all keys to find the one with matching ID
	keys, _, err := c.client.ListKeys(ctx, 1000, "")
	if err != nil {
		return err
	}

	for _, k := range keys {
		if k.ID() == keyID {
			return k.Delete(ctx)
		}
	}

	return ErrKeyNotFound
}

// ErrKeyNotFound is returned when a key cannot be found.
var ErrKeyNotFound = &KeyNotFoundError{}

// KeyNotFoundError indicates the requested key was not found.
type KeyNotFoundError struct{}

func (e *KeyNotFoundError) Error() string {
	return "key not found"
}
