import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faCertificate, faSync, faTriangleExclamation, faShieldHalved, faServer } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, CertificatesResponse, CertInfo } from '../../core/services/api-client.service';

@Component({
  selector: 'app-certificates',
  standalone: true,
  imports: [CommonModule, FaIconComponent],
  templateUrl: './certificates.component.html',
})
export class CertificatesComponent implements OnInit {
  private api = inject(ApiClientService);
  private toastr = inject(ToastrService);

  faSpinner = faSpinner;
  faCert = faCertificate;
  faSync = faSync;
  faWarn = faTriangleExclamation;
  faShield = faShieldHalved;
  faServer = faServer;

  isLoading = true;
  data: CertificatesResponse | null = null;

  ngOnInit(): void { this.load(); }

  load(): void {
    this.isLoading = true;
    this.api.getCertificates().subscribe({
      next: (res) => { this.data = res; this.isLoading = false; },
      error: () => { this.isLoading = false; this.toastr.error('Sertifikalar yüklenemedi.'); },
    });
  }

  /** Kalan güne göre renk tonu: >30 yeşil, 8-30 sarı, <=7 veya negatif kırmızı. */
  tone(daysLeft: number | undefined): 'ok' | 'warn' | 'danger' {
    if (daysLeft === undefined) return 'warn';
    if (daysLeft <= 7) return 'danger';
    if (daysLeft <= 30) return 'warn';
    return 'ok';
  }

  toneClass(daysLeft: number | undefined): string {
    switch (this.tone(daysLeft)) {
      case 'ok': return 'text-success-400';
      case 'warn': return 'text-warning-400';
      case 'danger': return 'text-danger-400';
    }
  }

  daysLabel(cert: CertInfo | undefined): string {
    if (!cert) return '—';
    if (cert.days_left < 0) return `${Math.abs(cert.days_left)} gün önce doldu`;
    return `${cert.days_left} gün kaldı`;
  }
}
