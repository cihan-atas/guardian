import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, NotificationSettings } from '../../core/services/api-client.service';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { Subject } from 'rxjs';
import { debounceTime, switchMap } from 'rxjs/operators';
import {
  faGear, faBell, faEnvelope, faTriangleExclamation, faPaperPlane, faSpinner, faSave, faBroom
} from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent],
  templateUrl: './settings.component.html',
})
export class SettingsComponent implements OnInit {
  faGear = faGear;
  faBell = faBell;
  faEnvelope = faEnvelope;
  faWarn = faTriangleExclamation;
  faTest = faPaperPlane;
  faSpinner = faSpinner;
  faSave = faSave;
  faBroom = faBroom;

  isLoading = true;
  isSaving = false;
  isTesting = false;
  smtpPassSet = false;

  // Kayıt saklama önizlemesi (kaç olay kaydı etkilenecek).
  retentionPreviewCount: number | null = null;
  retentionLastRun = '';
  retentionLastDeleted = 0;
  private retentionPreview$ = new Subject<number>();

  readonly retentionPresets = [
    { value: 0, label: 'Sınırsız' },
    { value: 30, label: '30 gün' },
    { value: 90, label: '90 gün' },
    { value: 180, label: '180 gün' },
    { value: 365, label: '1 yıl' },
  ];

  model: NotificationSettings = {
    webhook_url: '',
    smtp_host: '',
    smtp_port: '587',
    smtp_user: '',
    smtp_pass: '',
    smtp_from: '',
    alert_email_to: '',
    risky_autoaction: 'none',
    retention_days: 0,
  };

  readonly autoActions = [
    { value: 'none', label: 'Sadece uyar', hint: 'Alarm + bildirim düşer; müdahale admin\'de kalır.' },
    { value: 'terminate', label: 'Oturumu kes', hint: 'Kritik komutta oturum anında sonlandırılır.' },
    { value: 'ban', label: 'Anahtarı yasakla', hint: 'Kritik komutta oturum kesilir + anahtar 60 dk yasaklanır.' },
  ];

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService
  ) {}

  ngOnInit(): void {
    this.load();
    // Süre değiştikçe (debounce'lu) önizlemeyi güncelle.
    this.retentionPreview$
      .pipe(
        debounceTime(400),
        switchMap((days) => this.apiClient.getRetentionPreview(days))
      )
      .subscribe({
        next: (r) => (this.retentionPreviewCount = r.count),
        error: () => (this.retentionPreviewCount = null),
      });
  }

  load(): void {
    this.isLoading = true;
    this.apiClient.getSettings().subscribe({
      next: (s) => {
        this.smtpPassSet = !!s.smtp_pass_set;
        this.model = {
          webhook_url: s.webhook_url || '',
          smtp_host: s.smtp_host || '',
          smtp_port: s.smtp_port || '587',
          smtp_user: s.smtp_user || '',
          smtp_pass: '', // write-only: her zaman boş gelir
          smtp_from: s.smtp_from || '',
          alert_email_to: s.alert_email_to || '',
          risky_autoaction: s.risky_autoaction || 'none',
          retention_days: s.retention_days ?? 0,
        };
        this.retentionLastRun = s.retention_last_run || '';
        this.retentionLastDeleted = s.retention_last_deleted ?? 0;
        this.isLoading = false;
        this.refreshRetentionPreview();
      },
      error: () => {
        this.toastr.error('Ayarlar yüklenemedi.', 'Hata');
        this.isLoading = false;
      }
    });
  }

  save(): void {
    this.isSaving = true;
    // Parola boşsa göndermeyiz (mevcut değer korunur).
    const payload: NotificationSettings = { ...this.model };
    if (!payload.smtp_pass) delete payload.smtp_pass;

    this.apiClient.updateSettings(payload).subscribe({
      next: () => {
        this.toastr.success('Ayarlar kaydedildi ve anında uygulandı.', 'Kaydedildi');
        this.isSaving = false;
        if (this.model.smtp_pass) this.smtpPassSet = true;
        this.model.smtp_pass = '';
        // Saklama temizliği kayıtta tetiklenir; son-temizlik bilgisini tazelemek
        // için ayarları yeniden yükle (kısa gecikme purge'ün bitmesini bekler).
        if (this.retentionDays > 0) setTimeout(() => this.load(), 800);
      },
      error: (err) => {
        this.toastr.error(err.error || 'Ayarlar kaydedilemedi.', 'Hata');
        this.isSaving = false;
      }
    });
  }

  sendTest(): void {
    this.isTesting = true;
    this.apiClient.testNotification().subscribe({
      next: () => {
        this.toastr.info('Test bildirimi gönderildi. Kanallarınızı kontrol edin.', 'Gönderildi');
        this.isTesting = false;
      },
      error: () => {
        this.toastr.error('Test bildirimi gönderilemedi.', 'Hata');
        this.isTesting = false;
      }
    });
  }

  get webhookEnabled(): boolean {
    return !!this.model.webhook_url.trim();
  }

  get emailEnabled(): boolean {
    return !!this.model.smtp_host.trim() && !!this.model.alert_email_to.trim();
  }

  get retentionDays(): number {
    return this.model.retention_days ?? 0;
  }

  setRetention(days: number): void {
    this.model.retention_days = days;
    this.refreshRetentionPreview();
  }

  onRetentionInput(): void {
    let d = Number(this.model.retention_days);
    if (!Number.isFinite(d) || d < 0) d = 0;
    this.model.retention_days = Math.floor(d);
    this.refreshRetentionPreview();
  }

  refreshRetentionPreview(): void {
    const d = this.retentionDays;
    if (d <= 0) {
      this.retentionPreviewCount = 0;
      return;
    }
    this.retentionPreview$.next(d);
  }
}
