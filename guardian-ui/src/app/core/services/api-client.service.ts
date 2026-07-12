// guardian-ui/src/app/core/services/api-client.service.ts

import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { environment } from '../../../environments/environment';


// --- DÜZELTME BAŞLANGICI ---

// Go backend'indeki sql.NullString tipine karşılık gelen TypeScript interface'i
export interface NullString {
  String: string;
  Valid: boolean;
}

// User, CreateUserPayload ve UpdateUserPayload interface'lerini NullString kullanacak şekilde güncelle
export interface User {
  id: number;
  username: string;
  description: NullString; // Değişiklik
  created_at: string;
}

export interface CreateUserPayload {
  username: string;
  description: NullString; // Değişiklik
}

export interface UpdateUserPayload {
  description: NullString; // Değişiklik
}

// --- DÜZELTME BİTİŞİ ---


export interface Server {
  id: number;
  hostname: string;
  ip_address: string;
  description: string;
  created_at: string;
}

export interface PaginatedResponse<T> {
  total_records: number;
  page: number;
  limit: number;
  data: T[];
}

export interface Key {
  id: number;
  key_name: string;
  ssh_public_key: string;
  fingerprint_sha256: string;
  created_at: string;
  /** Aktif yasak varsa liste yanıtında dolu gelir (LEFT JOIN key_bans). */
  banned_until?: string;
  ban_reason?: string;
}

export interface Rule {
  id: number;
  server_id: number;
  public_key_id: number;
  system_user_id: number;
  status: string;
  valid_until: string;
  valid_from: string;
  server_hostname: string;
  username: string;
  key_name: string;
}

export interface Session {
  id: number;
  rule_id?: number;
  server_id: number;
  username: string;
  start_time: string;
  end_time?: string;
  status: string;
  server_hostname: string;
  server_ip: string;
}

export interface DashboardStats {
  active_sessions: number;
  expired_rules: number;
  total_servers: number;
  total_users: number;
  pending_rules: number;
  total_keys: number;
  today_sessions: number;
  failed_sessions: number;
  banned_keys: number;
}

export interface ParsedCommand {
  timestamp: string;
  command: string;
  output: string;
}

export interface SessionDetails {
  session_info: {
    id: number;
    username: string;
    server_id: number;
    server_hostname: string;
    server_ip: string;
    start_time: string;
    end_time?: string;
    status: string;
    rule_id?: number;
    system_user_id?: number;
    public_key_id?: number;
    public_key_name?: string;
  };
  commands: ParsedCommand[];
}

export interface KeyBan {
  id: number;
  public_key_id: number;
  reason?: string;
  banned_at: string;
  banned_until: string;
}

export interface KeyBanStatus {
  banned: boolean;
  ban?: KeyBan;
}

export interface CreateServerPayload {
  hostname: string;
  ip_address: string;
  description: string;
}

export interface CreateKeyPayload {
  key_name: string;
  ssh_public_key: string;
}

export interface UpdateKeyPayload {
  key_name: string;
}

export interface CreateRulePayload {
  server_id: number;
  public_key_id: number;
  system_user_id: number;
  valid_from: string;
  valid_until: string;
}

export interface UpdateRulePayload {
  valid_from?: string;
  valid_until?: string;
  status?: string;
}

export type UpdateServerPayload = Partial<{
  hostname: string;
  ip_address: string;
  description: string;
}>;

export interface SessionEvent {
  id: number;
  session_id: number;
  event_type: string;
  data: string;
  event_time: string;
}

export interface SessionReplay {
  cols: number;
  rows: number;
  events: SessionEvent[];
}

export interface SessionTerminalSize {
  cols: number;
  rows: number;
}

export interface ChartData {
  name: string;
  value: number;
}
export interface SeriesChartData {
  name: string;
  series: ChartData[];
}

/** Tam komut bazında kullanım istatistiği (sunucu kırılımıyla). */
export interface CommandStat {
  command: string;
  base: string;
  count: number;
  servers: Record<string, number>;
}

