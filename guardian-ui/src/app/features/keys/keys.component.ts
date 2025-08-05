import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, Key, CreateKeyPayload, UpdateKeyPayload } from '../../core/services/api-client.service';

 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faKey } from '@fortawesome/free-solid-svg-icons'; // faKey ikonunu buraya ekleyin

@Component({
  selector: 'app-keys',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FaIconComponent],
  templateUrl: './keys.component.html',
  styleUrl: './keys.component.scss'
})
export class KeysComponent implements OnInit {
   faPlus = faPlus;
  faKey = faKey; 

  keys: Key[] = [];
  isLoading = true;
 
  error: string | null = null;

  isModalOpen = false;
  keyForm: FormGroup;

   editingKey: Key | null = null;
  isEditMode = false;
currentPage = 1;
  limit = 8;
  totalRecords = 0;
  totalPages = 0;

  constructor(
    private apiClient: ApiClientService,
    private fb: FormBuilder,
    private toastr: ToastrService
  ) {
    this.keyForm = this.fb.group({
      key_name: ['', Validators.required],
       ssh_public_key: ['', Validators.required]
    });
  }

  ngOnInit(): void {
    this.loadKeys(this.currentPage);
  }

  loadKeys(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getKeys(this.currentPage, this.limit).subscribe({
      next: (response) => {
        this.keys = response.data;
        this.totalRecords = response.total_records;
        this.totalPages = Math.ceil(this.totalRecords / this.limit) || 1;
        this.isLoading = false;
      },
      error: (err) => {
        this.error = 'SSH Anahtar verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

   nextPage(): void {
    if (this.currentPage < this.totalPages) {
      this.loadKeys(this.currentPage + 1);
    }
  }

  prevPage(): void {
    if (this.currentPage > 1) {
      this.loadKeys(this.currentPage - 1);
    }
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
          this.keys[index] = updatedKey;
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

 onDeleteKey(key: Key): void {
    if (!confirm(`EMİN MİSİNİZ?\nAnahtar '${key.key_name}' kalıcı olarak silinecek.`)) return;
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