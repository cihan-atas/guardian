import {
  Component, ElementRef, ViewChild, Input, Output, EventEmitter,
  OnDestroy, AfterViewInit, HostBinding, NgZone
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { SearchAddon } from 'xterm-addon-search';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import {
  faExpand, faCompress, faPlus, faMinus, faSearch,
  faChevronUp, faChevronDown, faDownload, faArrowDown
} from '@fortawesome/free-solid-svg-icons';

const MIN_FONT_SIZE = 10;
const MAX_FONT_SIZE = 24;
const DEFAULT_FONT_SIZE = 14;

/** Base64 string'i ham baytlara çevirir (UTF-8 çok baytlı karakterler için). */
export function decodeBase64ToBytes(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/**
 * xterm.js kurulumunu tek yerde toplayan paylaşılan görüntüleyici.
 *
 * Boyutlandırma: terminal, saran konteyneri dolduracak şekilde fit edilir
 * (fitAddon). ÖNEMLİ: xterm'in canvas renderer'ı bir CSS `transform: scale()`
 * içine konulursa, kaydırma/yeniden çizimden sonra canvas siyah kalıyor
 * (bilinen uyumsuzluk). Bu yüzden ölçekleme YAPMIYORUZ; terminal doğrudan
 * konteyner boyutunda render edilir, hem kararma olmaz hem ekran dolu görünür.
 */
@Component({
  selector: 'app-terminal-viewer',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent],
  templateUrl: './terminal-viewer.component.html',
  // xterm.css component'te DEĞİL, global styles.scss'te yükleniyor (runtime'da
  // oluşan xterm DOM'una scoped stil uygulanmadığı için).
  styleUrls: ['./terminal-viewer.component.scss']
})
export class TerminalViewerComponent implements AfterViewInit, OnDestroy {
  @ViewChild('terminalEl', { static: true }) private terminalEl!: ElementRef<HTMLElement>;
  @ViewChild('containerEl', { static: true }) private containerEl!: ElementRef<HTMLElement>;
  @ViewChild('rootEl', { static: true }) private rootEl!: ElementRef<HTMLElement>;

  /** Kaydedilen PTY boyutu — bilgi amaçlı; terminal konteynere fit edilir. */
  @Input() cols = 0;
  @Input() rows = 0;
  @Input() filename = 'session.log';
  /** Canlı modda, en alttayken gelen veriyle otomatik takip aktiftir. */
  @Input() followMode = false;

  @Output() readonly ready = new EventEmitter<void>();

  @HostBinding('class.is-fullscreen') isFullscreen = false;

  isSearchOpen = false;
  searchTerm = '';
  searchResultText = '';
  fontSize = DEFAULT_FONT_SIZE;
  /** Kullanıcı elle yukarı kaydırdı; canlı akışın gerisinde. "CANLIYA DÖN"
   * butonu bununla gösterilir ve otomatik takip bununla duraklatılır. */
  isBehindLive = false;
  userScrolledUp = false;

  faExpand = faExpand;
  faCompress = faCompress;
  faPlus = faPlus;
  faMinus = faMinus;
  faSearch = faSearch;
  faChevronUp = faChevronUp;
  faChevronDown = faChevronDown;
  faDownload = faDownload;
  faArrowDown = faArrowDown;

  private term!: Terminal;
  private fitAddon = new FitAddon();
  private searchAddon = new SearchAddon();
  private rawBuffer = '';
  private resizeObserver?: ResizeObserver;
  private fullscreenHandler = () => {
    this.isFullscreen = !!document.fullscreenElement;
    setTimeout(() => this.applyLayout(), 80);
  };

  constructor(private ngZone: NgZone) {}

  ngAfterViewInit(): void {
    this.term = new Terminal({
      cursorBlink: true,
      convertEol: true,
      fontSize: this.fontSize,
      fontFamily: '"IBM Plex Mono", ui-monospace, monospace',
      theme: { background: '#0d1117' },
      scrollback: 10000,
    });
    this.term.loadAddon(this.fitAddon);
    this.term.loadAddon(this.searchAddon);
    this.term.open(this.terminalEl.nativeElement);

    // Layout'un oturması için bir frame bekle (0-boyut ölçüm hatasına karşı).
    requestAnimationFrame(() => this.applyLayout());
    // KRİTİK: fitAddon hücre boyutunu ölçerek satır/sütun hesaplar. Web fontu
    // (IBM Plex Mono) henüz yüklenmemişse hücre yanlış ölçülüp binlerce satır
    // hesaplanıyor ve ekran kararıyordu. Font hazır olunca yeniden fit et.
    if ((document as any).fonts?.ready) {
      (document as any).fonts.ready.then(() => this.applyLayout());
    }

    this.resizeObserver = new ResizeObserver(() => this.applyLayout());
    this.resizeObserver.observe(this.containerEl.nativeElement);

    // Kullanıcının ELLE yukarı kaydırmasını yakala (fare tekerleği). Otomatik
    // takip yalnızca kullanıcı yukarı kaydırdığında duraklar; yazımdan kaynaklı
    // programatik scroll bunu tetiklemez.
    const viewportEl = this.terminalEl.nativeElement.querySelector('.xterm-viewport');
    viewportEl?.addEventListener('wheel', (e) => {
      if ((e as WheelEvent).deltaY < 0) this.setBehind(true);
    }, { passive: true });
    this.term.onScroll(() => {
      const buf = this.term.buffer.active;
      // En alta gelindiyse takip tekrar aktif.
      if (buf.viewportY >= buf.baseY) this.setBehind(false);
    });

    this.searchAddon.onDidChangeResults((res) => {
      if (!res || res.resultCount === 0) {
        this.searchResultText = this.searchTerm ? '0/0' : '';
      } else {
        this.searchResultText = `${res.resultIndex + 1}/${res.resultCount}`;
      }
    });

    document.addEventListener('fullscreenchange', this.fullscreenHandler);
    this.ready.emit();
  }

  ngOnDestroy(): void {
    document.removeEventListener('fullscreenchange', this.fullscreenHandler);
    this.resizeObserver?.disconnect();
    this.term?.dispose();
  }

  /** Terminali saran konteyneri dolduracak şekilde boyutlandırır. Konteyner
   * henüz layout almamışsa (0 boyut) bozuk hücre boyutuna kilitlememek için atlar. */
  private applyLayout(): void {
    const proposed = this.fitAddon.proposeDimensions();
    if (!proposed || !isFinite(proposed.cols) || !isFinite(proposed.rows) || proposed.cols <= 0 || proposed.rows <= 0) {
      return;
    }
    this.fitAddon.fit();
  }

  fit(): void {
    this.applyLayout();
  }

  private setBehind(behind: boolean): void {
    this.userScrolledUp = behind;
    if (behind !== this.isBehindLive) {
      this.ngZone.run(() => this.isBehindLive = behind);
    }
  }

  jumpToLive(): void {
    this.setBehind(false);
    this.term.scrollToBottom();
  }

  writeBase64(base64: string): void {
    const bytes = decodeBase64ToBytes(base64);
    this.rawBuffer += new TextDecoder('utf-8', { fatal: false }).decode(bytes);
    // Canlı akışta kullanıcı elle yukarı kaydırmadıysa en altta kal. followMode
    // + kullanıcı yukarıda değilse her yazımdan sonra dibe in.
    const shouldFollow = this.followMode && !this.userScrolledUp;
    this.term.write(bytes, () => {
      if (shouldFollow) this.term.scrollToBottom();
    });
  }

  /** Çok sayıda base64 parçayı tek seferde yazar (hızlı seek için). */
  writeBase64Batch(base64Chunks: string[]): void {
    let totalLen = 0;
    const parts = base64Chunks.map(chunk => {
      const bytes = decodeBase64ToBytes(chunk);
      totalLen += bytes.length;
      return bytes;
    });
    const merged = new Uint8Array(totalLen);
    let offset = 0;
    for (const p of parts) {
      merged.set(p, offset);
      offset += p.length;
    }
    this.rawBuffer += new TextDecoder('utf-8', { fatal: false }).decode(merged);
    this.term.write(merged, () => this.term.scrollToBottom());
  }

  write(data: string): void {
    this.rawBuffer += data;
    this.term.write(data);
  }

  reset(): void {
    this.rawBuffer = '';
    this.term.reset();
  }

  getRawText(): string {
    return this.rawBuffer;
  }

  downloadLog(filename = this.filename): void {
    const blob = new Blob([this.rawBuffer], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }

  toggleFullscreen(): void {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      this.rootEl.nativeElement.requestFullscreen();
    }
  }

  increaseFontSize(): void {
    this.fontSize = Math.min(MAX_FONT_SIZE, this.fontSize + 2);
    this.term.options.fontSize = this.fontSize;
    this.fit();
  }

  decreaseFontSize(): void {
    this.fontSize = Math.max(MIN_FONT_SIZE, this.fontSize - 2);
    this.term.options.fontSize = this.fontSize;
    this.fit();
  }

  toggleSearch(): void {
    this.isSearchOpen = !this.isSearchOpen;
    if (!this.isSearchOpen) {
      this.searchTerm = '';
      this.searchResultText = '';
      this.searchAddon.clearDecorations();
    }
  }

  onSearchInput(): void {
    if (!this.searchTerm) {
      this.searchResultText = '';
      this.searchAddon.clearDecorations();
      return;
    }
    this.searchAddon.findNext(this.searchTerm, { incremental: true });
  }

  searchNext(): void {
    if (this.searchTerm) this.searchAddon.findNext(this.searchTerm);
  }

  searchPrevious(): void {
    if (this.searchTerm) this.searchAddon.findPrevious(this.searchTerm);
  }
}