export interface ActiveSessionInfo {
  id: number;
  username: string;
  start_time: string;
  server_hostname: string;
}

/** Bildirim/alarm ayarları (UI'dan yönetilir; smtp_pass write-only). */
export interface NotificationSettings {
  webhook_url: string;
  smtp_host: string;
  smtp_port: string;
  smtp_user: string;
  smtp_pass?: string;
  smtp_pass_set?: boolean;
  smtp_from: string;
  alert_email_to: string;
  risky_autoaction: 'none' | 'terminate' | 'ban' | string;
}

/** Güvenlik uyarısı (riskli komut tespiti). */
export interface Alert {
  id: number;
  session_id: number;
  severity: 'high' | 'critical' | string;
  rule_name: string;
  command: string;
  action_taken: string;
  created_at: string;
  username?: string;
  server_hostname?: string;
}

/** Global komut arama eşleşmesi. */
export interface CommandMatch {
  session_id: number;
  command_index: number;
  command: string;
  username: string;
  server_hostname: string;
  start_time: string;
  status: string;
}

/** Bir sertifikanın süre-sonu özeti. */
export interface CertInfo {
  subject: string;
  issuer: string;
  not_before: string;
  not_after: string;
  days_left: number;
}

/** Bir sunucunun agent sertifikası durumu. */
export interface AgentCertInfo {
  server_id: number;
  hostname: string;
  ip_address: string;
  online: boolean;
  cert?: CertInfo;
}

/** GET /api/certificates yanıtı. */
export interface CertificatesResponse {
  ca?: CertInfo;
  ca_error?: string;
  server?: CertInfo;
  server_error?: string;
  agents?: AgentCertInfo[];
}

/** Agent kayıt token'ı + kurulum komutu yanıtı. */
export interface EnrollTokenResponse {
  token: string;
  expires_at: string;
  server_id: number;
  server_hostname: string;
  server_ip: string;
  base_url: string;
  os?: string;
  install_command: string;
  binary_available: boolean;
}

/** SSH ile uzaktan kurulum sonucu. */
export interface SSHInstallResult {
  success: boolean;
  output: string;
  error?: string;
}

export interface SSHInstallPayload {
  ssh_host?: string;
  ssh_port?: string;
  ssh_user: string;
  ssh_password?: string;
  ssh_private_key?: string;
  validity_days?: number;
}

/** Sunucu agent sağlık durumu. */
export interface ServerHealth {
  server_id: number;
  hostname: string;
  ip_address: string;
  online: boolean;
  latency_ms: number;
}

export interface AuditLog {
  id: number;
  admin_ref: string;
  action: string;
  target_type: string;
  target_id?: number;
  status: string;
  error_message?: string;
  created_at: string;
}

export type Role = 'viewer' | 'operator' | 'admin';

/** Giriş yanıtı / oturum kimliği. */
export interface LoginResponse {
  token: string;
  username: string;
  role: Role;
  display_name: string;
}

export interface AuthIdentity {
  id?: number;
  username: string;
  role: Role;
  display_name: string;
}

/** Yönetici hesabı (RBAC). */
export interface AdminUser {
  id: number;
  username: string;
  role: Role;
  display_name: string;
  disabled: boolean;
  created_at: string;
  last_login?: string;
}

export interface CreateAdminPayload {
  username: string;
  password: string;
  role: Role;
  display_name: string;
}

export interface UpdateAdminPayload {
  role?: Role;
  display_name?: string;
  disabled?: boolean;
  password?: string;
}

/** Erişim talebi (onay akışı). */
export interface AccessRequest {
  id: number;
  server_id: number;
  public_key_id: number;
  system_user_id: number;
  valid_from: string;
  valid_until: string;
  status: string;
  created_at: string;
  server_hostname: string;
  username: string;
  key_name: string;
  request_reason: string;
  reject_reason?: string;
  requested_by?: string;
  approved_by?: string;
  decided_at?: string;
}

