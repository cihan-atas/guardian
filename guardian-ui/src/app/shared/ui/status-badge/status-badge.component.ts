import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { statusLabel, statusTone } from '../status-labels';

/** Durum kodunu Türkçe etiket + tutarlı renk tonuyla gösteren rozet. */
@Component({
  selector: 'app-status-badge',
  standalone: true,
  imports: [CommonModule],
  template: `
    <span class="inline-flex items-center gap-1.5 text-[11px] font-semibold px-2.5 py-1 rounded-full whitespace-nowrap"
          [ngClass]="toneClass">
      <span *ngIf="pulse" class="relative flex h-1.5 w-1.5">
        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-live opacity-75"></span>
        <span class="relative inline-flex rounded-full h-1.5 w-1.5 bg-live"></span>
      </span>
      {{ label }}
    </span>
  `,
})
export class StatusBadgeComponent {
  @Input({ required: true }) status = '';

  get label(): string {
    return statusLabel(this.status);
  }

  /** Aktif durumda canlı nabız noktası göster. */
  get pulse(): boolean {
    return this.status === 'active';
  }

  get toneClass(): string {
    switch (statusTone(this.status)) {
      case 'live': return 'bg-live/10 text-live';
      case 'success': return 'bg-success-500/10 text-success-400';
      case 'danger': return 'bg-danger-500/10 text-danger-400';
      case 'warning': return 'bg-warning-500/10 text-warning-400';
      default: return 'bg-dark-interactive text-text-secondary';
    }
  }
}
