import { Component, OnInit, OnDestroy, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { ToastrService } from 'ngx-toastr';
import { AuthService } from '../../core/services/auth.service';
import { ApiClientService, SessionDetails, KeyBanStatus } from '../../core/services/api-client.service';
import { Subscription, lastValueFrom } from 'rxjs';
import { environment } from '../../../environments/environment';
import { TerminalViewerComponent } from '../../shared/terminal-viewer/terminal-viewer.component';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPowerOff, faShieldHalved, faBan, faPause, faTriangleExclamation, faXmark } from '@fortawesome/free-solid-svg-icons';

/** Canlı oturumda sunucudan gelen riskli komut uyarısı. */
interface LiveAlert {
  severity: string;
  rule: string;
  command: string;
  action: string;
  timestamp: string;
}

@Component({
  selector: 'app-live-session',
  standalone: true,
  imports: [CommonModule, RouterLink, TerminalViewerComponent, FaIconComponent],
  templateUrl: './live-session.component.html',
  styleUrls: ['./live-session.component.scss']
})
export class LiveSessionComponent implements OnInit, OnDestroy {
  @ViewChild(TerminalViewerComponent) private viewer!: TerminalViewerComponent;

  faTerminate = faPowerOff;
  faShield = faShieldHalved;
  faBan = faBan;
  faPause = faPause;
  faWarn = faTriangleExclamation;
  faClose = faXmark;

  /** Bu oturumda tespit edilen riskli komutlar (en yeni en üstte). */
  public alerts: LiveAlert[] = [];

  private routeSub: Subscription | undefined;
  private durationTimer: ReturnType<typeof setInterval> | undefined;
  private connectedAt: number | null = null;
  private viewerReady = false;
  private pendingConnect = false;

  ws: WebSocket | null = null;

  public sessionId: string | null = null;
  public status = { message: 'Bağlanıyor...', color: 'orange', isLive: false };
  public info: SessionDetails['session_info'] | null = null;
  public keyBanStatus: KeyBanStatus | null = null;
  public cols = 0;
  public rows = 0;
  public durationLabel = '00:00';

  constructor(
    private route: ActivatedRoute,
    private authService: AuthService,
    private apiClient: ApiClientService,
    private toastr: ToastrService
  ) {}

  ngOnInit(): void {
    this.routeSub = this.route.params.subscribe(params => {
      this.sessionId = params['id'];
      if (this.sessionId) {
        this.loadDetails();
        this.prepareAndConnect();
      }
    });
  }

  ngOnDestroy(): void {
    this.ws?.close();
    this.routeSub?.unsubscribe();
    if (this.durationTimer) clearInterval(this.durationTimer);
  }

  onViewerReady(): void {
    this.viewerReady = true;
    if (this.pendingConnect) {
      this.pendingConnect = false;
      this.connect();
    }
  }

  private loadDetails(): void {
    if (!this.sessionId) return;
    this.apiClient.getSessionDetails(Number(this.sessionId)).subscribe({
      next: (data) => {
        this.info = data.session_info;
        const keyId = data.session_info.public_key_id;
        if (keyId) {
          this.apiClient.getKeyBanStatus(keyId).subscribe({
            next: (status) => this.keyBanStatus = status,
            error: () => this.keyBanStatus = null
          });
        }
      },
      error: () => { /* bilgi paneli olmadan da izleme devam edebilir */ }
    });
  }

  private async prepareAndConnect(): Promise<void> {
    if (!this.sessionId) return;
    // Terminali, kaydın yapıldığı PTY ile aynı boyuta getiriyoruz; aksi halde
    // ANSI imleç konumlandırma/scroll-region kodları yanlış yorumlanır.
    try {
      const size = await lastValueFrom(this.apiClient.getSessionTerminalSize(Number(this.sessionId)));
      if (size?.cols > 0 && size?.rows > 0) {
        this.cols = size.cols;
        this.rows = size.rows;
      }
    } catch {
      // boyut alınamazsa TerminalViewerComponent kendi konteynerine fit eder.
    }

    if (this.viewerReady) {
      this.connect();
    } else {
      this.pendingConnect = true;
    }
  }

