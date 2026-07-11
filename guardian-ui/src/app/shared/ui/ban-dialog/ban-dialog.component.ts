import { Component, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faBan } from '@fortawesome/free-solid-svg-icons';
import { BanDialogService } from './ban-dialog.service';

interface DurationPreset {
  label: string;
  minutes: number;
}

/** App kökünde bir kez yer alır; BanDialogService state'ini render eder. */
@Component({
  selector: 'app-ban-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent],
  template: `
    <div *ngIf="svc.active() as req"
         class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4"
         (click)="svc.close(null)">
      <div class="bg-dark-card border border-dark-border rounded-xl shadow-2xl w-full max-w-md animate-fade-in"
           (click)="$event.stopPropagation()">
        <div class="p-6">
          <div class="flex items-start gap-4">
            <div class="w-10 h-10 rounded-lg bg-danger-500/10 text-danger-400 flex items-center justify-center shrink-0">
              <fa-icon [icon]="faBan"></fa-icon>
            </div>
            <div class="min-w-0 flex-grow">
              <h3 class="text-base font-semibold text-text-main">Anahtarı Yasakla</h3>
              <p class="text-sm text-text-secondary mt-1">
                <span class="font-mono text-text-main">{{ req.req.keyName }}</span> anahtarının tüm
                aktif erişim kuralları iptal edilir ve süre boyunca yeni kural oluşturulamaz.
              </p>

              <p class="text-xs font-semibold uppercase tracking-wider text-text-secondary mt-5 mb-2">Yasak Süresi</p>
              <div class="flex flex-wrap gap-2">
                <button *ngFor="let p of presets" (click)="selectPreset(p.minutes)"
                        class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
                        [ngClass]="!isCustom && durationMinutes === p.minutes
                          ? 'bg-danger-600 text-white'
                          : 'bg-dark-interactive text-text-secondary hover:text-text-main'">
                  {{ p.label }}
                </button>
                <button (click)="isCustom = true"
                        class="px-3 py-1.5 rounded-lg text-xs font-semibold transition-colors"
                        [ngClass]="isCustom ? 'bg-danger-600 text-white' : 'bg-dark-interactive text-text-secondary hover:text-text-main'">
                  Özel…
                </button>
              </div>
              <div *ngIf="isCustom" class="mt-2 flex items-center gap-2">
                <input type="number" min="1" [(ngModel)]="customMinutes"
                       class="w-28 bg-dark-main border border-dark-border rounded-lg px-3 py-1.5 text-sm text-text-main focus:outline-none focus:border-danger-500">
                <span class="text-xs text-text-secondary">dakika</span>
              </div>

              <p class="text-xs font-semibold uppercase tracking-wider text-text-secondary mt-5 mb-2">Gerekçe <span class="normal-case font-normal">(opsiyonel)</span></p>
              <textarea [(ngModel)]="reason" rows="2" placeholder="örn. şüpheli komut aktivitesi"
                        class="w-full bg-dark-main border border-dark-border rounded-lg px-3 py-2 text-sm text-text-main focus:outline-none focus:border-danger-500 resize-none"></textarea>
            </div>
          </div>
        </div>
        <div class="px-6 py-4 bg-dark-main/40 rounded-b-xl flex justify-end gap-3">
          <button (click)="svc.close(null)"
                  class="px-4 py-2 rounded-lg text-sm font-semibold bg-dark-interactive text-text-main hover:bg-dark-border transition-colors">
            Vazgeç
          </button>
          <button (click)="submit()" [disabled]="effectiveMinutes < 1"
                  class="px-4 py-2 rounded-lg text-sm font-bold text-white bg-danger-600 hover:bg-danger-700 transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-2">
            <fa-icon [icon]="faBan"></fa-icon> Yasakla ({{ durationLabel }})
          </button>
        </div>
      </div>
    </div>
  `,
})
export class BanDialogComponent {
  faBan = faBan;

  readonly presets: DurationPreset[] = [
    { label: '30 dk', minutes: 30 },
    { label: '1 saat', minutes: 60 },
    { label: '24 saat', minutes: 1440 },
    { label: '7 gün', minutes: 10080 },
  ];

  durationMinutes = 60;
  isCustom = false;
  customMinutes = 60;
  reason = '';

  constructor(public svc: BanDialogService) {}

  get effectiveMinutes(): number {
    return this.isCustom ? Math.floor(this.customMinutes) || 0 : this.durationMinutes;
  }

  get durationLabel(): string {
    const m = this.effectiveMinutes;
    if (m < 60) return `${m} dk`;
    if (m < 1440) return `${Math.floor(m / 60)} saat`;
    return `${Math.floor(m / 1440)} gün`;
  }

  selectPreset(minutes: number): void {
    this.isCustom = false;
    this.durationMinutes = minutes;
  }

  submit(): void {
    if (this.effectiveMinutes < 1) return;
    const reason = this.reason.trim();
    this.svc.close({ durationMinutes: this.effectiveMinutes, reason: reason || undefined });
    this.resetForm();
  }

  @HostListener('window:keydown.escape')
  onEscape(): void {
    if (this.svc.active()) {
      this.svc.close(null);
      this.resetForm();
    }
  }

  private resetForm(): void {
    this.durationMinutes = 60;
    this.isCustom = false;
    this.customMinutes = 60;
    this.reason = '';
  }
}
