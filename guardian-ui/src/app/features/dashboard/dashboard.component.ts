import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, DashboardStats, ChartData, SeriesChartData, ActiveSessionInfo, AuditLog } from '../../core/services/api-client.service';
import { NgxChartsModule, Color, ScaleType, LegendPosition } from '@swimlane/ngx-charts';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { 
  faServer, faUsers, faWifi, faCalendarXmark, faSpinner, faClock, 
  faKey, faCalendarDay, faExclamationCircle, faEye, faClipboardList 
} from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, FaIconComponent, RouterLink, NgxChartsModule],
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit {
  isLoading = true;
  error: string | null = null;

  stats: DashboardStats | null = null;
  sessionActivity: SeriesChartData[] = [];
  topServers: ChartData[] = [];
  activeSessions: ActiveSessionInfo[] = [];
  auditLogs: AuditLog[] = [];
  
  view: [number, number] = [700, 300]; 
  
    colorScheme: Color = {
    name: 'guardianDark',
    selectable: true,
    group: ScaleType.Ordinal,
    domain: ['#2f81f7', '#3fb950', '#d29922', '#f85149', '#a371f7'] // Mavi, Yeşil, Sarı, Kırmızı, Mor
  };
   
  gradient: boolean = false;
  showXAxis = true;
  showYAxis = true;
  showLegend = false;
  showXAxisLabel = true;
  xAxisLabel = 'Tarih';
  showYAxisLabel = true;
  yAxisLabel = 'Oturum Sayısı';

  pieGradient: boolean = true;
  showPieLabels: boolean = true;
  isDoughnut: boolean = false;
  legendPosition: LegendPosition = LegendPosition.Below;

  faWifi = faWifi;
  faCalendarXmark = faCalendarXmark;
  faServer = faServer;
  faUsers = faUsers;
  faSpinner = faSpinner;
  faPending = faClock;
  faKeys = faKey;
  faToday = faCalendarDay;
  faFailed = faExclamationCircle;
  faLive = faEye;
  faAudit = faClipboardList;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private router: Router
  ) {}

  ngOnInit(): void {
    this.loadDashboardData(); 
  }

  loadDashboardData(): void { 
    this.isLoading = true;
    this.error = null;

    forkJoin({
      stats: this.apiClient.getDashboardStats(),
      activity: this.apiClient.getSessionActivity(),
      topServers: this.apiClient.getTopServers(),
      activeSessions: this.apiClient.getActiveSessionsList(),
      auditLogs: this.apiClient.getAuditLogStream()
    }).subscribe({
      next: (data) => {
        this.stats = data.stats;
        this.sessionActivity = [{ name: 'Oturumlar', series: data.activity }];
        this.topServers = data.topServers;
        this.activeSessions = data.activeSessions;
        this.auditLogs = data.auditLogs;
        this.isLoading = false;
      },
      error: (err) => {
        this.error = 'Dashboard verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  openLiveReplay(sessionId: number): void {
    const url = this.router.serializeUrl(this.router.createUrlTree(['/live', sessionId]));
    window.open(url, '_blank');
  }
}