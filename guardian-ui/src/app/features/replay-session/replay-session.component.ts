import { Component, OnInit, OnDestroy, ElementRef, ViewChild, NgZone, AfterViewInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { ApiClientService, SessionEvent } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { lastValueFrom } from 'rxjs';

 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlay, faPause, faRedo } from '@fortawesome/free-solid-svg-icons';

interface ReplayEvent extends SessionEvent {
  timestamp: number;
}

@Component({
  selector: 'app-replay-session',
  standalone: true,
   imports: [CommonModule, FormsModule, FaIconComponent],
  styleUrls: [
    './replay-session.component.scss',
    '../../../../node_modules/xterm/css/xterm.css'
  ],
  templateUrl: './replay-session.component.html',
})
export class ReplaySessionComponent implements OnInit, OnDestroy, AfterViewInit {
  @ViewChild('terminal', { static: true }) private terminalEl!: ElementRef;
  private term: Terminal;
  private fitAddon: FitAddon;
  private routeSub: Subscription | undefined;
  // Oturumun kaydedildiği PTY boyutu (backend'den gelir). Tekrar oynatma
  // terminali, ANSI imleç konumlandırma/scroll-region kodlarının doğru
  // yorumlanabilmesi için bu boyutla oluşturulmalı; konteynere göre dinamik
  // "fit" edilirse (kayıt boyutundan farklıysa) ekran birden fazla ekranı
  // doldurunca imleç sıçraması ve bozuk kaydırma oluşur.
  private recordedCols = 0;
  private recordedRows = 0;

   faPlay = faPlay;
  faPause = faPause;
  faRedo = faRedo;
  

  public sessionId: string | null = null;
  public player = {
    events: [] as ReplayEvent[],
    isPlaying: false,
    isLoaded: false,
    timeoutId: null as any,
    currentEventIndex: 0,
    playbackSpeed: 1,
    virtualStartTime: 0,
    sessionDuration: 0,
    progress: 0,
    currentTime: '00:00',
    totalTime: '00:00'
  };

  constructor(
    private route: ActivatedRoute,
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private ngZone: NgZone
  ) {
    this.term = new Terminal({
      cursorBlink: false,
      convertEol: true,
      theme: { background: '#000000' },
      scrollback: 10000,
    });
    this.fitAddon = new FitAddon();
    this.term.loadAddon(this.fitAddon);
  }

   
  ngOnInit(): void {
    this.term.open(this.terminalEl.nativeElement);
    this.routeSub = this.route.params.subscribe(params => {
      this.sessionId = params['id'];
      if (this.sessionId) {
        this.loadSession();
      }
    });
  }

  ngAfterViewInit(): void {
    this.fitAddon.fit();
    window.addEventListener('resize', () => {
      // Kayıtlı bir PTY boyutu biliniyorsa terminal boyutunu sabit tutuyoruz;
      // aksi halde (eski kayıtlarda meta veri yoksa) konteynere göre fit ediyoruz.
      if (!this.recordedCols || !this.recordedRows) {
        this.fitAddon.fit();
      }
    });
  }

  ngOnDestroy(): void {
    clearTimeout(this.player.timeoutId);
    this.term.dispose();
    this.routeSub?.unsubscribe();
  }

