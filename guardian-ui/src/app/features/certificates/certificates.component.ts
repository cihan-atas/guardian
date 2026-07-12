import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faCertificate, faSync, faTriangleExclamation, faShieldHalved, faServer, faRotate } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, CertificatesResponse, CertInfo, EnrollTokenResponse } from '../../core/services/api-client.service';
import { CopyButtonComponent } from '../../shared/ui/copy-button/copy-button.component';

@Component({
  selector: 'app-certificates',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, CopyButtonComponent],
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
  faRenew = faRotate;

  isLoading = true;
  data: CertificatesResponse | null = null;

  // Ortak süre seçici (yenileme + agent enroll).
  readonly validityOptions = [
    { label: '1 yıl', days: 365 },
    { label: '2 yıl', days: 730 },
    { label: '5 yıl', days: 1825 },
    { label: '10 yıl', days: 3650 },
  ];
  selectedValidity = 3650;

  isRenewingServer = false;
  // serverID → o agent için üretilmiş enroll komutu (yenileme).
  agentEnroll: Record<number, EnrollTokenResponse> = {};
  renewingAgentId: number | null = null;

  ngOnInit(): void { this.load(); }

  load(): void {
    this.isLoading = true;
    this.api.getCertificates().subscribe({
      next: (res) => { this.data = res; this.isLoading = false; },
      error: () => { this.isLoading = false; this.toastr.error('Sertifikalar yüklenemedi.'); },
    });
  }

  renewServer(): void {
    this.isRenewingServer = true;
    this.api.renewServerCert(this.selectedValidity).subscribe({
      next: () => {
        this.isRenewingServer = false;
        this.toastr.success('Server sertifikası yenilendi. Etkin olması için guardian-server yeniden başlatılmalı.', 'Yenilendi', { timeOut: 9000 });
        this.load();
      },
      error: (err) => {
        this.isRenewingServer = false;
        this.toastr.error(err?.error || 'Yenileme başarısız.');
      },
    });
  }

  renewAgent(serverId: number): void {
    this.renewingAgentId = serverId;
    this.api.generateEnrollToken(serverId, this.selectedValidity).subscribe({
      next: (res) => {
        this.renewingAgentId = null;
        this.agentEnroll[serverId] = res;
      },
      error: (err) => {
        this.renewingAgentId = null;
        this.toastr.error(err?.error || 'Yenileme komutu üretilemedi.');
      },
    });
  }

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
