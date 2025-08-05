package models

import "time"

type AccessRule struct {
	ID           int       `json:"id"`
	ServerID     int       `json:"server_id"`
	PublicKeyID  int       `json:"public_key_id"`
	SystemUserID int       `json:"system_user_id"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidUntil   time.Time `json:"valid_until"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type AccessRuleDetails struct {
	ID             int       `json:"id"`
	ServerID       int       `json:"server_id"`
	PublicKeyID    int       `json:"public_key_id"`
	SystemUserID   int       `json:"system_user_id"`
	ValidFrom      time.Time `json:"valid_from"`
	ValidUntil     time.Time `json:"valid_until"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	ServerHostname string    `json:"server_hostname"`
	Username       string    `json:"username"`
	KeyName        string    `json:"key_name"`
}

type KeyPayload struct {
	RuleID       int    `json:"rule_id"`
	Username     string `json:"username"`
	SshPublicKey string `json:"ssh_public_key"`
}
