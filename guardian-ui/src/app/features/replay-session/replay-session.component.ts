import { Component, OnInit, OnDestroy, ViewChild, NgZone, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { ApiClientService, SessionEvent, ParsedCommand, SessionDetails } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { FormsModule } from '@angular/forms';
import { Subscription, lastValueFrom } from 'rxjs';
import { TerminalViewerComponent } from '../../shared/terminal-viewer/terminal-viewer.component';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import {
  faListUl, faTerminal, faTriangleExclamation,
  faChevronLeft, faChevronRight, faRotateLeft, faShieldHalved, faClock
} from '@fortawesome/free-solid-svg-icons';

interface ReplayEvent {
  data: string;
  timestamp: number;
}

// Görüntüleme katmanında admin'in gözüne çarpması gereken komutlar. Güvenlik
// kontrolü değil (bypass edilebilir); hızlı tarama için görsel işaret.
const RISKY_PATTERNS: RegExp[] = [
  /\bsudo\b/, /\brm\s+-rf\b|\brm\s+(-\w*\s+)*-\w*r\w*f/, /\bchmod\s+777\b/,
  /curl[^|]*\|\s*(ba)?sh/, /wget[^|]*\|\s*(ba)?sh/, /\bnc\s+-/, /\bncat\b/,
  /\bdd\s+if=/, /\biptables\s+-F\b/, /history\s+-c/, /\bbase64\s+-d/,
  /\/etc\/(passwd|shadow)/, /\bsystemctl\s+(stop|disable)/, /\buseradd\b|\busermod\b/,
];

export function isRiskyCommand(cmd: string): boolean {
  return RISKY_PATTERNS.some(p => p.test(cmd));
}

@Component({
  selector: 'app-replay-session',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent, TerminalViewerComponent],
  styleUrls: ['./replay-session.component.scss'],
  templateUrl: './replay-session.component.html',
})
export class ReplaySessionComponent implements OnInit, OnDestroy {
  @ViewChild(TerminalViewerComponent) private viewer!: TerminalViewerComponent;

  private routeSub: Subscription | undefined;
  private viewerReady = false;
  private events: ReplayEvent[] = [];
  private pendingRender = false;

  public cols = 0;
  public rows = 0;

  faListUl = faListUl;
  faTerminal = faTerminal;
  faRisky = faTriangleExclamation;
  faPrev = faChevronLeft;
  faNext = faChevronRight;
  faRestart = faRotateLeft;
  faShield = faShieldHalved;
  faClock = faClock;

  public sessionId: string | null = null;
  public info: SessionDetails['session_info'] | null = null;
  public commands: ParsedCommand[] = [];
  public activeIndex = -1;
  public isCommandsPanelOpen = true;
  public isLoaded = false;
  public riskyCount = 0;

  constructor(
    private route: ActivatedRoute,
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private ngZone: NgZone
  ) {}

  ngOnInit(): void {
    this.routeSub = this.route.params.subscribe(params => {
      this.sessionId = params['id'];
      if (this.sessionId) {
        this.loadCommands();
        this.loadEvents();
      }
    });
  }

  ngOnDestroy(): void {
    this.routeSub?.unsubscribe();
  }

