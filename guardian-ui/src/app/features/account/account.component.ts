import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ToastrService } from 'ngx-toastr';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import {
  faUserShield, faKey, faLock, faShieldHalved, faSpinner, faCheck, faQrcode
} from '@fortawesome/free-solid-svg-icons';
import { ApiClientService } from '../../core/services/api-client.service';
import { AuthService } from '../../core/services/auth.service';

@Component({
  selector: 'app-account',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent],
  templateUrl: './account.component.html',
})
export class AccountComponent implements OnInit {
  faUserShield = faUserShield;
  faKey = faKey;
  faLock = faLock;
  faShield = faShieldHalved;
  faSpinner = faSpinner;
  faCheck = faCheck;
  faQrcode = faQrcode;

  // Parola değiştirme
  currentPassword = '';
  newPassword = '';
  newPassword2 = '';
  isChangingPassword = false;

  // 2FA durumu
  totpEnabled = false;
  isLoading = true;

  // 2FA kurulum akışı
  setupActive = false;
  setupSecret = '';
  setupUri = '';
  setupQrDataUrl = '';
  enableCode = '';
  isSettingUp = false;
  isEnabling = false;

  // 2FA kapatma
  disablePassword = '';
  isDisabling = false;

  constructor(
    private api: ApiClientService,
    private auth: AuthService,
    private toastr: ToastrService,
  ) {}

  ngOnInit(): void {
    this.api.getMe().subscribe({
      next: (me) => {
        this.totpEnabled = !!me.totp_enabled;
        this.isLoading = false;
      },
      error: () => (this.isLoading = false),
    });
  }

  get username(): string {
    return this.auth.session?.display_name || this.auth.session?.username || '';
  }

  // --- Parola değiştirme ---
  changePassword(): void {
    if (this.newPassword.length < 6) {
      this.toastr.warning('Yeni parola en az 6 karakter olmalı.', 'Uyarı');
      return;
    }
    if (this.newPassword !== this.newPassword2) {
      this.toastr.warning('Yeni parolalar eşleşmiyor.', 'Uyarı');
      return;
    }
    this.isChangingPassword = true;
    this.api.changePassword(this.currentPassword, this.newPassword).subscribe({
      next: () => {
        this.toastr.success('Parola değiştirildi. Yeniden giriş yapmanız gerekebilir.', 'Başarılı');
        this.currentPassword = this.newPassword = this.newPassword2 = '';
        this.isChangingPassword = false;
        // Diğer oturumlar düşürüldüğü için mevcut token da geçersiz olabilir.
        setTimeout(() => this.auth.logout(), 1500);
      },
      error: (err) => {
        this.toastr.error(err.error || 'Parola değiştirilemedi.', 'Hata');
        this.isChangingPassword = false;
      },
    });
  }

  // --- 2FA kurulum ---
  startSetup(): void {
    this.isSettingUp = true;
    this.api.setup2fa().subscribe({
      next: async (res) => {
        this.setupSecret = res.secret;
        this.setupUri = res.otpauth_uri;
        this.setupActive = true;
        this.isSettingUp = false;
        // QR'ı istemcide üret (harici servis yok).
        try {
          const QRCode = await import('qrcode');
          this.setupQrDataUrl = await QRCode.toDataURL(res.otpauth_uri, { margin: 1, width: 220 });
        } catch {
          this.setupQrDataUrl = ''; // QR üretilemezse manuel anahtar yeterli.
        }
      },
      error: () => {
        this.toastr.error('2FA kurulumu başlatılamadı.', 'Hata');
        this.isSettingUp = false;
      },
    });
  }

  confirmEnable(): void {
    if (this.enableCode.trim().length < 6) return;
    this.isEnabling = true;
    this.api.enable2fa(this.enableCode.trim()).subscribe({
      next: () => {
        this.toastr.success('İki adımlı doğrulama etkinleştirildi.', 'Başarılı');
        this.totpEnabled = true;
        this.resetSetup();
        this.isEnabling = false;
      },
      error: (err) => {
        this.toastr.error(err.error || 'Kod doğrulanamadı.', 'Hata');
        this.isEnabling = false;
      },
    });
  }

  cancelSetup(): void {
    this.resetSetup();
  }

  private resetSetup(): void {
    this.setupActive = false;
    this.setupSecret = '';
    this.setupUri = '';
    this.setupQrDataUrl = '';
    this.enableCode = '';
  }

  // --- 2FA kapatma ---
  disable2fa(): void {
    if (!this.disablePassword) {
      this.toastr.warning('Parolanızı girin.', 'Uyarı');
      return;
    }
    this.isDisabling = true;
    this.api.disable2fa(this.disablePassword).subscribe({
      next: () => {
        this.toastr.success('İki adımlı doğrulama kapatıldı.', 'Başarılı');
        this.totpEnabled = false;
        this.disablePassword = '';
        this.isDisabling = false;
      },
      error: (err) => {
        this.toastr.error(err.error || '2FA kapatılamadı.', 'Hata');
        this.isDisabling = false;
      },
    });
  }

  /** Gizli anahtarı 4'lü gruplar halinde okunur biçimde döner. */
  get formattedSecret(): string {
    return this.setupSecret.replace(/(.{4})/g, '$1 ').trim();
  }

  copySecret(): void {
    navigator.clipboard?.writeText(this.setupSecret).then(
      () => this.toastr.info('Anahtar kopyalandı.', ''),
      () => {},
    );
  }
}
