package models

import "time"

type FileInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mimeType"`
	UserID    string    `json:"userId"`
	Checksum  string    `json:"checksum,omitempty"`
	IsDir     bool      `json:"isDir"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UploadResponse struct {
	File  *FileInfo `json:"file"`
	URL   string    `json:"url,omitempty"`
}

type ListResponse struct {
	Files    []*FileInfo `json:"files"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
}
