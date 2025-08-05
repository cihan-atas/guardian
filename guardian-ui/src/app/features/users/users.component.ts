// guardian-ui/src/app/features/users/users.component.ts

import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ApiClientService, User, CreateUserPayload, UpdateUserPayload, NullString } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faUsers } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-users',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FaIconComponent],
  templateUrl: './users.component.html',
  styleUrl: './users.component.scss'
})
export class UsersComponent implements OnInit {
  faPlus = faPlus;
  faUsers = faUsers;

  users: User[] = [];
  isLoading = true;
  error: string | null = null;

  isModalOpen = false;
  userForm: FormGroup;

  editingUser: User | null = null;
  isEditMode = false;

  currentPage = 1;
  limit = 8;
  totalRecords = 0;
  totalPages = 0;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private fb: FormBuilder
  ) {
    this.userForm = this.fb.group({
      username: ['', Validators.required],
      description: ['']
    });
  }

  ngOnInit(): void {
    this.loadUsers(this.currentPage);
  }

  loadUsers(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getUsers(this.currentPage, this.limit).subscribe({
      next: (response) => {
        this.users = response.data;
        this.totalRecords = response.total_records;
        this.totalPages = Math.ceil(this.totalRecords / this.limit) || 1;
        this.isLoading = false;
      },
      error: (err) => {
        this.error = 'Kullanıcı verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  nextPage(): void {
    if (this.currentPage < this.totalPages) {
      this.loadUsers(this.currentPage + 1);
    }
  }

  prevPage(): void {
    if (this.currentPage > 1) {
      this.loadUsers(this.currentPage - 1);
    }
  }

  openModal(userToEdit: User | null = null): void {
    if (userToEdit) {
      // --- DÜZENLEME MODU ---
      this.isEditMode = true;
      this.editingUser = userToEdit;
      // DEĞİŞİKLİK: Formu doldururken description'ın içindeki String değerini kullan.
      this.userForm.patchValue({
        username: userToEdit.username,
        description: userToEdit.description.Valid ? userToEdit.description.String : ''
      });
      this.userForm.get('username')?.disable();
    } else {
      // --- OLUŞTURMA MODU ---
      this.isEditMode = false;
      this.editingUser = null;
      this.userForm.reset();
      this.userForm.get('username')?.enable();
    }
    this.isModalOpen = true;
  }

  closeModal(): void {
    this.isModalOpen = false;
  }

  onSubmit(): void {
    if (this.userForm.invalid) {
      this.toastr.warning('Lütfen gerekli alanları doldurun.');
      return;
    }

    if (this.isEditMode) {
      this.handleUpdateUser();
    } else {
      this.handleCreateUser();
    }
  }

  handleCreateUser(): void {
    const formValue = this.userForm.value;

    // DEĞİŞİKLİK: Payload'ı backend'in beklediği NullString formatına çevir.
    const payload: CreateUserPayload = {
      username: formValue.username,
      description: {
        String: formValue.description || '', // Eğer boşsa boş string gönder
        Valid: !!formValue.description    // Eğer doluysa true, boşsa false gönder
      }
    };

    this.apiClient.createUser(payload).subscribe({
      next: () => {
        this.toastr.success('Kullanıcı başarıyla oluşturuldu.');
        this.closeModal();
        this.loadUsers(1);
      },
      error: (err) => this.toastr.error(err.error?.message || err.error || 'Kullanıcı oluşturulamadı.', 'Hata')
    });
  }

  handleUpdateUser(): void {
    if (!this.editingUser) return;

    // DEĞİŞİKLİK: Payload'ı backend'in beklediği NullString formatına çevir.
    const payload: UpdateUserPayload = {
      description: {
        String: this.userForm.get('description')?.value || '',
        Valid: !!this.userForm.get('description')?.value
      }
    };

    this.apiClient.updateUser(this.editingUser.id, payload).subscribe({
      next: (updatedUser) => {
        this.toastr.success(`Kullanıcı '${updatedUser.username}' başarıyla güncellendi.`);
        this.closeModal();
        const index = this.users.findIndex(u => u.id === updatedUser.id);
        if (index !== -1) {
          this.users[index] = updatedUser;
        } else {
          this.loadUsers(this.currentPage);
        }
      },
      error: (err) => this.toastr.error(err.error?.message || err.error || 'Kullanıcı güncellenemedi.', 'Hata')
    });
  }

  onDeleteUser(userId: number): void {
    if (!confirm(`Kullanıcı ID ${userId} silinecek, emin misiniz?`)) return;
    this.apiClient.deleteUser(userId).subscribe({
      next: () => {
        this.toastr.success('Kullanıcı başarıyla silindi.');
        if (this.users.length === 1 && this.currentPage > 1) {
          this.loadUsers(this.currentPage - 1);
        } else {
          this.loadUsers(this.currentPage);
        }
      },
      error: (err) => this.toastr.error(err.error || 'Kullanıcı silinemedi.', 'Hata')
    });
  }
}