import { Component, Input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faCopy, faCheck } from '@fortawesome/free-solid-svg-icons';

/** Panoya kopyalama butonu — başarıda 1.5 sn ✓ gösterir. */
@Component({
  selector: 'app-copy-button',
  standalone: true,
  imports: [CommonModule, FaIconComponent],
  template: `
    <button (click)="copy($event)" [title]="copied ? 'Kopyalandı' : title"
            class="inline-flex items-center justify-center w-6 h-6 rounded transition-colors align-middle"
            [ngClass]="copied ? 'text-success-400' : 'text-text-secondary/60 hover:text-accent hover:bg-dark-interactive'">
      <fa-icon [icon]="copied ? faCheck : faCopy" class="text-xs"></fa-icon>
    </button>
  `,
})
export class CopyButtonComponent {
  @Input({ required: true }) value = '';
  @Input() title = 'Kopyala';

  copied = false;
  faCopy = faCopy;
  faCheck = faCheck;

  copy(event: Event): void {
    event.stopPropagation();
    navigator.clipboard.writeText(this.value).then(() => {
      this.copied = true;
      setTimeout(() => (this.copied = false), 1500);
    });
  }
}