  connect(): void {
    if (!this.sessionId) return;
    if (this.ws) { this.ws.close(); }

    this.viewer.reset();
    this.status = { message: `Oturum ${this.sessionId}'e bağlanılıyor...`, color: 'orange', isLive: false };

    const token = this.authService.getToken();
    if (!token) {
      this.toastr.error('Kimlik doğrulama token\'ı bulunamadı!', 'Hata');
      this.status = { message: 'Yetkilendirme hatası!', color: 'red', isLive: false };
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
    const host = window.location.host;
    const apiPath = environment.apiUrl;
    const wsURL = `${protocol}://${host}${apiPath}/ws/sessions/${this.sessionId}?role=viewer&token=${token}`;

    this.ws = new WebSocket(wsURL);

    this.ws.onopen = () => {
      this.status = { message: `Oturum ${this.sessionId} izleniyor (CANLI)`, color: 'green', isLive: true };
      this.connectedAt = Date.now();
      this.startDurationTimer();
    };

    this.ws.onmessage = (event) => {
      const messageObject = JSON.parse(event.data);
      // Sunucu hem "input" (kullanıcının bastığı tuşlar) hem "output" (shell'in
      // ekrana bastığı) olaylarını yayınlar. Gerçek bir terminal görüntüleyici
      // gibi SADECE output'u basıyoruz; shell zaten yazılanı kendisi output
      // olarak yankılıyor. İkisini birden yazmak "ll" -> "llll" gibi çift
      // karakter oluşturuyordu.
      if (messageObject && messageObject.type === 'output' && messageObject.data) {
        try {
          this.viewer.writeBase64(messageObject.data);
        } catch (e) {
          console.error('Base64 decode hatası:', e, messageObject.data);
        }
      } else if (messageObject && messageObject.type === 'alert') {
        this.handleAlert(messageObject as LiveAlert);
      }
    };

    this.ws.onclose = () => {
      this.status = { message: 'Bağlantı kesildi', color: 'red', isLive: false };
      this.ws = null;
      if (this.durationTimer) clearInterval(this.durationTimer);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket hatası:', error);
      this.status = { message: 'Bağlantı hatası!', color: 'red', isLive: false };
    };
  }

  private handleAlert(alert: LiveAlert): void {
    this.alerts.unshift(alert);
    if (this.alerts.length > 20) this.alerts.pop();

    const label = alert.severity === 'critical' ? 'KRİTİK' : 'RİSKLİ';
    let msg = `${alert.rule}: ${alert.command}`;
    if (alert.action && alert.action !== 'none') {
      msg += ` — otomatik aksiyon: ${alert.action}`;
    }
    if (alert.severity === 'critical') {
      this.toastr.error(msg, `⚠ ${label} KOMUT`, { timeOut: 12000, closeButton: true });
    } else {
      this.toastr.warning(msg, `⚠ ${label} KOMUT`, { timeOut: 9000, closeButton: true });
    }
  }

  dismissAlert(index: number): void {
    this.alerts.splice(index, 1);
  }

  clearAlerts(): void {
    this.alerts = [];
  }

  private startDurationTimer(): void {
    if (this.durationTimer) clearInterval(this.durationTimer);
    this.durationTimer = setInterval(() => {
      if (!this.connectedAt) return;
      const totalSeconds = Math.floor((Date.now() - this.connectedAt) / 1000);
      const min = Math.floor(totalSeconds / 60).toString().padStart(2, '0');
      const sec = (totalSeconds % 60).toString().padStart(2, '0');
      this.durationLabel = `${min}:${sec}`;
    }, 1000);
  }

  disconnect(): void {
    this.ws?.close();
  }

  killSession(): void {
    if (!this.sessionId) return;
    if (!confirm(`EMİN MİSİNİZ?\n\nOturum ID ${this.sessionId} kalıcı olarak sonlandırılacak! Bu işlem geri alınamaz.`)) return;

    this.status = { ...this.status, message: `Oturum ${this.sessionId} için sonlandırma komutu gönderiliyor...`, color: 'orange' };
    this.apiClient.terminateSession(Number(this.sessionId)).subscribe({
      next: () => {
        this.toastr.success('Sonlandırma komutu gönderildi.');
        this.status = { ...this.status, message: 'Sonlandırma komutu gönderildi', color: 'green' };
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Hata');
        this.status = { ...this.status, message: `Hata: ${errorMessage}`, color: 'red' };
      }
    });
  }

  banKey(): void {
    const keyId = this.info?.public_key_id;
    const keyName = this.info?.public_key_name || `#${keyId}`;
    if (!keyId) return;

    const durationStr = prompt(`"${keyName}" anahtarını kaç dakika yasaklamak istiyorsunuz?`, '60');
    if (!durationStr) return;
    const duration = parseInt(durationStr, 10);
    if (isNaN(duration) || duration <= 0) {
      this.toastr.error('Geçerli bir dakika değeri girin.', 'Hata');
      return;
    }
    const reason = prompt('Yasaklama sebebi (opsiyonel):', '') || undefined;

    this.apiClient.banKey(keyId, duration, reason).subscribe({
      next: (ban) => {
        this.toastr.success(`Anahtar ${duration} dakika süreyle yasaklandı.`, 'Yasaklandı');
        this.keyBanStatus = { banned: true, ban };
      },
      error: (err) => this.toastr.error(err.error || 'Anahtar yasaklanamadı.', 'Hata')
    });
  }

  unbanKey(): void {
    const keyId = this.info?.public_key_id;
    if (!keyId) return;
    if (!confirm('Bu anahtar üzerindeki yasağı şimdi kaldırmak istediğinize emin misiniz?')) return;

    this.apiClient.unbanKey(keyId).subscribe({
      next: () => {
        this.toastr.success('Yasak kaldırıldı.');
        this.keyBanStatus = { banned: false };
      },
      error: () => this.toastr.error('Yasak kaldırılamadı.', 'Hata')
    });
  }
}
