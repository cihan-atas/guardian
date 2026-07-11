import { Injectable, signal } from '@angular/core';

export interface BanRequest {
  /** Modal başlığında gösterilecek anahtar adı. */
  keyName: string;
}

export interface BanResult {
  durationMinutes: number;
  reason?: string;
}

interface ActiveBan {
  req: BanRequest;
  resolve: (result: BanResult | null) => void;
}

/**
 * Anahtar yasaklama modalı — iki ardışık window.prompt'un yerini alır.
 * `await banDialog.open({keyName})` → süre+gerekçe ya da iptalde null.
 */
@Injectable({ providedIn: 'root' })
export class BanDialogService {
  readonly active = signal<ActiveBan | null>(null);

  open(req: BanRequest): Promise<BanResult | null> {
    return new Promise<BanResult | null>((resolve) => {
      this.active.set({ req, resolve });
    });
  }

  close(result: BanResult | null): void {
    const current = this.active();
    this.active.set(null);
    current?.resolve(result);
  }
}
