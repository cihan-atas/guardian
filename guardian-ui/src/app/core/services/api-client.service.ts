// guardian-ui/src/app/core/services/api-client.service.ts

import { HttpClient, HttpHeaders } from '@angular/common/http';
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

    checkAuth(token: string): Observable<any> {
     const headers = new HttpHeaders({ 'Authorization': `Bearer ${token}` });
     return this.http.get(`${this.apiUrl}/auth/check`, { headers });
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

  getAuditLogStream(): Observable<AuditLog[]> {
    return this.http.get<AuditLog[]>(`${this.apiUrl}/dashboard/audit-stream`);
  }

 }