import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule, formatDate } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators, FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { forkJoin, Subject, Subscription, debounceTime } from 'rxjs';
import { ApiClientService, CreateRulePayload, Key, Rule, Server, User, UpdateRulePayload } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faGavel, faSync, faMagnifyingGlass, faClock } from '@fortawesome/free-solid-svg-icons';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';
import { StatusBadgeComponent } from '../../shared/ui/status-badge/status-badge.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';
import { formatRemaining, windowPercent } from '../../shared/time-utils';

@Component({
  selector: 'app-rules',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterLink, FaIconComponent, PaginationComponent, StatusBadgeComponent],
  templateUrl: './rules.component.html',
  styleUrl: './rules.component.scss'
})
export class RulesComponent implements OnInit, OnDestroy {
  faPlus = faPlus;
  faGavel = faGavel;
  faSync = faSync;
  faSearch = faMagnifyingGlass;
  faClock = faClock;

  rules: Rule[] = [];
  isLoading = true;
  error: string | null = null;

  isEditMode = false;
  editingRule: Rule | null = null;

  isModalOpen = false;
  ruleForm: FormGroup;

  servers: Server[] = [];
  users: User[] = [];
  keys: Key[] = [];

  currentPage = 1;
  limit = 8;
  totalRecords = 0;

  searchTerm = '';
  private search$ = new Subject<string>();
  private searchSub?: Subscription;

  /** Kalan süre göstergeleri için 30 sn'de bir tazelenir. */
  now = Date.now();
  private nowTimer?: ReturnType<typeof setInterval>;

  constructor(
    private apiClient: ApiClientService,
    private fb: FormBuilder,
    private toastr: ToastrService,
    private confirmDialog: ConfirmDialogService
  ) {
    this.ruleForm = this.fb.group({
      server_id: [null, Validators.required],
      system_user_id: [null, Validators.required],
      public_key_id: [null, Validators.required],
      valid_from: ['', Validators.required],
      valid_until: ['', Validators.required],
    });
  }

  ngOnInit(): void {
    this.loadInitialData(1);
    this.searchSub = this.search$.pipe(debounceTime(300)).subscribe(() => this.loadInitialData(1));
    this.nowTimer = setInterval(() => (this.now = Date.now()), 30000);
  }

  ngOnDestroy(): void {
    this.searchSub?.unsubscribe();
    if (this.nowTimer) clearInterval(this.nowTimer);
  }

  onSearchInput(): void {
    this.search$.next(this.searchTerm);
  }

  loadInitialData(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    forkJoin({
      rulesResponse: this.apiClient.getRules(this.currentPage, this.limit, this.searchTerm),
      serversResponse: this.apiClient.getServers(1, 1000),
      usersResponse: this.apiClient.getUsers(1, 1000),
      keysResponse: this.apiClient.getKeys(1, 1000),
    }).subscribe({
      next: (data) => {
        this.rules = data.rulesResponse.data ?? [];
        this.totalRecords = data.rulesResponse.total_records;
        this.servers = data.serversResponse.data ?? [];
        this.users = data.usersResponse.data ?? [];
        this.keys = data.keysResponse.data ?? [];
        this.isLoading = false;
      },
      error: () => {
        this.error = 'Sayfa verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error('Sayfa verileri yüklenemedi. API bağlantısını kontrol edin.', 'Yükleme Hatası');
      }
    });
  }

  isExampleRule(rule: Rule): boolean {
    return rule.id === 1 || rule.server_hostname.startsWith('example-');
  }

  // --- Kalan süre göstergesi ---

  remainingMs(rule: Rule): number {
    return new Date(rule.valid_until).getTime() - this.now;
  }

  remainingLabel(rule: Rule): string {
    return formatRemaining(this.remainingMs(rule));
  }

  percentElapsed(rule: Rule): number {
    return windowPercent(rule.valid_from, rule.valid_until, this.now);
  }

  isExpiringSoon(rule: Rule): boolean {
    const ms = this.remainingMs(rule);
    return ms > 0 && ms < 30 * 60000;
  }

  /** Bekleyen kural için başlangıca kalan süre. */
  startsInLabel(rule: Rule): string {
    return formatRemaining(new Date(rule.valid_from).getTime() - this.now);
  }

