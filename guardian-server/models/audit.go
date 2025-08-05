package models

import "time"

type AuditLog struct {
	ID           int       `json:"id"`
	AdminRef     string    `json:"admin_ref"`
	Action       string    `json:"action"`
	TargetType   string    `json:"target_type"`
	TargetID     *int      `json:"target_id,omitempty"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
