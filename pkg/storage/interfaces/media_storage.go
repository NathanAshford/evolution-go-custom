package storage_interfaces

import "context"

// MediaStorage defines the contract for storing and retrieving media files
type MediaStorage interface {
	// Store saves the media data and returns a public URL to access it
	Store(ctx context.Context, data []byte, fileName string, contentType string) (string, error)

	// Delete removes the stored media
	Delete(ctx context.Context, fileName string) error

	// GetURL returns the public URL for accessing the media
	GetURL(ctx context.Context, fileName string) (string, error)
}
