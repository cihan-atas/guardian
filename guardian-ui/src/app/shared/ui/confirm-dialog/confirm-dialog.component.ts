import { Component, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faTriangleExclamation, faCircleQuestion } from '@fortawesome/free-solid-svg-icons';
import { ConfirmDialogService } from './confirm-dialog.service';

/** App kökünde bir kez yer alır; ConfirmDialogService state'ini render eder. */
@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [CommonModule, FaIconComponent],
  template: `
    <div *ngIf="svc.active() as req"
         class="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4"
         (click)="svc.close(false)">
      <div class="bg-dark-card border border-dark-border rounded-xl shadow-2xl w-full max-w-md animate-fade-in"
           (click)="$event.stopPropagation()">
        <div class="p-6">
          <div class="flex items-start gap-4">
            <div class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
                 [ngClass]="req.opts.danger ? 'bg-danger-500/10 text-danger-400' : 'bg-accent/10 text-accent'">
              <fa-icon [icon]="req.opts.danger ? faWarn : faQuestion"></fa-icon>
            </div>
            <div class="min-w-0">
              <h3 class="text-base font-semibold text-text-main">{{ req.opts.title }}</h3>
              <p class="text-sm text-text-secondary mt-1.5 whitespace-pre-line">{{ req.opts.message }}</p>
            </div>
          </div>
        </div>
        <div class="px-6 py-4 bg-dark-main/40 rounded-b-xl flex justify-end gap-3">
          <button (click)="svc.close(false)"
                  class="px-4 py-2 rounded-lg text-sm font-semibold bg-dark-interactive text-text-main hover:bg-dark-border transition-colors">
            {{ req.opts.cancelText || 'Vazgeç' }}
          </button>
          <button (click)="svc.close(true)"
                  class="px-4 py-2 rounded-lg text-sm font-bold text-white transition-colors"
                  [ngClass]="req.opts.danger ? 'bg-danger-600 hover:bg-danger-700' : 'bg-accent hover:bg-accent-hover'">
            {{ req.opts.confirmText || 'Onayla' }}
          </button>
        </div>
      </div>
    </div>
  `,
})
export class ConfirmDialogComponent {
  faWarn = faTriangleExclamation;
  faQuestion = faCircleQuestion;

  constructor(public svc: ConfirmDialogService) {}

  @HostListener('window:keydown.escape')
  onEscape(): void {
    if (this.svc.active()) this.svc.close(false);
  }
}
