import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiClientService, Session, SessionDetails } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { Router } from '@angular/router';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { 
  faSync, 
  faClockRotateLeft,
  faEye,
  faPlayCircle,
  faInfoCircle,
  faPowerOff,
  faSpinner,
  faTerminal
} from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-sessions',
  standalone: true,
  imports: [CommonModule, FaIconComponent],
  templateUrl: './sessions.component.html',
  styleUrl: './sessions.component.scss'
})
export class SessionsComponent implements OnInit {
   faSync = faSync;
  faSessions = faClockRotateLeft;
  faLive = faEye;
  faReplay = faPlayCircle;
  faDetails = faInfoCircle;
  faTerminate = faPowerOff;
  faSpinner = faSpinner;
  faTerminal = faTerminal;
  
  sessions: Session[] = [];
  isLoading = true;
  error: string | null = null;

  isModalOpen = false;
  selectedSessionDetails: SessionDetails | null = null;
  isLoadingDetails = false;

  currentPage = 1;
  limit = 8;
  totalRecords = 0;
  totalPages = 0;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private router: Router
  ) {}

  ngOnInit(): void {
    this.loadSessions(this.currentPage);
  }

  loadSessions(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getSessions(this.currentPage, this.limit).subscribe({
      next: (response) => {
        this.sessions = response.data;
        this.totalRecords = response.total_records;
        this.totalPages = Math.ceil(this.totalRecords / this.limit) || 1;
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Oturum verisi alınırken hata oluştu:', err);
        this.error = 'Oturum verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

   nextPage(): void {
    if (this.currentPage < this.totalPages) {
      this.loadSessions(this.currentPage + 1);
    }
  }

  prevPage(): void {
    if (this.currentPage > 1) {
      this.loadSessions(this.currentPage - 1);
    }
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

    this.apiClient.getSessionDetails(sessionId).subscribe({
      next: (data) => {
        this.selectedSessionDetails = data;
        this.isLoadingDetails = false;
      },
      error: (err) => {
        this.toastr.error('Oturum detayları alınamadı.', 'Hata');
        this.isLoadingDetails = false;
        this.closeModal();
      }
    });
  }
  
  terminate(session: Session): void {
    if (!confirm(`EMİN MİSİNİZ?\nAktif oturum ID ${session.id} zorla sonlandırılacak!`)) return;

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