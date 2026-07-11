import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';

/**
 * Liste ekranlarının ortak sayfalama çubuğu.
 * 5 ekranda kopyalanan "Önceki / Sayfa X / Sonraki" bloğunun tek kaynağı.
 */
@Component({
  selector: 'app-pagination',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div *ngIf="totalPages > 1 || total > 0"
         class="flex items-center justify-between px-6 py-3 border-t border-dark-border text-sm">
      <span class="text-text-secondary font-mono text-xs">
        {{ total }} kayıt<span *ngIf="totalPages > 1"> · sayfa {{ page }}/{{ totalPages }}</span>
      </span>
      <div *ngIf="totalPages > 1" class="flex items-center gap-2">
        <button (click)="pageChange.emit(page - 1)" [disabled]="page <= 1" class="pg-btn">‹ Önceki</button>
        <button (click)="pageChange.emit(page + 1)" [disabled]="page >= totalPages" class="pg-btn">Sonraki ›</button>
      </div>
    </div>
  `,
  styles: [`
    .pg-btn {
      @apply px-3 py-1.5 rounded-lg bg-dark-interactive text-text-main text-xs font-semibold
        hover:bg-dark-border transition-colors disabled:opacity-40 disabled:cursor-not-allowed;
    }
  `]
})
export class PaginationComponent {
  @Input() page = 1;
  @Input() limit = 8;
  @Input() total = 0;
  @Output() readonly pageChange = new EventEmitter<number>();

  get totalPages(): number {
    return Math.max(1, Math.ceil(this.total / this.limit));
  }
}
