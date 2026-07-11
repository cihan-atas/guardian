import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
import { BehaviorSubject } from 'rxjs';

export type Role = 'viewer' | 'operator' | 'admin';

export interface SessionInfo {
  token: string;
  username: string;
  role: Role;
  display_name: string;
}

const ROLE_RANK: Record<string, number> = { viewer: 1, operator: 2, admin: 3 };

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly TOKEN_KEY = 'guardian_admin_token';
  private readonly USER_KEY = 'guardian_admin_user';

  private _isAuthenticated = new BehaviorSubject<boolean>(this.getToken() !== null);
  public isAuthenticated$ = this._isAuthenticated.asObservable();

  private _session = new BehaviorSubject<SessionInfo | null>(this.readSession());
  public session$ = this._session.asObservable();

  constructor(private router: Router) { }

  login(session: SessionInfo): void {
    sessionStorage.setItem(this.TOKEN_KEY, session.token);
    sessionStorage.setItem(this.USER_KEY, JSON.stringify(session));
    this._session.next(session);
    this._isAuthenticated.next(true);
    this.router.navigate(['/dashboard']);
  }

  logout(): void {
    sessionStorage.removeItem(this.TOKEN_KEY);
    sessionStorage.removeItem(this.USER_KEY);
    this._session.next(null);
    this._isAuthenticated.next(false);
    this.router.navigate(['/login']);
  }

  getToken(): string | null {
    return sessionStorage.getItem(this.TOKEN_KEY);
  }

  isAuthenticated(): boolean {
    return this._isAuthenticated.value;
  }

  get session(): SessionInfo | null {
    return this._session.value;
  }

  get role(): Role | null {
    return this._session.value?.role ?? null;
  }

  /** Kullanıcının en az verilen role sahip olup olmadığını döndürür. */
  hasRole(min: Role): boolean {
    const r = this.role;
    if (!r) return false;
    return (ROLE_RANK[r] ?? 0) >= (ROLE_RANK[min] ?? 99);
  }

  private readSession(): SessionInfo | null {
    const raw = sessionStorage.getItem(this.USER_KEY);
    if (!raw) return null;
    try {
      return JSON.parse(raw) as SessionInfo;
    } catch {
      return null;
    }
  }
}
