import { Injectable } from '@angular/core';
import { Router } from '@angular/router';
import { BehaviorSubject } from 'rxjs';  

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly TOKEN_KEY = 'guardian_admin_token';

   private _isAuthenticated = new BehaviorSubject<boolean>(this.getToken() !== null);
  
   public isAuthenticated$ = this._isAuthenticated.asObservable();

  constructor(private router: Router) { }

  login(token: string): void {
    sessionStorage.setItem(this.TOKEN_KEY, token);
    this._isAuthenticated.next(true);  
    this.router.navigate(['/dashboard']);
  }

  logout(): void {
    sessionStorage.removeItem(this.TOKEN_KEY);
    this._isAuthenticated.next(false);  
    this.router.navigate(['/login']);
  }

  getToken(): string | null {
    return sessionStorage.getItem(this.TOKEN_KEY);
  }

   isAuthenticated(): boolean {
    return this._isAuthenticated.value;
  }
}