import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { ApiClientService, Session, SessionDetails, KeyBanStatus } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { Router, RouterLink } from '@angular/router';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import {
  faSync,
  faClockRotateLeft,
  faEye,
  faPlayCircle,
  faInfoCircle,
  faPowerOff,
  faSpinner,
  faTerminal,
  faMagnifyingGlass,
  faBan
} from '@fortawesome/free-solid-svg-icons';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';
import { StatusBadgeComponent } from '../../shared/ui/status-badge/status-badge.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';
import { BanDialogService } from '../../shared/ui/ban-dialog/ban-dialog.service';
import { formatDuration } from '../../shared/time-utils';

/** Durum filtre çipleri: etiket → backend'e gidecek durum listesi ('' = filtre yok). */
const STATUS_FILTERS: { label: string; statuses: string }[] = [
  { label: 'Tümü', statuses: '' },
  { label: 'Aktif', statuses: 'active' },
  { label: 'Tamamlanan', statuses: 'ended,timed_out,terminated_by_admin,terminated_by_expiry,terminated_by_ban,terminated_by_rule_deletion' },
  { label: 'Hatalı', statuses: 'error,lost_contact,error_pid_creation,error_ssh_session,error_ws_connect,error_shell' },
];

@Component({
  selector: 'app-sessions',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, RouterLink, PaginationComponent, StatusBadgeComponent],
  templateUrl: './sessions.component.html',
  styleUrl: './sessions.component.scss'
})
export class SessionsComponent implements OnInit, OnDestroy {
  faSync = faSync;
  faSessions = faClockRotateLeft;
  faLive = faEye;
  faReplay = faPlayCircle;
  faDetails = faInfoCircle;
  faTerminate = faPowerOff;
  faSpinner = faSpinner;
  faTerminal = faTerminal;
  faSearch = faMagnifyingGlass;
  faBan = faBan;

  sessions: Session[] = [];
  isLoading = true;
  error: string | null = null;

  readonly statusFilters = STATUS_FILTERS;
  activeFilter = STATUS_FILTERS[0];
  searchTerm = '';
  private search$ = new Subject<string>();
  private searchSub?: Subscription;

  isModalOpen = false;
  selectedSessionDetails: SessionDetails | null = null;
  isLoadingDetails = false;
  keyBanStatus: KeyBanStatus | null = null;

  currentPage = 1;
  limit = 8;
  totalRecords = 0;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private router: Router,
    private confirmDialog: ConfirmDialogService,
    private banDialog: BanDialogService
  ) {}

  ngOnInit(): void {
    this.loadSessions(1);
    this.searchSub = this.search$.pipe(debounceTime(300)).subscribe(() => this.loadSessions(1));
  }

  ngOnDestroy(): void {
    this.searchSub?.unsubscribe();
  }

  onSearchInput(): void {
    this.search$.next(this.searchTerm);
  }

  setFilter(filter: { label: string; statuses: string }): void {
    this.activeFilter = filter;
    this.loadSessions(1);
  }

  loadSessions(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getSessions(page, this.limit, this.searchTerm, this.activeFilter.statuses).subscribe({
      next: (response) => {
        this.sessions = response.data ?? [];
        this.totalRecords = response.total_records;
        this.isLoading = false;
      },
      error: () => {
        this.error = 'Oturum verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  duration(session: Session): string {
    const end = session.end_time ? new Date(session.end_time).getTime() : Date.now();
    return formatDuration(new Date(session.start_time).getTime(), end);
  }

  openLiveReplay(sessionId: number): void {
    const url = this.router.serializeUrl(this.router.createUrlTree(['/live', sessionId]));
    window.open(url, '_blank');
  }

  openReplay(sessionId: number): void {
    const url = this.router.serializeUrl(this.router.createUrlTree(['/replay', sessionId]));
    window.open(url, '_blank');
  }

  showCommands(sessionId: number): void {
    this.isModalOpen = true;
    this.isLoadingDetails = true;
    this.selectedSessionDetails = null;
    this.keyBanStatus = null;

    this.apiClient.getSessionDetails(sessionId).subscribe({
      next: (data) => {
        this.selectedSessionDetails = data;
        this.isLoadingDetails = false;
        const keyId = data.session_info.public_key_id;
        if (keyId) {
          this.apiClient.getKeyBanStatus(keyId).subscribe({
            next: (status) => this.keyBanStatus = status,
            error: () => this.keyBanStatus = null
          });
        }
      },
      error: () => {
        this.toastr.error('Oturum detayları alınamadı.', 'Hata');
        this.isLoadingDetails = false;
        this.closeModal();
      }
    });
  }

  async banSelectedKey(): Promise<void> {
    const keyId = this.selectedSessionDetails?.session_info.public_key_id;
    const keyName = this.selectedSessionDetails?.session_info.public_key_name || `#${keyId}`;
    if (!keyId) return;

    const result = await this.banDialog.open({ keyName });
    if (!result) return;

    this.apiClient.banKey(keyId, result.durationMinutes, result.reason).subscribe({
      next: (ban) => {
        this.toastr.success(`Anahtar ${result.durationMinutes} dakika süreyle yasaklandı, bağlı kurallar iptal edildi.`, 'Yasaklandı');
        this.keyBanStatus = { banned: true, ban };
        this.loadSessions(this.currentPage);
      },
      error: (err) => {
        this.toastr.error(err.error || 'Anahtar yasaklanamadı.', 'Hata');
      }
    });
  }

  async unbanSelectedKey(): Promise<void> {
    const keyId = this.selectedSessionDetails?.session_info.public_key_id;
    if (!keyId) return;
    const ok = await this.confirmDialog.confirm({
      title: 'Yasağı Kaldır',
      message: 'Bu anahtar üzerindeki yasak şimdi kaldırılacak. Emin misiniz?',
      confirmText: 'Yasağı Kaldır',
    });
    if (!ok) return;

    this.apiClient.unbanKey(keyId).subscribe({
      next: () => {
        this.toastr.success('Yasak kaldırıldı.');
        this.keyBanStatus = { banned: false };
      },
      error: () => this.toastr.error('Yasak kaldırılamadı.', 'Hata')
    });
  }

  async terminate(session: Session): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Oturumu Zorla Sonlandır',
      message: `#${session.id} — ${session.username}@${session.server_hostname} oturumu anında kesilecek.`,
      confirmText: 'Sonlandır',
      danger: true,
    });
    if (!ok) return;

    this.apiClient.terminateSession(session.id).subscribe({
      next: (successMessage) => {
        this.toastr.success(successMessage, 'Başarılı');
        this.loadSessions(this.currentPage);
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Sonlandırma Başarısız');
      }
    });
  }

  closeModal(): void {
    this.isModalOpen = false;
    this.selectedSessionDetails = null;
  }
}