  async onDeleteRule(rule: Rule): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Kuralı Sil',
      message: `#${rule.id} — ${rule.username}@${rule.server_hostname} (${rule.key_name}) kuralı kalıcı olarak silinecek ve varsa hedef sunucudaki erişim kaldırılacak.`,
      confirmText: 'Sil',
      danger: true,
    });
    if (!ok) return;

    this.apiClient.deleteRule(rule.id).subscribe({
      next: () => {
        this.toastr.success(`Kural #${rule.id} başarıyla silindi.`);
        if (this.rules.length === 1 && this.currentPage > 1) {
          this.loadInitialData(this.currentPage - 1);
        } else {
          this.loadInitialData(this.currentPage);
        }
      },
      error: (err) => {
        const errorMessage = err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Silme Başarısız');
      }
    });
  }

  openModal(ruleToEdit: Rule | null = null): void {
    if (ruleToEdit) {
      this.isEditMode = true;
      this.editingRule = ruleToEdit;

      this.ruleForm.patchValue({
        server_id: ruleToEdit.server_id,
        system_user_id: ruleToEdit.system_user_id,
        public_key_id: ruleToEdit.public_key_id,
        valid_from: formatDate(ruleToEdit.valid_from, 'yyyy-MM-ddTHH:mm', 'en-US'),
        valid_until: formatDate(ruleToEdit.valid_until, 'yyyy-MM-ddTHH:mm', 'en-US'),
      });

      this.ruleForm.get('server_id')?.disable();
      this.ruleForm.get('system_user_id')?.disable();
      this.ruleForm.get('public_key_id')?.disable();
    } else {
      this.isEditMode = false;
      this.editingRule = null;

      const now = new Date();
      const oneHourLater = new Date(now.getTime() + 60 * 60 * 1000);

      this.ruleForm.reset({
        server_id: null,
        system_user_id: null,
        public_key_id: null,
        valid_from: formatDate(now, 'yyyy-MM-ddTHH:mm', 'en-US'),
        valid_until: formatDate(oneHourLater, 'yyyy-MM-ddTHH:mm', 'en-US')
      });

      this.ruleForm.get('server_id')?.enable();
      this.ruleForm.get('system_user_id')?.enable();
      this.ruleForm.get('public_key_id')?.enable();
    }
    this.isModalOpen = true;
  }

  closeModal(): void {
    this.isModalOpen = false;
  }

  onSubmit(): void {
    if (this.ruleForm.invalid) {
      this.ruleForm.markAllAsTouched();
      this.toastr.warning('Lütfen tüm zorunlu alanları doldurun.');
      return;
    }

    if (this.isEditMode) {
      this.handleUpdateRule();
    } else {
      this.handleCreateRule();
    }
  }

  handleCreateRule(): void {
    const formValue = this.ruleForm.value;
    const payload: CreateRulePayload = {
      server_id: Number(formValue.server_id),
      system_user_id: Number(formValue.system_user_id),
      public_key_id: Number(formValue.public_key_id),
      valid_from: new Date(formValue.valid_from).toISOString(),
      valid_until: new Date(formValue.valid_until).toISOString(),
    };

    this.apiClient.createRule(payload).subscribe({
      next: (newRule) => {
        this.toastr.success(`Kural #${newRule.id} başarıyla oluşturuldu.`);
        this.closeModal();
        this.loadInitialData(1);
      },
      error: (err) => {
        const errorMessage = err.error?.message || err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Oluşturma Başarısız');
      }
    });
  }

  handleUpdateRule(): void {
    if (!this.editingRule) return;

    const formValue = this.ruleForm.getRawValue();
    const payload: UpdateRulePayload = {
      valid_from: new Date(formValue.valid_from).toISOString(),
      valid_until: new Date(formValue.valid_until).toISOString(),
    };

    this.apiClient.updateRule(this.editingRule.id, payload).subscribe({
      next: (updatedRule) => {
        this.toastr.success(`Kural #${updatedRule.id} başarıyla güncellendi.`);
        this.closeModal();
        this.loadInitialData(this.currentPage);
      },
      error: (err) => {
        const errorMessage = err.error?.message || err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Güncelleme Başarısız');
      }
    });
  }
}
