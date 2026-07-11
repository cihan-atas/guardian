import { Component, OnInit, OnDestroy, ElementRef, ViewChild, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { forkJoin } from 'rxjs';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, DashboardStats, ChartData, SeriesChartData, ActiveSessionInfo, AuditLog, Rule, Session, CommandStat } from '../../core/services/api-client.service';
import { NgxChartsModule, Color, ScaleType, LegendPosition } from '@swimlane/ngx-charts';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { IconDefinition } from '@fortawesome/fontawesome-svg-core';
import {
  faServer, faUsers, faWifi, faCalendarXmark, faSpinner, faClock,
  faKey, faCalendarDay, faExclamationCircle, faEye, faClipboardList, faBan, faSync, faTerminal,
  faArrowRight, faChartLine, faShieldHalved, faBoltLightning, faHourglassHalf, faClockRotateLeft,
  faMagnifyingGlass, faChevronRight, faTriangleExclamation, faFilter
} from '@fortawesome/free-solid-svg-icons';

import { FormsModule } from '@angular/forms';

/** Üstteki metrik kartı. */
export interface KpiTile {
  label: string;
  value: number;
  icon: IconDefinition;
  link: string;
  tone: 'live' | 'accent' | 'warning' | 'danger' | 'neutral';
  hint?: string;
  pulse?: boolean;
  alert?: boolean;
}

import { statusLabel, statusTone } from '../../shared/ui/status-labels';
import { formatRemaining, formatDuration, windowPercent } from '../../shared/time-utils';

function translateLabels(data: ChartData[]): ChartData[] {
  return data.map(d => ({ ...d, name: statusLabel(d.name) }));
}

type ChartKey = 'line' | 'topServers' | 'sessionStatus' | 'ruleStatus' | 'userActivity' | 'hourlyActivity';

/** Baz komuta (ilk kelime) göre gruplanmış, açılıp tam varyantları
 * görülebilen komut kümesi. */
export interface CommandGroup {
  base: string;
  count: number;
  risky: boolean;
  variants: { command: string; count: number; servers: string[]; risky: boolean }[];
}

/** Riskli sayılan komut kalıpları (görüntüleme katmanı — backend'i etkilemez). */
const RISKY_PATTERNS: RegExp[] = [
  /\brm\s+-[rf]/, /\bsudo\b/, /\bchmod\s+777/, /\bchown\b/, /\bdd\s+if=/,
  /\bmkfs/, /\b(curl|wget)\b.*\|\s*(sh|bash)/, /\bnc\b/, /\bncat\b/,
  /\biptables\s+-F/, /\bhistory\s+-c/, /\b(shutdown|reboot|poweroff)\b/,
  /\bpasswd\b/, /\buseradd\b/, /\buserdel\b/, /\bkill(all)?\b/, /:\(\)\{/,
  /\/etc\/(passwd|shadow|sudoers)/, /\bsystemctl\s+(stop|disable)/,
];

function isRiskyCommand(cmd: string): boolean {
  return RISKY_PATTERNS.some(re => re.test(cmd));
}

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [CommonModule, FaIconComponent, RouterLink, NgxChartsModule, FormsModule],
  templateUrl: './dashboard.component.html',
  styleUrls: ['./dashboard.component.scss']
})
export class DashboardComponent implements OnInit, AfterViewInit, OnDestroy {
  @ViewChild('lineChartCard') lineChartCard!: ElementRef<HTMLElement>;
  @ViewChild('topServersCard') topServersCard!: ElementRef<HTMLElement>;
  @ViewChild('sessionStatusCard') sessionStatusCard!: ElementRef<HTMLElement>;
  @ViewChild('ruleStatusCard') ruleStatusCard!: ElementRef<HTMLElement>;
  @ViewChild('userActivityCard') userActivityCard!: ElementRef<HTMLElement>;
  @ViewChild('hourlyActivityCard') hourlyActivityCard!: ElementRef<HTMLElement>;

  isLoading = true;
  error: string | null = null;
  readonly today = new Date();

