import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService, Role } from '../services/auth.service';

/**
 * Rota `data.role` alanında belirtilen minimum role sahip olmayan kullanıcıyı
 * gösterge paneline yönlendirir. authGuard ile birlikte kullanılır.
 */
export const roleGuard: CanActivateFn = (route) => {
  const authService = inject(AuthService);
  const router = inject(Router);

  const required = (route.data?.['role'] as Role) ?? 'viewer';
  if (authService.hasRole(required)) {
    return true;
  }
  router.navigate(['/dashboard']);
  return false;
};
