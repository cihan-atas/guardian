import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';

import { AuthService, SessionInfo } from './auth.service';

describe('AuthService', () => {
  let service: AuthService;
  let routerSpy: jasmine.SpyObj<Router>;

  const session: SessionInfo = {
    token: 'tok-1',
    username: 'admin',
    role: 'operator',
    display_name: 'Yönetici',
  };

  beforeEach(() => {
    sessionStorage.clear();
    routerSpy = jasmine.createSpyObj<Router>('Router', ['navigate']);
    TestBed.configureTestingModule({
      providers: [{ provide: Router, useValue: routerSpy }],
    });
    service = TestBed.inject(AuthService);
  });

  it('başlangıçta oturum yokken kimliksiz olmalı', () => {
    expect(service.isAuthenticated()).toBeFalse();
    expect(service.role).toBeNull();
    expect(service.getToken()).toBeNull();
  });

  it('login oturumu saklamalı ve dashboard\'a yönlendirmeli', () => {
    service.login(session);
    expect(service.isAuthenticated()).toBeTrue();
    expect(service.getToken()).toBe('tok-1');
    expect(service.session?.username).toBe('admin');
    expect(routerSpy.navigate).toHaveBeenCalledWith(['/dashboard']);
  });

  it('logout oturumu temizlemeli ve login\'e yönlendirmeli', () => {
    service.login(session);
    service.logout();
    expect(service.isAuthenticated()).toBeFalse();
    expect(service.getToken()).toBeNull();
    expect(service.session).toBeNull();
    expect(routerSpy.navigate).toHaveBeenCalledWith(['/login']);
  });

  describe('hasRole (rütbe karşılaştırması)', () => {
    it('operator, viewer ve operator gereksinimini karşılar ama admin\'i karşılamaz', () => {
      service.login(session); // operator
      expect(service.hasRole('viewer')).toBeTrue();
      expect(service.hasRole('operator')).toBeTrue();
      expect(service.hasRole('admin')).toBeFalse();
    });

    it('admin tüm rolleri karşılar', () => {
      service.login({ ...session, role: 'admin' });
      expect(service.hasRole('viewer')).toBeTrue();
      expect(service.hasRole('operator')).toBeTrue();
      expect(service.hasRole('admin')).toBeTrue();
    });

    it('oturum yokken hiçbir rolü karşılamaz', () => {
      expect(service.hasRole('viewer')).toBeFalse();
    });
  });

  it('sessionStorage\'daki mevcut oturumu okur', () => {
    // AuthService singleton; beforeEach onu boş storage ile oluşturdu. Yeni bir
    // örneğin constructor'da mevcut oturumu okuduğunu görmek için modülü
    // sıfırlayıp storage'ı doldurduktan sonra yeniden inject ediyoruz.
    TestBed.resetTestingModule();
    sessionStorage.setItem('guardian_admin_token', 'tok-2');
    sessionStorage.setItem('guardian_admin_user', JSON.stringify({ ...session, role: 'admin' }));
    TestBed.configureTestingModule({
      providers: [{ provide: Router, useValue: routerSpy }],
    });
    const fresh = TestBed.inject(AuthService);
    expect(fresh.isAuthenticated()).toBeTrue();
    expect(fresh.role).toBe('admin');
  });
});