export interface CreateAccessRequestPayload {
  server_id: number;
  public_key_id: number;
  system_user_id: number;
  valid_from: string;
  valid_until: string;
  reason: string;
}

@Injectable({
  providedIn: 'root'
})
export class ApiClientService {
   private apiUrl = environment.apiUrl;

   constructor(private http: HttpClient) { }

  /** Liste sorgu parametrelerini üretir; boş search/status eklenmez. */
  private listParams(page: number, limit: number, search?: string, status?: string): string {
    let params = `page=${page}&limit=${limit}`;
    if (search?.trim()) params += `&search=${encodeURIComponent(search.trim())}`;
    if (status?.trim()) params += `&status=${encodeURIComponent(status.trim())}`;
    return params;
  }

  login(username: string, password: string): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${this.apiUrl}/auth/login`, { username, password });
  }

  logout(): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/auth/logout`, {});
  }

  getMe(): Observable<AuthIdentity> {
    return this.http.get<AuthIdentity>(`${this.apiUrl}/auth/me`);
  }

  changePassword(currentPassword: string, newPassword: string): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/auth/change-password`, {
      current_password: currentPassword, new_password: newPassword,
    });
  }

  // --- Yönetici hesapları (RBAC) ---
  getAdminUsers(): Observable<AdminUser[]> {
    return this.http.get<AdminUser[]>(`${this.apiUrl}/admin-users`);
  }
  createAdminUser(payload: CreateAdminPayload): Observable<AdminUser> {
    return this.http.post<AdminUser>(`${this.apiUrl}/admin-users`, payload);
  }
  updateAdminUser(id: number, payload: UpdateAdminPayload): Observable<void> {
    return this.http.patch<void>(`${this.apiUrl}/admin-users/${id}`, payload);
  }
  deleteAdminUser(id: number): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/admin-users/${id}`);
  }

  // --- Erişim talepleri (onay akışı) ---
  getAccessRequests(status?: string): Observable<AccessRequest[]> {
    const q = status?.trim() ? `?status=${encodeURIComponent(status.trim())}` : '';
    return this.http.get<AccessRequest[]>(`${this.apiUrl}/access-requests${q}`);
  }
  createAccessRequest(payload: CreateAccessRequestPayload): Observable<{ id: number }> {
    return this.http.post<{ id: number }>(`${this.apiUrl}/access-requests`, payload);
  }
  approveAccessRequest(id: number): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/access-requests/${id}/approve`, {});
  }
  rejectAccessRequest(id: number, reason?: string): Observable<void> {
    return this.http.post<void>(`${this.apiUrl}/access-requests/${id}/reject`, { reason });
  }

      getServers(page: number = 1, limit: number = 20, search?: string): Observable<PaginatedResponse<Server>> {
    return this.http.get<PaginatedResponse<Server>>(`${this.apiUrl}/servers?${this.listParams(page, limit, search)}`);
  }
   createServer(payload: CreateServerPayload): Observable<Server> {
     return this.http.post<Server>(`${this.apiUrl}/servers`, payload);
   }
     updateServer(serverId: number, payload: UpdateServerPayload): Observable<Server> {
    return this.http.patch<Server>(`${this.apiUrl}/servers/${serverId}`, payload);
  }
    updateUser(userId: number, payload: UpdateUserPayload): Observable<User> {
    return this.http.patch<User>(`${this.apiUrl}/users/${userId}`, payload);
  }

   updateKey(keyId: number, payload: UpdateKeyPayload): Observable<Key> {
    return this.http.patch<Key>(`${this.apiUrl}/keys/${keyId}`, payload);
  }

   deleteServer(serverId: number): Observable<void> {
     return this.http.delete<void>(`${this.apiUrl}/servers/${serverId}`);
   }

        getUsers(page: number = 1, limit: number = 20, search?: string): Observable<PaginatedResponse<User>> {
    return this.http.get<PaginatedResponse<User>>(`${this.apiUrl}/users?${this.listParams(page, limit, search)}`);
  }
   createUser(payload: CreateUserPayload): Observable<User> {
     return this.http.post<User>(`${this.apiUrl}/users`, payload);
   }
   deleteUser(userId: number): Observable<void> {
     return this.http.delete<void>(`${this.apiUrl}/users/${userId}`);
   }

       getKeys(page: number = 1, limit: number = 20, search?: string): Observable<PaginatedResponse<Key>> {
    return this.http.get<PaginatedResponse<Key>>(`${this.apiUrl}/keys?${this.listParams(page, limit, search)}`);
  }
   createKey(payload: CreateKeyPayload): Observable<Key> {
     return this.http.post<Key>(`${this.apiUrl}/keys`, payload);
   }
   deleteKey(keyId: number): Observable<void> {
     return this.http.delete<void>(`${this.apiUrl}/keys/${keyId}`);
   }

      getRules(page: number = 1, limit: number = 20, search?: string): Observable<PaginatedResponse<Rule>> {
    return this.http.get<PaginatedResponse<Rule>>(`${this.apiUrl}/rules?${this.listParams(page, limit, search)}`);
  }
   createRule(payload: CreateRulePayload): Observable<Rule> {
     return this.http.post<Rule>(`${this.apiUrl}/rules`, payload);
   }
   deleteRule(ruleId: number): Observable<void> {
     return this.http.delete<void>(`${this.apiUrl}/rules/${ruleId}`);
   }

  updateRule(ruleId: number, payload: UpdateRulePayload): Observable<Rule> {
    return this.http.patch<Rule>(`${this.apiUrl}/rules/${ruleId}`, payload);
  }

  getSessions(page: number = 1, limit: number = 20, search?: string, status?: string): Observable<PaginatedResponse<Session>> {
    return this.http.get<PaginatedResponse<Session>>(`${this.apiUrl}/sessions?${this.listParams(page, limit, search, status)}`);
  }

      getSessionReplay(sessionId: number): Observable<SessionReplay> {
    return this.http.get<SessionReplay>(`${this.apiUrl}/sessions/${sessionId}/replay`);
  }

  getSessionTerminalSize(sessionId: number): Observable<SessionTerminalSize> {
    return this.http.get<SessionTerminalSize>(`${this.apiUrl}/sessions/${sessionId}/meta`);
  }

  getSessionDetails(sessionId: number): Observable<SessionDetails> {
    return this.http.get<SessionDetails>(`${this.apiUrl}/sessions/${sessionId}/commands`);
  }

  terminateSession(sessionId: number): Observable<string> {
    return this.http.delete(`${this.apiUrl}/sessions/${sessionId}`, { responseType: 'text' });
  }

  getKeyBanStatus(keyId: number): Observable<KeyBanStatus> {
    return this.http.get<KeyBanStatus>(`${this.apiUrl}/keys/${keyId}/ban`);
  }

  banKey(keyId: number, durationMinutes: number, reason?: string): Observable<KeyBan> {
    return this.http.post<KeyBan>(`${this.apiUrl}/keys/${keyId}/ban`, { duration_minutes: durationMinutes, reason });
  }

  unbanKey(keyId: number): Observable<void> {
    return this.http.delete<void>(`${this.apiUrl}/keys/${keyId}/ban`);
  }

    getDashboardStats(): Observable<DashboardStats> {
    return this.http.get<DashboardStats>(`${this.apiUrl}/dashboard/stats`);
  }

  getSessionActivity(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/session-activity`);
  }

  getTopServers(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/top-servers`);
  }

  getSessionStatusBreakdown(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/session-status`);
  }

  getRuleStatusBreakdown(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/rule-status`);
  }

  getTopCommands(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/top-commands`);
  }

  getCommandStats(): Observable<CommandStat[]> {
    return this.http.get<CommandStat[]>(`${this.apiUrl}/dashboard/command-stats`);
  }

  getUserActivity(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/user-activity`);
  }

  getHourlyActivity(): Observable<ChartData[]> {
    return this.http.get<ChartData[]>(`${this.apiUrl}/dashboard/hourly-activity`);
  }

  getActiveSessionsList(): Observable<ActiveSessionInfo[]> {
    return this.http.get<ActiveSessionInfo[]>(`${this.apiUrl}/dashboard/active-sessions`);
  }

  getAlerts(limit: number = 20): Observable<Alert[]> {
    return this.http.get<Alert[]>(`${this.apiUrl}/dashboard/alerts?limit=${limit}`);
  }

  getSettings(): Observable<NotificationSettings> {
    return this.http.get<NotificationSettings>(`${this.apiUrl}/settings`);
  }

  updateSettings(payload: NotificationSettings): Observable<any> {
    return this.http.put(`${this.apiUrl}/settings`, payload);
  }

  testNotification(): Observable<any> {
    return this.http.post(`${this.apiUrl}/settings/test`, {});
  }

  getAuditLogStream(): Observable<AuditLog[]> {
    return this.http.get<AuditLog[]>(`${this.apiUrl}/dashboard/audit-stream`);
  }

  // Denetim kaydı ekranı (filtre + sayfalama).
  getAuditLogs(page = 1, limit = 20, search?: string, action?: string, status?: string): Observable<PaginatedResponse<AuditLog>> {
    let params = `page=${page}&limit=${limit}`;
    if (search?.trim()) params += `&search=${encodeURIComponent(search.trim())}`;
    if (action?.trim()) params += `&action=${encodeURIComponent(action.trim())}`;
    if (status?.trim()) params += `&status=${encodeURIComponent(status.trim())}`;
    return this.http.get<PaginatedResponse<AuditLog>>(`${this.apiUrl}/audit-logs?${params}`);
  }

  // Sunucu agent sağlık durumu (paralel ping).
  getServersHealth(): Observable<ServerHealth[]> {
    return this.http.get<ServerHealth[]>(`${this.apiUrl}/servers/health`);
  }

  // Global komut arama (tüm oturumlarda).
  searchCommands(q: string, limit = 100): Observable<CommandMatch[]> {
    return this.http.get<CommandMatch[]>(`${this.apiUrl}/commands/search?q=${encodeURIComponent(q)}&limit=${limit}`);
  }

  // Sertifika süre-sonu göstergesi (CA + server + agent cert'leri).
  getCertificates(): Observable<CertificatesResponse> {
    return this.http.get<CertificatesResponse>(`${this.apiUrl}/certificates`);
  }

  // Guardian sunucu sertifikasını seçilen süreyle yeniden imzala.
  renewServerCert(validityDays: number): Observable<{ cert: CertInfo; restart_required: boolean }> {
    return this.http.post<{ cert: CertInfo; restart_required: boolean }>(`${this.apiUrl}/certificates/server/renew`, { validity_days: validityDays });
  }

  // Agent kurulumu: bir sunucu için kayıt token'ı + kurulum komutu üret.
  // validityDays: sertifika geçerlilik süresi (gün); os: linux|windows.
  generateEnrollToken(serverId: number, validityDays?: number, os: 'linux' | 'windows' = 'linux'): Observable<EnrollTokenResponse> {
    return this.http.post<EnrollTokenResponse>(`${this.apiUrl}/servers/${serverId}/enroll-token`, { validity_days: validityDays ?? 0, os });
  }

  // Agent kurulumu: SSH ile uzaktan kur.
  sshInstallAgent(serverId: number, payload: SSHInstallPayload): Observable<SSHInstallResult> {
    return this.http.post<SSHInstallResult>(`${this.apiUrl}/servers/${serverId}/ssh-install`, payload);
  }

 }