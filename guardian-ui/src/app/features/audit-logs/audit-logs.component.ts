import { Component, OnInit, OnDestroy, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faClipboardList, faMagnifyingGlass, faSync } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, AuditLog } from '../../core/services/api-client.service';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';

/** Denetim aksiyonlarının Türkçe etiketleri. */
const ACTION_LABELS: Record<string, string> = {
  CREATE_SERVER: 'Sunucu Oluştur', UPDATE_SERVER: 'Sunucu Güncelle', PATCH_SERVER: 'Sunucu Düzenle', DELETE_SERVER: 'Sunucu Sil',
  CREATE_USER: 'Kullanıcı Oluştur', PATCH_USER: 'Kullanıcı Düzenle', DELETE_USER: 'Kullanıcı Sil',
  CREATE_KEY: 'Anahtar Oluştur', PATCH_KEY: 'Anahtar Düzenle', DELETE_KEY: 'Anahtar Sil',
  CREATE_RULE: 'Kural Oluştur', UPDATE_RULE: 'Kural Güncelle', PATCH_RULE: 'Kural Düzenle', DELETE_RULE: 'Kural Sil',
  TERMINATE_SESSION: 'Oturum Sonlandır', BAN_KEY: 'Anahtar Yasakla', UNBAN_KEY: 'Yasak Kaldır',
  LOGIN: 'Giriş', UPDATE_SETTINGS: 'Ayar Güncelle',
  CREATE_ADMIN: 'Yönetici Oluştur', UPDATE_ADMIN: 'Yönetici Güncelle', DELETE_ADMIN: 'Yönetici Sil',
  REQUEST_ACCESS: 'Erişim Talebi', APPROVE_ACCESS: 'Talep Onayla', REJECT_ACCESS: 'Talep Reddet',
};

@Component({
  selector: 'app-audit-logs',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, PaginationComponent],
  templateUrl: './audit-logs.component.html',
})
export class AuditLogsComponent implements OnInit, OnDestroy {
  private api = inject(ApiClientService);
  private toastr = inject(ToastrService);

  faSpinner = faSpinner;
  faClipboardList = faClipboardList;
  faSearch = faMagnifyingGlass;
  faSync = faSync;

  logs: AuditLog[] = [];
  isLoading = true;

  currentPage = 1;
  limit = 20;
  totalRecords = 0;

  searchTerm = '';
  actionFilter = '';
  statusFilter = '';

  // Filtre açılır listesi: kategorilere ayrılmış aksiyonlar.
  readonly actionOptions = Object.keys(ACTION_LABELS).map(k => ({ value: k, label: ACTION_LABELS[k] }));

  private search$ = new Subject<void>();
  private sub?: Subscription;

  ngOnInit(): void {
    this.load(1);
    this.sub = this.search$.pipe(debounceTime(300)).subscribe(() => this.load(1));
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }

  onSearchInput(): void { this.search$.next(); }

  onFilterChange(): void { this.load(1); }

  load(page: number): void {
    this.isLoading = true;
    this.currentPage = page;
    this.api.getAuditLogs(page, this.limit, this.searchTerm, this.actionFilter, this.statusFilter).subscribe({
      next: (res) => {
        this.logs = res.data ?? [];
        this.totalRecords = res.total_records;
        this.isLoading = false;
      },
      error: () => { this.isLoading = false; this.toastr.error('Denetim kayıtları yüklenemedi.'); },
    });
  }

  actionLabel(action: string): string {
    return ACTION_LABELS[action] ?? action;
  }
}