  @HostListener('window:keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    const target = event.target as HTMLElement;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT')) return;
    if (!this.isLoaded) return;

    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        event.preventDefault();
        this.step(1);
        break;
      case 'ArrowLeft':
      case 'ArrowUp':
        event.preventDefault();
        this.step(-1);
        break;
      case 'Home':
        event.preventDefault();
        this.goTo(0);
        break;
      case 'End':
        event.preventDefault();
        this.goTo(this.commands.length - 1);
        break;
    }
  }

  onViewerReady(): void {
    this.viewerReady = true;
    if (this.pendingRender) {
      this.pendingRender = false;
      this.renderUpToActive();
    }
  }

  isRisky(cmd: ParsedCommand): boolean {
    return isRiskyCommand(cmd.command);
  }

  outputPreview(cmd: ParsedCommand): string {
    const out = (cmd.output || '').replace(/\s+/g, ' ').trim();
    return out.length > 90 ? out.slice(0, 90) + '…' : out;
  }

  get activeCommand(): ParsedCommand | null {
    return this.activeIndex >= 0 && this.activeIndex < this.commands.length ? this.commands[this.activeIndex] : null;
  }

  private loadCommands(): void {
    if (!this.sessionId) return;
    this.apiClient.getSessionDetails(Number(this.sessionId)).subscribe({
      next: (data) => {
        this.info = data.session_info;
        this.commands = data.commands ?? [];
        this.riskyCount = this.commands.filter(c => this.isRisky(c)).length;
        this.maybeInitialRender();
      },
      error: () => { this.commands = []; }
    });
  }

  private async loadEvents(): Promise<void> {
    if (!this.sessionId) return;
    try {
      const replay = await lastValueFrom(this.apiClient.getSessionReplay(Number(this.sessionId)));
      const rawEvents = replay?.events;
      if (!rawEvents || rawEvents.length === 0) {
        this.toastr.info('Bu oturum için kayıtlı terminal çıktısı bulunamadı.');
        return;
      }
      this.cols = replay.cols;
      this.rows = replay.rows;
      this.events = rawEvents
        .filter(e => e.event_type === 'output')
        .map(e => ({ data: e.data, timestamp: new Date(e.event_time).getTime() }));
      this.isLoaded = this.events.length > 0;
      this.maybeInitialRender();
    } catch (error: any) {
      this.toastr.error(error.error || error.message || 'Yükleme hatası', 'Hata');
    }
  }

  /** Hem komutlar hem olaylar geldiğinde ilk kareyi (son komut) göster. */
  private maybeInitialRender(): void {
    if (!this.isLoaded) return;
    if (this.activeIndex === -1) {
      // Baştan başla: admin → ile ilerleyerek oturumu adım adım inceler.
      this.activeIndex = 0;
    }
    this.renderUpToActive();
  }

  step(direction: 1 | -1): void {
    if (this.commands.length === 0) return;
    this.goTo(this.activeIndex + direction);
  }

  goTo(index: number): void {
    if (this.commands.length === 0) return;
    const clamped = Math.max(0, Math.min(this.commands.length - 1, index));
    this.activeIndex = clamped;
    this.renderUpToActive();
    this.scrollActiveIntoView();
  }

  /**
   * Terminali sıfırlayıp, seçili komutun ÇIKTISI dahil o ana kadarki tüm
   * output olaylarını tek seferde basar. Böylece admin komutu + sonucunu
   * birlikte, anında görür (kare kare inceleme).
   */
  private renderUpToActive(): void {
    if (!this.viewerReady) { this.pendingRender = true; return; }
    if (!this.isLoaded || this.commands.length === 0) return;

    // Bu komutun çıktısının bittiği an ≈ bir sonraki komutun yazıldığı an.
    const cutoff = this.activeIndex + 1 < this.commands.length
      ? new Date(this.commands[this.activeIndex + 1].timestamp).getTime()
      : Infinity;

    const chunks: string[] = [];
    for (const ev of this.events) {
      if (ev.timestamp <= cutoff) chunks.push(ev.data);
      else break;
    }

    this.viewer.reset();
    if (chunks.length > 0) {
      try { this.viewer.writeBase64Batch(chunks); } catch { /* ignore */ }
    }
  }

  private scrollActiveIntoView(): void {
    setTimeout(() => {
      const el = document.querySelector('.cmd-item.active') as HTMLElement | null;
      el?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }, 0);
  }

  selectCommand(index: number): void {
    this.goTo(index);
  }

  reload(): void {
    this.activeIndex = -1;
    this.events = [];
    this.isLoaded = false;
    this.loadCommands();
    this.loadEvents();
  }
}
