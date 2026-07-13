import { TestBed } from '@angular/core/testing';
import { HttpRequest, HttpHandlerFn, HttpErrorResponse, HttpEvent } from '@angular/common/http';
import { of, throwError, firstValueFrom } from 'rxjs';

import { authInterceptor } from './auth-interceptor';
import { AuthService } from '../services/auth.service';

describe('authInterceptor', () => {
  let authSpy: jasmine.SpyObj<AuthService>;

  function run(req: HttpRequest<unknown>, next: HttpHandlerFn) {
    return TestBed.runInInjectionContext(() => authInterceptor(req, next));
  }

  beforeEach(() => {
    authSpy = jasmine.createSpyObj<AuthService>('AuthService', ['getToken', 'logout']);
    TestBed.configureTestingModule({
      providers: [{ provide: AuthService, useValue: authSpy }],
    });
  });

  it('token varsa Authorization başlığını ekler', async () => {
    authSpy.getToken.and.returnValue('tok-xyz');
    let seen: HttpRequest<unknown> | null = null;
    const next: HttpHandlerFn = (r) => {
      seen = r;
      return of({} as HttpEvent<unknown>);
    };
    await firstValueFrom(run(new HttpRequest('GET', '/api/x'), next));
    expect(seen!.headers.get('Authorization')).toBe('Bearer tok-xyz');
  });

  it('token yoksa Authorization başlığı eklemez', async () => {
    authSpy.getToken.and.returnValue(null);
    let seen: HttpRequest<unknown> | null = null;
    const next: HttpHandlerFn = (r) => {
      seen = r;
      return of({} as HttpEvent<unknown>);
    };
    await firstValueFrom(run(new HttpRequest('GET', '/api/x'), next));
    expect(seen!.headers.has('Authorization')).toBeFalse();
  });

  it('401 hatasında logout çağırır', async () => {
    authSpy.getToken.and.returnValue('tok');
    const next: HttpHandlerFn = () =>
      throwError(() => new HttpErrorResponse({ status: 401 }));
    await expectAsync(firstValueFrom(run(new HttpRequest('GET', '/api/x'), next))).toBeRejected();
    expect(authSpy.logout).toHaveBeenCalled();
  });

  it('403 hatasında logout ÇAĞIRMAZ (yetki reddi, oturum düşmez)', async () => {
    authSpy.getToken.and.returnValue('tok');
    const next: HttpHandlerFn = () =>
      throwError(() => new HttpErrorResponse({ status: 403 }));
    await expectAsync(firstValueFrom(run(new HttpRequest('GET', '/api/x'), next))).toBeRejected();
    expect(authSpy.logout).not.toHaveBeenCalled();
  });
});