  stats: DashboardStats | null = null;
  primaryKpis: KpiTile[] = [];
  inventory: KpiTile[] = [];
  sessionActivity: SeriesChartData[] = [];
  topServers: ChartData[] = [];
  sessionStatusBreakdown: ChartData[] = [];
  ruleStatusBreakdown: ChartData[] = [];
  userActivity: ChartData[] = [];
  hourlyActivity: ChartData[] = [];
  activeSessions: ActiveSessionInfo[] = [];
  auditLogs: AuditLog[] = [];
  accessWindows: Rule[] = [];
  recentSessions: Session[] = [];

  // --- Komut analitiği (tam komut + sunucu filtresi + drill-down) ---
  private commandStats: CommandStat[] = [];
  commandServers: string[] = [];       // filtre için sunucu listesi
  selectedCmdServer = '';              // '' = tüm sunucular
  commandSearch = '';
  commandGroups: CommandGroup[] = [];  // aktif filtreye göre hesaplanmış
  maxCommandCount = 0;                 // çubuk oranları için
  expandedBases = new Set<string>();
  totalCommandRuns = 0;                // aktif filtredeki toplam çalıştırma

  /** Geri sayımların canlı hissettirmesi için 30 sn'de bir güncellenir. */
  now = Date.now();
  private nowTimer?: ReturnType<typeof setInterval>;

  views: Record<ChartKey, [number, number]> = {
    line: [700, 360],
    topServers: [350, 300],
    sessionStatus: [350, 300],
    ruleStatus: [350, 300],
    userActivity: [500, 360],
    hourlyActivity: [500, 320],
  };

  private resizeObserver?: ResizeObserver;

  colorScheme: Color = {
    name: 'guardianDark',
    selectable: true,
    group: ScaleType.Ordinal,
    domain: ['#2f81f7', '#3fb950', '#d29922', '#f85149', '#a371f7', '#39c5cf', '#db61a2']
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
  isDoughnut: boolean = true;
  legendPosition: LegendPosition = LegendPosition.Below;

  faWifi = faWifi;
  faCalendarXmark = faCalendarXmark;
  faServer = faServer;
  faUsers = faUsers;
  faSpinner = faSpinner;
  faPending = faClock;
  faClock = faClock;
  faKeys = faKey;
  faToday = faCalendarDay;
  faFailed = faExclamationCircle;
  faLive = faEye;
  faAudit = faClipboardList;
  faBan = faBan;
  faSync = faSync;
  faTerminal = faTerminal;
  faArrowRight = faArrowRight;
  faChartLine = faChartLine;
  faShield = faShieldHalved;
  faBolt = faBoltLightning;
  faHourglass = faHourglassHalf;
  faHistory = faClockRotateLeft;
  faSearch = faMagnifyingGlass;
  faChevronRight = faChevronRight;
  faWarn = faTriangleExclamation;
  faFilter = faFilter;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private router: Router
  ) {}

  ngOnInit(): void {
    this.loadDashboardData();
    this.nowTimer = setInterval(() => (this.now = Date.now()), 30000);
  }

  ngAfterViewInit(): void {
    // Grafikleri sabit piksel boyutu yerine kart konteynerinin gerçek
    // genişliğine göre boyutlandırıyoruz; aksi halde geniş ekranlarda taşıyor,
    // dar ekranlarda kırpılıyordu (ngx-charts'ın kendisi otomatik responsive
    // değil, [view] input'una gerçek piksel boyutu vermek gerekiyor).
    this.resizeObserver = new ResizeObserver(() => this.recalculateChartSizes());
    const refs = this.chartRefs();
    (Object.keys(refs) as ChartKey[]).forEach(key => {
      if (refs[key]?.nativeElement) this.resizeObserver!.observe(refs[key].nativeElement);
    });
    setTimeout(() => this.recalculateChartSizes(), 0);
  }

  ngOnDestroy(): void {
    this.resizeObserver?.disconnect();
    if (this.nowTimer) clearInterval(this.nowTimer);
  }

  private chartRefs(): Record<ChartKey, ElementRef<HTMLElement>> {
    return {
      line: this.lineChartCard,
      topServers: this.topServersCard,
      sessionStatus: this.sessionStatusCard,
      ruleStatus: this.ruleStatusCard,
      userActivity: this.userActivityCard,
      hourlyActivity: this.hourlyActivityCard,
    };
  }

