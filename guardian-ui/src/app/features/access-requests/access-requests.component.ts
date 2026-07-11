import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faInbox, faCheck, faXmark, faPlus, faClock } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';
import { forkJoin } from 'rxjs';

import { ApiClientService, AccessRequest, Server, User, Key } from '../../core/services/api-client.service';
import { AuthService } from '../../core/services/auth.service';
import { StatusBadgeComponent } from '../../shared/ui/status-badge/status-badge.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';

interface RequestForm {
  server_id: number | null;
  public_key_id: number | null;
  system_user_id: number | null;
  valid_from: string;
  valid_until: string;
  reason: string;
}

@Component({
  selector: 'app-access-requests',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, StatusBadgeComponent],
  templateUrl: './access-requests.component.html',
})
export class AccessRequestsComponent implements OnInit {
  private api = inject(ApiClientService);
  private auth = inject(AuthService);
  private toastr = inject(ToastrService);
  private confirm = inject(ConfirmDialogService);

  faSpinner = faSpinner;
  faInbox = faInbox;
  faCheck = faCheck;
  faXmark = faXmark;
  faPlus = faPlus;
  faClock = faClock;

  isLoading = true;
  requests: AccessRequest[] = [];

  // Durum filtresi sekmeleri.
  readonly tabs = [
    { key: 'awaiting_approval', label: 'Onay Bekleyen' },
    { key: 'pending,active', label: 'Onaylanan' },
    { key: 'rejected', label: 'Reddedilen' },
    { key: '', label: 'Tümü' },
  ];
  activeTab = 'awaiting_approval';

  // Talep oluşturma formu.
  showForm = false;
  isSubmitting = false;
  servers: Server[] = [];
  users: User[] = [];
  keys: Key[] = [];
  form: RequestForm = this.emptyForm();

  get canApprove(): boolean { return this.auth.hasRole('admin'); }
  get canRequest(): boolean { return this.auth.hasRole('operator'); }

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.isLoading = true;
    this.api.getAccessRequests(this.activeTab).subscribe({
      next: (rows) => { this.requests = rows; this.isLoading = false; },
      error: () => { this.isLoading = false; this.toastr.error('Talepler yüklenemedi.'); },
    });
  }

  selectTab(key: string): void {
    this.activeTab = key;
    this.load();
  }

  openForm(): void {
    this.form = this.emptyForm();
    this.showForm = true;
    // Dropdown verilerini (sunucu/kullanıcı/anahtar) yükle.
    forkJoin({
      servers: this.api.getServers(1, 500),
      users: this.api.getUsers(1, 500),
      keys: this.api.getKeys(1, 500),
    }).subscribe({
      next: (r) => {
        this.servers = r.servers.data;
        this.users = r.users.data;
        this.keys = r.keys.data;
      },
      error: () => this.toastr.error('Seçenekler yüklenemedi.'),
    });
  }

  submit(): void {
    if (!this.form.server_id || !this.form.public_key_id || !this.form.system_user_id) {
      this.toastr.warning('Sunucu, anahtar ve kullanıcı seçin.');
      return;
    }
    if (!this.form.valid_until) {
      this.toastr.warning('Geçerlilik bitişi girin.');
      return;
    }
    this.isSubmitting = true;
    this.api.createAccessRequest({
      server_id: this.form.server_id,
      public_key_id: this.form.public_key_id,
      system_user_id: this.form.system_user_id,
      valid_from: new Date(this.form.valid_from).toISOString(),
      valid_until: new Date(this.form.valid_until).toISOString(),
      reason: this.form.reason,
    }).subscribe({
      next: () => {
        this.isSubmitting = false;
        this.showForm = false;
        this.toastr.success('Erişim talebi oluşturuldu.');
        this.activeTab = 'awaiting_approval';
        this.load();
      },
      error: (err) => {
        this.isSubmitting = false;
        this.toastr.error(err?.error || 'Talep oluşturulamadı.');
      },
    });
  }

  async approve(req: AccessRequest): Promise<void> {
    const ok = await this.confirm.confirm({
      title: 'Talebi Onayla',
      message: `${req.username}@${req.server_hostname} erişimi onaylanacak. Onay sonrası kural, başlangıç zamanı geldiğinde otomatik etkinleşir.`,
      confirmText: 'Onayla',
    });
    if (!ok) return;
    this.api.approveAccessRequest(req.id).subscribe({
      next: () => { this.toastr.success('Talep onaylandı.'); this.load(); },
      error: (err) => this.toastr.error(err?.error || 'Onaylanamadı.'),
    });
  }

  async reject(req: AccessRequest): Promise<void> {
    const ok = await this.confirm.confirm({
      title: 'Talebi Reddet',
      message: `${req.username}@${req.server_hostname} erişim talebi reddedilecek.`,
      confirmText: 'Reddet',
      danger: true,
    });
    if (!ok) return;
    this.api.rejectAccessRequest(req.id).subscribe({
      next: () => { this.toastr.success('Talep reddedildi.'); this.load(); },
      error: (err) => this.toastr.error(err?.error || 'Reddedilemedi.'),
    });
  }

  private emptyForm(): RequestForm {
    const now = new Date();
    const later = new Date(now.getTime() + 2 * 60 * 60 * 1000);
    return {
      server_id: null,
      public_key_id: null,
      system_user_id: null,
      valid_from: this.toLocalInput(now),
      valid_until: this.toLocalInput(later),
      reason: '',
    };
  }

  /** Date → "YYYY-MM-DDTHH:mm" (datetime-local input formatı). */
  private toLocalInput(d: Date): string {
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }
}
