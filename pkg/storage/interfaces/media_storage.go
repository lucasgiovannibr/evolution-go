package storage_interfaces

import "context"

// MediaStorage defines the contract for storing and retrieving media files
type MediaStorage interface {
	// Store saves a media file of an instance and returns a (presigned) URL to access it
	Store(ctx context.Context, instanceID string, data []byte, fileName string, contentType string) (string, error)

	// Delete removes one stored media file of an instance
	Delete(ctx context.Context, instanceID string, fileName string) error

	// GetURL returns a URL for accessing a stored media file of an instance
	GetURL(ctx context.Context, instanceID string, fileName string) (string, error)

	// DeleteInstance removes every media file of an instance and returns how many
	DeleteInstance(ctx context.Context, instanceID string) (int, error)
}