  private recalculateChartSizes(): void {
    const heights: Record<ChartKey, number> = {
      line: 360, topServers: 300, sessionStatus: 300, ruleStatus: 300,
      userActivity: 360, hourlyActivity: 320,
    };
    const refs = this.chartRefs();
    (Object.keys(refs) as ChartKey[]).forEach(key => {
      const el = refs[key]?.nativeElement;
      if (el && el.clientWidth > 0) {
        this.views[key] = [el.clientWidth, heights[key]];
      }
    });
  }

  loadDashboardData(): void {
    this.isLoading = true;
    this.error = null;

    forkJoin({
      stats: this.apiClient.getDashboardStats(),
      activity: this.apiClient.getSessionActivity(),
      topServers: this.apiClient.getTopServers(),
      sessionStatus: this.apiClient.getSessionStatusBreakdown(),
      ruleStatus: this.apiClient.getRuleStatusBreakdown(),
      commandStats: this.apiClient.getCommandStats(),
      userActivity: this.apiClient.getUserActivity(),
      hourlyActivity: this.apiClient.getHourlyActivity(),
      activeSessions: this.apiClient.getActiveSessionsList(),
      auditLogs: this.apiClient.getAuditLogStream(),
      rules: this.apiClient.getRules(1, 100),
      sessions: this.apiClient.getSessions(1, 7)
    }).subscribe({
      next: (data) => {
        this.stats = data.stats;
        this.buildKpis(data.stats);
        this.sessionActivity = [{ name: 'Oturumlar', series: data.activity ?? [] }];
        this.topServers = data.topServers ?? [];
        this.sessionStatusBreakdown = translateLabels(data.sessionStatus ?? []);
        this.ruleStatusBreakdown = translateLabels(data.ruleStatus ?? []);
        this.setCommandStats(data.commandStats ?? []);
        this.userActivity = data.userActivity ?? [];
        this.hourlyActivity = data.hourlyActivity ?? [];
        this.activeSessions = data.activeSessions ?? [];
        this.auditLogs = data.auditLogs ?? [];
        this.accessWindows = (data.rules?.data ?? [])
          .filter(r => r.status === 'active')
          .sort((a, b) => new Date(a.valid_until).getTime() - new Date(b.valid_until).getTime())
          .slice(0, 6);
        this.recentSessions = data.sessions?.data ?? [];
        this.isLoading = false;
        setTimeout(() => this.recalculateChartSizes(), 0);
      },
      error: (err) => {
        this.error = 'Dashboard verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  /** stats geldikçe üstteki metrik + envanter kartlarını üretir. */
  private buildKpis(s: DashboardStats): void {
    const securityAlerts = s.failed_sessions + s.banned_keys;
    this.primaryKpis = [
      { label: 'Aktif Oturum', value: s.active_sessions, icon: this.faLive, tone: 'live',
        link: '/sessions', hint: 'şu an bağlı', pulse: s.active_sessions > 0 },
      { label: 'Bugünkü Oturum', value: s.today_sessions, icon: this.faToday, tone: 'accent',
        link: '/sessions', hint: 'son 24 saat' },
      { label: 'Bekleyen Kural', value: s.pending_rules, icon: this.faPending, tone: 'warning',
        link: '/rules', hint: 'başlangıç bekliyor' },
      { label: 'Güvenlik Uyarısı', value: securityAlerts, icon: this.faShield, tone: 'danger',
        link: '/sessions', hint: `${s.failed_sessions} hatalı · ${s.banned_keys} yasaklı`,
        alert: securityAlerts > 0 },
    ];
    this.inventory = [
      { label: 'Sunucu', value: s.total_servers, icon: this.faServer, tone: 'neutral', link: '/servers' },
      { label: 'Kullanıcı', value: s.total_users, icon: this.faUsers, tone: 'neutral', link: '/users' },
      { label: 'SSH Anahtarı', value: s.total_keys, icon: this.faKeys, tone: 'neutral', link: '/keys' },
      { label: 'Eskimiş Kural', value: s.expired_rules, icon: this.faCalendarXmark, tone: 'neutral', link: '/rules' },
    ];
  }

  /** KPI kartındaki ikon kabarcığının renk sınıfları. */
  toneIconClass(tone: KpiTile['tone']): string {
    switch (tone) {
      case 'live': return 'bg-live/10 text-live';
      case 'accent': return 'bg-accent/10 text-accent';
      case 'warning': return 'bg-warning-500/10 text-warning-400';
      case 'danger': return 'bg-danger-500/10 text-danger-400';
      default: return 'bg-dark-interactive text-text-secondary';
    }
  }

  // --- Erişim pencereleri (JIT) ---

  windowPercent(r: Rule): number {
    return windowPercent(r.valid_from, r.valid_until, this.now);
  }

  windowRemainingMs(r: Rule): number {
    return new Date(r.valid_until).getTime() - this.now;
  }

  isExpiringSoon(r: Rule): boolean {
    const ms = this.windowRemainingMs(r);
    return ms > 0 && ms < 30 * 60000;
  }

  formatRemaining(ms: number): string {
    return formatRemaining(ms);
  }

  // --- Son oturumlar ---

  sessionDuration(s: Session): string {
    const end = s.end_time ? new Date(s.end_time).getTime() : this.now;
    return formatDuration(new Date(s.start_time).getTime(), end);
  }

  statusLabel(status: string): string {
    return statusLabel(status);
  }

  sessionBadgeClass(status: string): string {
    switch (statusTone(status)) {
      case 'live': return 'bg-live/10 text-live';
      case 'success': return 'bg-success-500/10 text-success-400';
      case 'danger': return 'bg-danger-500/10 text-danger-400';
      case 'warning': return 'bg-warning-500/10 text-warning-400';
      default: return 'bg-dark-interactive text-text-secondary';
    }
  }

  // --- Komut analitiği ---

  private setCommandStats(stats: CommandStat[]): void {
    this.commandStats = stats;
    // Filtre için tüm sunucu adlarını topla.
    const servers = new Set<string>();
    stats.forEach(s => Object.keys(s.servers ?? {}).forEach(h => servers.add(h)));
    this.commandServers = Array.from(servers).sort();
    this.rebuildCommandGroups();
  }

  onCmdServerChange(host: string): void {
    this.selectedCmdServer = host;
    this.rebuildCommandGroups();
  }

  onCommandSearch(): void {
    this.rebuildCommandGroups();
  }

  toggleBase(base: string): void {
    if (this.expandedBases.has(base)) this.expandedBases.delete(base);
    else this.expandedBases.add(base);
  }

  isExpanded(base: string): boolean {
    return this.expandedBases.has(base);
  }

  isRisky(cmd: string): boolean {
    return isRiskyCommand(cmd);
  }

  cmdBarPercent(count: number): number {
    return this.maxCommandCount > 0 ? (count / this.maxCommandCount) * 100 : 0;
  }

  /** Aktif sunucu filtresi + aramaya göre komut gruplarını yeniden hesaplar. */
  private rebuildCommandGroups(): void {
    const server = this.selectedCmdServer;
    const search = this.commandSearch.trim().toLowerCase();

    // 1) Filtreye göre her tam komutun sayısını belirle.
    type Variant = { command: string; count: number; servers: string[]; risky: boolean };
    const variants: Variant[] = [];
    for (const s of this.commandStats) {
      const count = server ? (s.servers?.[server] ?? 0) : s.count;
      if (count <= 0) continue;
      if (search && !s.command.toLowerCase().includes(search) && !s.base.toLowerCase().includes(search)) continue;
      variants.push({
        command: s.command,
        count,
        servers: Object.keys(s.servers ?? {}).sort(),
        risky: isRiskyCommand(s.command),
      });
    }

    // 2) Baz komuta göre grupla.
    const groupMap = new Map<string, CommandGroup>();
    for (const v of variants) {
      const base = v.command.split(/\s+/)[0] || v.command;
      let g = groupMap.get(base);
      if (!g) {
        g = { base, count: 0, risky: false, variants: [] };
        groupMap.set(base, g);
      }
      g.count += v.count;
      g.variants.push(v);
      if (v.risky) g.risky = true;
    }

    const groups = Array.from(groupMap.values());
    groups.forEach(g => g.variants.sort((a, b) => b.count - a.count));
    groups.sort((a, b) => b.count - a.count);

    this.commandGroups = groups;
    this.maxCommandCount = groups.reduce((m, g) => Math.max(m, g.count), 0);
    this.totalCommandRuns = groups.reduce((sum, g) => sum + g.count, 0);
  }

  openLiveReplay(sessionId: number): void {
    const url = this.router.serializeUrl(this.router.createUrlTree(['/live', sessionId]));
    window.open(url, '_blank');
  }
}
