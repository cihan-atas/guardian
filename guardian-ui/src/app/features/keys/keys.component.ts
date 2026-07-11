import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators, FormsModule } from '@angular/forms';
import { ToastrService } from 'ngx-toastr';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { ApiClientService, Key, CreateKeyPayload, UpdateKeyPayload } from '../../core/services/api-client.service';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faKey, faBan, faLockOpen, faSync, faMagnifyingGlass, faEye } from '@fortawesome/free-solid-svg-icons';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';
import { CopyButtonComponent } from '../../shared/ui/copy-button/copy-button.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';
import { BanDialogService } from '../../shared/ui/ban-dialog/ban-dialog.service';

@Component({
  selector: 'app-keys',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, FaIconComponent, PaginationComponent, CopyButtonComponent],
  templateUrl: './keys.component.html',
  styleUrl: './keys.component.scss'
})
export class KeysComponent implements OnInit, OnDestroy {
  faPlus = faPlus;
  faKey = faKey;
  faBan = faBan;
  faLockOpen = faLockOpen;
  faSync = faSync;
  faSearch = faMagnifyingGlass;
  faView = faEye;

  keys: Key[] = [];
  isLoading = true;
  error: string | null = null;

  isModalOpen = false;
  keyForm: FormGroup;

  editingKey: Key | null = null;
  isEditMode = false;

  /** "Görüntüle" ile açılan tam public key modalı. */
  viewingKey: Key | null = null;

  currentPage = 1;
  limit = 8;
  totalRecords = 0;

  searchTerm = '';
  private search$ = new Subject<string>();
  private searchSub?: Subscription;

  constructor(
    private apiClient: ApiClientService,
    private fb: FormBuilder,
    private toastr: ToastrService,
    private confirmDialog: ConfirmDialogService,
    private banDialog: BanDialogService
  ) {
    this.keyForm = this.fb.group({
      key_name: ['', Validators.required],
      ssh_public_key: ['', Validators.required]
    });
  }

  ngOnInit(): void {
    this.loadKeys(1);
    this.searchSub = this.search$.pipe(debounceTime(300)).subscribe(() => this.loadKeys(1));
  }

  ngOnDestroy(): void {
    this.searchSub?.unsubscribe();
  }

  onSearchInput(): void {
    this.search$.next(this.searchTerm);
  }

  loadKeys(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    // Yasak durumu artık liste yanıtında geliyor (banned_until alanı) —
    // anahtar başına ayrı ban-status isteği (N+1) kaldırıldı.
    this.apiClient.getKeys(this.currentPage, this.limit, this.searchTerm).subscribe({
      next: (response) => {
        this.keys = response.data ?? [];
        this.totalRecords = response.total_records;
        this.isLoading = false;
      },
      error: () => {
        this.error = 'SSH Anahtar verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  isBanned(key: Key): boolean {
    return !!key.banned_until && new Date(key.banned_until).getTime() > Date.now();
  }

  isExampleKey(key: Key): boolean {
    return key.key_name.startsWith('example-');
  }

  /** Parmak izini kısaltılmış gösterim için hazırlar (SHA256:ilk12…son6). */
  shortFingerprint(key: Key): string {
    const fp = key.fingerprint_sha256 || '';
    if (fp.length <= 24) return fp;
    return `${fp.slice(0, 18)}…${fp.slice(-6)}`;
  }

  async banKey(key: Key): Promise<void> {
    const result = await this.banDialog.open({ keyName: key.key_name });
    if (!result) return;

    this.apiClient.banKey(key.id, result.durationMinutes, result.reason).subscribe({
      next: (ban) => {
        this.toastr.success(`"${key.key_name}" ${result.durationMinutes} dakika süreyle yasaklandı.`, 'Yasaklandı');
        key.banned_until = ban.banned_until;
        key.ban_reason = ban.reason;
      },
      error: (err) => this.toastr.error(err.error || 'Anahtar yasaklanamadı.', 'Hata')
    });
  }

  async unbanKey(key: Key): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Yasağı Kaldır',
      message: `"${key.key_name}" üzerindeki yasak şimdi kaldırılacak. Emin misiniz?`,
      confirmText: 'Yasağı Kaldır',
    });
    if (!ok) return;

    this.apiClient.unbanKey(key.id).subscribe({
      next: () => {
        this.toastr.success('Yasak kaldırıldı.');
        key.banned_until = undefined;
        key.ban_reason = undefined;
      },
      error: () => this.toastr.error('Yasak kaldırılamadı.', 'Hata')
    });
  }

  openViewModal(key: Key): void {
    this.viewingKey = key;
  }

  closeViewModal(): void {
    this.viewingKey = null;
  }

  openModal(keyToEdit: Key | null = null): void {
    if (keyToEdit) {
      this.isEditMode = true;
      this.editingKey = keyToEdit;
      this.keyForm.patchValue({ key_name: keyToEdit.key_name });
      this.keyForm.get('ssh_public_key')?.clearValidators();
      this.keyForm.get('ssh_public_key')?.updateValueAndValidity();
    } else {
      this.isEditMode = false;
      this.editingKey = null;
      this.keyForm.reset();
      this.keyForm.get('ssh_public_key')?.setValidators([Validators.required]);
      this.keyForm.get('ssh_public_key')?.updateValueAndValidity();
    }
    this.isModalOpen = true;
  }

  closeModal(): void {
    this.isModalOpen = false;
  }

  onSubmit(): void {
    if (this.keyForm.invalid) {
      this.toastr.warning('Lütfen tüm zorunlu alanları doldurun.');
      return;
    }

    if (this.isEditMode) {
      this.handleUpdateKey();
    } else {
      this.handleCreateKey();
    }
  }

  handleCreateKey(): void {
    const payload: CreateKeyPayload = this.keyForm.value;
    this.apiClient.createKey(payload).subscribe({
      next: (newKey) => {
        this.toastr.success(`Anahtar '${newKey.key_name}' başarıyla oluşturuldu.`);
        this.closeModal();
        this.loadKeys(1);
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Oluşturma Başarısız');
      }
    });
  }

  handleUpdateKey(): void {
    if (!this.editingKey) return;

    const payload: UpdateKeyPayload = {
      key_name: this.keyForm.get('key_name')?.value
    };

    this.apiClient.updateKey(this.editingKey.id, payload).subscribe({
      next: (updatedKey) => {
        this.toastr.success(`Anahtar adı başarıyla güncellendi.`);
        this.closeModal();
        const index = this.keys.findIndex(k => k.id === updatedKey.id);
        if (index !== -1) {
          // Güncelleme yanıtı ban alanlarını içermez; mevcut ban bilgisini koru.
          this.keys[index] = { ...updatedKey, banned_until: this.keys[index].banned_until, ban_reason: this.keys[index].ban_reason };
        } else {
          this.loadKeys();
        }
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Güncelleme Başarısız');
      }
    });
  }

  async onDeleteKey(key: Key): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Anahtarı Sil',
      message: `'${key.key_name}' kalıcı olarak silinecek. Bu anahtara bağlı kurallar da etkilenir.`,
      confirmText: 'Sil',
      danger: true,
    });
    if (!ok) return;

    this.apiClient.deleteKey(key.id).subscribe({
      next: () => {
        this.toastr.success(`Anahtar '${key.key_name}' başarıyla silindi.`);
        if (this.keys.length === 1 && this.currentPage > 1) {
          this.loadKeys(this.currentPage - 1);
        } else {
          this.loadKeys(this.currentPage);
        }
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Silme Başarısız');
      }
    });
  }
}
