package models

import "time"

type Session struct {
	ID        int        `json:"id"`
	RuleID    *int       `json:"rule_id,omitempty"`
	ServerID  int        `json:"server_id"`
	Username  string     `json:"username"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Status    string     `json:"status"`
}

type SessionDetailsAPI struct {
	ID             int        `json:"id"`
	RuleID         *int       `json:"rule_id,omitempty"`
	ServerID       int        `json:"server_id"`
	Username       string     `json:"username"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	Status         string     `json:"status"`
	ServerHostname string     `json:"server_hostname"`
	ServerIP       string     `json:"server_ip"`
}

type SessionEvent struct {
	ID        int64     `json:"id"`
	SessionID int       `json:"session_id"`
	EventType string    `json:"event_type"`
	Data      []byte    `json:"data"`
	EventTime time.Time `json:"event_time"`
}
