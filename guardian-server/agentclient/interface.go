// guardian/guardian-server/agentclient/interface.go

package agentclient

import "guardian.com/server/models"

// AgentCommunicator, scheduler ve handler gibi diğer paketlerin agent client ile
// nasıl iletişim kuracağını tanımlayan bir sözleşmedir (interface).
type AgentCommunicator interface {
	SendKeyCommand(ip, action string, payload models.KeyPayload) error
	TerminateSession(ip string, sessionID int) error
	ValidateUser(ip, username string) error // <-- EKLENEN SATIR
}

// Bu kontrol satırı sayesinde, eğer Client struct'ımız ValidateUser metodunu
// sağlamıyorsa, şimdi derleme hatası verecektir. (Ama zaten sağlıyor, o yüzden sorun yok).
var _ AgentCommunicator = (*Client)(nil)
