package models

import "time"

type File struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	OriginalName string    `json:"original_name"`
	StoragePath  string    `json:"-"` // Never expose storage path to client
	MIMEType     string    `json:"mime_type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
}
