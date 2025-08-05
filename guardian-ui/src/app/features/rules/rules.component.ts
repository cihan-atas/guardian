import { Component, OnInit } from '@angular/core';
import { CommonModule, formatDate } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { forkJoin } from 'rxjs';
import { ApiClientService, CreateRulePayload, Key, Rule, Server, User, UpdateRulePayload } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faGavel } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-rules',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FaIconComponent],
  templateUrl: './rules.component.html',
  styleUrl: './rules.component.scss'
})
export class RulesComponent implements OnInit {
  faPlus = faPlus;
  faGavel = faGavel;

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
  totalPages = 0;

  constructor(
    private apiClient: ApiClientService,
    private fb: FormBuilder,
    private toastr: ToastrService
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
    this.loadInitialData(this.currentPage);
  }

  loadInitialData(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    forkJoin({
      rulesResponse: this.apiClient.getRules(this.currentPage, this.limit),
      serversResponse: this.apiClient.getServers(1, 1000),
      usersResponse: this.apiClient.getUsers(1, 1000),
      keysResponse: this.apiClient.getKeys(1, 1000),
    }).subscribe({
      next: (data) => {
        this.rules = data.rulesResponse.data;
        this.totalRecords = data.rulesResponse.total_records;
        this.totalPages = Math.ceil(this.totalRecords / this.limit) || 1;
        this.servers = data.serversResponse.data;
        this.users = data.usersResponse.data;
        this.keys = data.keysResponse.data;
        this.isLoading = false;
      },
      error: (err) => {
        this.error = 'Sayfa verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error('Sayfa verileri yüklenemedi. API bağlantısını kontrol edin.', 'Yükleme Hatası');
      }
    });
  }

  nextPage(): void {
    if (this.currentPage < this.totalPages) {
      this.loadInitialData(this.currentPage + 1);
    }
  }

  prevPage(): void {
    if (this.currentPage > 1) {
      this.loadInitialData(this.currentPage - 1);
    }
  }

  onDeleteRule(ruleId: number): void {
    if (!confirm(`Emin misiniz? Kural ID ${ruleId} kalıcı olarak silinecek.`)) return;

    this.apiClient.deleteRule(ruleId).subscribe({
      next: () => {
        this.toastr.success(`Kural ID ${ruleId} başarıyla silindi.`);
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


// ...

  openModal(ruleToEdit: Rule | null = null): void {
    if (ruleToEdit) {
      // Düzenleme Modu
      this.isEditMode = true;
      this.editingRule = ruleToEdit;

      this.ruleForm.patchValue({
        server_id: ruleToEdit.server_id,
        system_user_id: ruleToEdit.system_user_id,
        public_key_id: ruleToEdit.public_key_id,
        // ===== BAŞLANGIÇ: DEĞİŞİKLİK (Yazım hatası düzeltildi) =====
        valid_from: formatDate(ruleToEdit.valid_from, 'yyyy-MM-ddTHH:mm', 'en-US'),
        valid_until: formatDate(ruleToEdit.valid_until, 'yyyy-MM-ddTHH:mm', 'en-US'),
        // =====  BİTİŞ: DEĞİŞİKLİK  =====
      });

      this.ruleForm.get('server_id')?.disable();
      this.ruleForm.get('system_user_id')?.disable();
      this.ruleForm.get('public_key_id')?.disable();
    } else {
      // Oluşturma Modu (Bu kısım doğru, değişiklik yok)
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
        this.toastr.success(`Kural ID ${newRule.id} başarıyla oluşturuldu.`);
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
        this.toastr.success(`Kural ID ${updatedRule.id} başarıyla güncellendi.`);
        this.closeModal();
        this.loadInitialData(this.currentPage); // Mevcut sayfayı yenile
      },
      error: (err) => {
        const errorMessage = err.error?.message || err.error || 'Bilinmeyen bir hata oluştu.';
        this.toastr.error(errorMessage, 'Güncelleme Başarısız');
      }
    });
  }
}