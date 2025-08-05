import { Component, OnInit, OnDestroy, ElementRef, ViewChild, NgZone, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { ToastrService } from 'ngx-toastr';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { AuthService } from '../../core/services/auth.service';
import { ApiClientService } from '../../core/services/api-client.service';
import { Subscription } from 'rxjs';
import { environment } from '../../../environments/environment'; 

@Component({
  selector: 'app-live-session',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './live-session.component.html',
  styleUrls: [
    './live-session.component.scss',
    '../../../../node_modules/xterm/css/xterm.css'
  ]
})
export class LiveSessionComponent implements OnInit, OnDestroy, AfterViewInit {
  @ViewChild('terminal', { static: true }) private terminalEl!: ElementRef;
  private term: Terminal;
  private fitAddon: FitAddon;
  private routeSub: Subscription | undefined;

  ws: WebSocket | null = null;
 
  public sessionId: string | null = null;
  public status = { message: 'Bağlanıyor...', color: 'orange', isLive: false };

  constructor(
    private route: ActivatedRoute,
    private authService: AuthService,
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private ngZone: NgZone
  ) {
    this.term = new Terminal({
      cursorBlink: true,
      convertEol: true,
      theme: { background: '#000000' }
    });
    this.fitAddon = new FitAddon();
    this.term.loadAddon(this.fitAddon);
  }

  ngOnInit(): void {
    this.term.open(this.terminalEl.nativeElement);
    this.routeSub = this.route.params.subscribe(params => {
      this.sessionId = params['id'];
      if (this.sessionId) {
        this.connect();
      }
    });
  }
  
  ngAfterViewInit(): void {
    this.fitAddon.fit();
    window.addEventListener('resize', () => this.fitAddon.fit());
  }

  ngOnDestroy(): void {
    if (this.ws) {
      this.ws.close();
    }
    this.term.dispose();
    this.routeSub?.unsubscribe();
  }
  
  updateStatus(message: string, color: 'green' | 'red' | 'orange' | 'black', isLive = false): void {
    this.ngZone.run(() => {
      this.status = { message, color, isLive };
    });
  }

  connect(): void {
    if (!this.sessionId) return;
    if (this.ws) { this.ws.close(); }

    this.term.reset();
    this.updateStatus(`Oturum ${this.sessionId}'e bağlanılıyor...`, 'orange');

    const token = this.authService.getToken();
    if (!token) {
      this.toastr.error('Kimlik doğrulama token\'ı bulunamadı!', 'Hata');
      this.updateStatus('Yetkilendirme hatası!', 'red');
      return;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';

    const host = window.location.host;

    const apiPath = environment.apiUrl;

    const wsURL = `${protocol}://${host}${apiPath}/ws/sessions/${this.sessionId}?role=viewer&token=${token}`;

    
    this.ws = new WebSocket(wsURL);

    this.ws.onopen = () => {
      this.updateStatus(`Oturum ${this.sessionId} izleniyor (CANLI)`, 'green', true);
    };

    this.ws.onmessage = (event) => {
      const messageObject = JSON.parse(event.data);
      if (messageObject && messageObject.data) {
        try {
          const decodedData = atob(messageObject.data);
          this.term.write(decodedData);
        } catch (e) {
          console.error("Base64 decode hatası:", e, messageObject.data);
        }
      }
    };

    this.ws.onclose = () => {
      this.updateStatus('Bağlantı kesildi', 'red');
      this.ws = null;
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket hatası:", error);
      this.updateStatus('Bağlantı hatası!', 'red');
    };
  }
  
  disconnect(): void {
    if (this.ws) {
        this.ws.close();
    }
  }

  killSession(): void {
    if (!this.sessionId) return;

    const confirmation = confirm(`EMİN MİSİNİZ?\n\nOturum ID ${this.sessionId} kalıcı olarak sonlandırılacak! Bu işlem geri alınamaz.`);
    
    if (confirmation) {
      this.updateStatus(`Oturum ${this.sessionId} için sonlandırma komutu gönderiliyor...`, 'orange');
      this.apiClient.terminateSession(Number(this.sessionId)).subscribe({
        next: () => {
          this.toastr.success('Sonlandırma komutu gönderildi.');
          this.updateStatus('Sonlandırma komutu gönderildi', 'green');
        },
        error: (err) => {
          const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
          this.toastr.error(errorMessage, 'Hata');
          this.updateStatus(`Hata: ${errorMessage}`, 'red');
        }
      });
    }
  }
}