  async loadSession(): Promise<void> {
    if (!this.sessionId) return;
    this.resetPlayer();
    this.term.write(`Oturum ID ${this.sessionId} yükleniyor...`);

    try {
      const replay = await lastValueFrom(this.apiClient.getSessionReplay(Number(this.sessionId)));
      const rawEvents = replay?.events;
      if (!rawEvents || rawEvents.length === 0) {
        this.term.write('\r\nBu oturum için kayıtlı terminal çıktısı bulunamadı.');
        this.toastr.info('Bu oturum için kayıtlı terminal çıktısı bulunamadı.');
        return;
      }

      this.recordedCols = replay.cols;
      this.recordedRows = replay.rows;
      if (this.recordedCols > 0 && this.recordedRows > 0) {
        // Terminali, kaydın yapıldığı PTY ile birebir aynı boyuta getir.
        // Konteyner daha küçükse tarayıcı kaydırma çubuklarıyla gösterilir;
        // bu, boyut uyuşmazlığından kaynaklanan imleç/kaydırma bozulmalarını önler.
        this.term.resize(this.recordedCols, this.recordedRows);
      } else {
        this.fitAddon.fit();
      }

      this.player.events = rawEvents
        .filter(e => e.event_type === 'output')
        .map(e => ({ ...e, timestamp: new Date(e.event_time).getTime() }));

      if (this.player.events.length === 0) {
        this.term.write('\r\nBu oturum için kayıtlı terminal çıktısı bulunamadı.');
        return;
      }

      this.player.isLoaded = true;
      this.player.virtualStartTime = this.player.events[0].timestamp;
      const endTime = this.player.events[this.player.events.length - 1].timestamp;
      this.player.sessionDuration = endTime - this.player.virtualStartTime;
      this.player.totalTime = this.formatTime(this.player.sessionDuration / 1000);

      this.term.reset();
      this.play();

    } catch (error: any) {
      const errorMessage = error.error || error.message || 'Bilinmeyen bir hata oluştu.';
      this.term.write(`\r\n\x1b[31mHATA: ${errorMessage}\x1b[0m`);
      this.toastr.error(errorMessage, 'Yükleme Hatası');
    }
  }

  playNextEvent(): void {
    if (!this.player.isPlaying || this.player.currentEventIndex >= this.player.events.length) {
      this.pause();
      return;
    }

    const event = this.player.events[this.player.currentEventIndex];
    try {
      const decodedData = atob(event.data);
      this.term.write(decodedData);
    } catch (e) { console.warn("Base64 decode hatası:", e); }

    this.ngZone.run(() => {
      this.updateProgress();
    });
    
    this.player.currentEventIndex++;

    if (this.player.currentEventIndex < this.player.events.length) {
      const nextEvent = this.player.events[this.player.currentEventIndex];
      const delay = (nextEvent.timestamp - event.timestamp) / this.player.playbackSpeed;
      this.player.timeoutId = setTimeout(() => this.playNextEvent(), delay);
    } else {
      this.pause();
    }
  }

  play(): void {
    if (this.player.isPlaying || !this.player.isLoaded) return;
    this.ngZone.run(() => {
      this.player.isPlaying = true;
      this.playNextEvent();
    });
  }

  pause(): void {
    if (!this.player.isPlaying) return;
    this.ngZone.run(() => {
      this.player.isPlaying = false;
      clearTimeout(this.player.timeoutId);
    });
  }

  restart(): void {
    this.pause();
    this.term.reset();
    this.player.currentEventIndex = 0;
    this.updateProgress();
    this.play();
  }

  onProgressChange(event: Event): void {
    this.pause();
    const input = event.target as HTMLInputElement;
    const percentage = parseFloat(input.value);
    const targetTime = this.player.virtualStartTime + (this.player.sessionDuration * (percentage / 100));

    let newIndex = this.player.events.findIndex(e => e.timestamp >= targetTime);
    if (newIndex === -1) newIndex = this.player.events.length - 1;

    this.player.currentEventIndex = newIndex;

    this.term.reset();
    for (let i = 0; i < this.player.currentEventIndex; i++) {
      try { this.term.write(atob(this.player.events[i].data)); } catch { }
    }
    this.updateProgress();
  }

  resetPlayer(): void {
    this.ngZone.run(() => {
      this.pause();
      this.term.reset();
      this.player = {
        ...this.player,
        events: [], isPlaying: false, isLoaded: false,
        timeoutId: null, currentEventIndex: 0, progress: 0,
        currentTime: '00:00', totalTime: '00:00'
      };
    });
  }

  updateProgress(): void {
    if (!this.player.isLoaded || this.player.events.length === 0) return;
    const currentEvent = this.player.events[this.player.currentEventIndex] || this.player.events[this.player.events.length - 1];
    const eventTimeOffset = currentEvent.timestamp - this.player.virtualStartTime;
    this.player.progress = (eventTimeOffset / this.player.sessionDuration) * 100;
    this.player.currentTime = this.formatTime(eventTimeOffset / 1000);
  }

  formatTime(seconds: number): string {
    const min = Math.floor(seconds / 60);
    const sec = Math.floor(seconds % 60);
    return `${String(min).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
  }
}