import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faDownload, faTerminal, faServer, faTriangleExclamation, faBolt } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, Server, EnrollTokenResponse, SSHInstallResult } from '../../core/services/api-client.service';
import { CopyButtonComponent } from '../../shared/ui/copy-button/copy-button.component';

@Component({
  selector: 'app-agent-install',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, CopyButtonComponent],
  templateUrl: './agent-install.component.html',
})
export class AgentInstallComponent implements OnInit {
  private api = inject(ApiClientService);
  private toastr = inject(ToastrService);

  faSpinner = faSpinner;
  faDownload = faDownload;
  faTerminal = faTerminal;
  faServer = faServer;
  faWarn = faTriangleExclamation;
  faBolt = faBolt;

  servers: Server[] = [];
  selectedServerId: number | null = null;

  mode: 'manual' | 'ssh' = 'manual';

  // Manuel kurulum
  isGenerating = false;
  enroll: EnrollTokenResponse | null = null;

  // SSH kurulum
  ssh = { host: '', port: '22', user: 'root', authType: 'password' as 'password' | 'key', password: '', privateKey: '' };
  isInstalling = false;
  sshResult: SSHInstallResult | null = null;

  ngOnInit(): void {
    this.api.getServers(1, 500).subscribe({
      next: (r) => { this.servers = r.data; },
      error: () => this.toastr.error('Sunucular yüklenemedi.'),
    });
  }

  get selectedServer(): Server | undefined {
    return this.servers.find(s => s.id === this.selectedServerId);
  }

  onServerChange(): void {
    this.enroll = null;
    this.sshResult = null;
    const s = this.selectedServer;
    if (s) this.ssh.host = s.ip_address;
  }

  generate(): void {
    if (!this.selectedServerId) { this.toastr.warning('Önce bir sunucu seçin.'); return; }
    this.isGenerating = true;
    this.enroll = null;
    this.api.generateEnrollToken(this.selectedServerId).subscribe({
      next: (res) => {
        this.enroll = res;
        this.isGenerating = false;
        if (!res.binary_available) {
          this.toastr.warning('Sunucuda agent binary bulunamadı; script binary indiremeyecek. GUARDIAN_AGENT_BINARY_PATH ayarlayın.', 'Uyarı', { timeOut: 8000 });
        }
      },
      error: (err) => {
        this.isGenerating = false;
        this.toastr.error(err?.error || 'Kurulum komutu üretilemedi.');
      },
    });
  }

  install(): void {
    if (!this.selectedServerId) { this.toastr.warning('Önce bir sunucu seçin.'); return; }
    if (!this.ssh.user.trim()) { this.toastr.warning('SSH kullanıcı adı gerekli.'); return; }
    if (this.ssh.authType === 'password' && !this.ssh.password) { this.toastr.warning('SSH parolası gerekli.'); return; }
    if (this.ssh.authType === 'key' && !this.ssh.privateKey.trim()) { this.toastr.warning('Özel anahtar gerekli.'); return; }

    this.isInstalling = true;
    this.sshResult = null;
    this.api.sshInstallAgent(this.selectedServerId, {
      ssh_host: this.ssh.host.trim() || undefined,
      ssh_port: this.ssh.port.trim() || undefined,
      ssh_user: this.ssh.user.trim(),
      ssh_password: this.ssh.authType === 'password' ? this.ssh.password : undefined,
      ssh_private_key: this.ssh.authType === 'key' ? this.ssh.privateKey : undefined,
    }).subscribe({
      next: (res) => {
        this.sshResult = res;
        this.isInstalling = false;
        if (res.success) this.toastr.success('Kurulum tamamlandı.');
        else this.toastr.error('Kurulum başarısız — çıktıyı inceleyin.');
      },
      error: (err) => {
        this.isInstalling = false;
        this.sshResult = { success: false, output: '', error: err?.error || 'Bağlantı/kurulum hatası.' };
        this.toastr.error(err?.error || 'Kurulum başarısız.');
      },
    });
  }
}
