import { Injectable, signal } from '@angular/core';

export interface ConfirmOptions {
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  /** true → onay butonu kırmızı (silme/sonlandırma gibi yıkıcı işlemler). */
  danger?: boolean;
}

interface ActiveConfirm {
  opts: ConfirmOptions;
  resolve: (result: boolean) => void;
}

/**
 * window.confirm yerine stilli onay modalı. Component'ler
 * `await confirmDialog.confirm({...})` çağırır; ConfirmDialogComponent
 * app kökünde bir kez durur ve buradaki state'i render eder.
 */
@Injectable({ providedIn: 'root' })
export class ConfirmDialogService {
  readonly active = signal<ActiveConfirm | null>(null);

  confirm(opts: ConfirmOptions): Promise<boolean> {
    return new Promise<boolean>((resolve) => {
      this.active.set({ opts, resolve });
    });
  }

  close(result: boolean): void {
    const current = this.active();
    this.active.set(null);
    current?.resolve(result);
  }
}
