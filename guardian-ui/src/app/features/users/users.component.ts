// guardian-ui/src/app/features/users/users.component.ts

import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { ApiClientService, User, CreateUserPayload, UpdateUserPayload } from '../../core/services/api-client.service';
import { ToastrService } from 'ngx-toastr';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators, FormsModule } from '@angular/forms';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faUsers, faSync, faMagnifyingGlass, faGavel } from '@fortawesome/free-solid-svg-icons';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';

@Component({
  selector: 'app-users',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterLink, FaIconComponent, PaginationComponent],
  templateUrl: './users.component.html',
  styleUrl: './users.component.scss'
})
export class UsersComponent implements OnInit, OnDestroy {
  faPlus = faPlus;
  faUsers = faUsers;
  faSync = faSync;
  faSearch = faMagnifyingGlass;
  faRules = faGavel;

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

  searchTerm = '';
  private search$ = new Subject<string>();
  private searchSub?: Subscription;

  constructor(
    private apiClient: ApiClientService,
    private toastr: ToastrService,
    private fb: FormBuilder,
    private confirmDialog: ConfirmDialogService
  ) {
    this.userForm = this.fb.group({
      username: ['', Validators.required],
      description: ['']
    });
  }

  ngOnInit(): void {
    this.loadUsers(1);
    this.searchSub = this.search$.pipe(debounceTime(300)).subscribe(() => this.loadUsers(1));
  }

  ngOnDestroy(): void {
    this.searchSub?.unsubscribe();
  }

  onSearchInput(): void {
    this.search$.next(this.searchTerm);
  }

  /** Go'nun sql.NullString şeklini şablona sızdırmadan düz metne çevirir. */
  desc(user: User): string {
    return user.description?.Valid ? user.description.String : '';
  }

  loadUsers(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getUsers(this.currentPage, this.limit, this.searchTerm).subscribe({
      next: (response) => {
        this.users = response.data ?? [];
        this.totalRecords = response.total_records;
        this.isLoading = false;
      },
      error: () => {
        this.error = 'Kullanıcı verileri yüklenemedi.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  openModal(userToEdit: User | null = null): void {
    if (userToEdit) {
      this.isEditMode = true;
      this.editingUser = userToEdit;
      this.userForm.patchValue({
        username: userToEdit.username,
        description: this.desc(userToEdit)
      });
      this.userForm.get('username')?.disable();
    } else {
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

    // Backend'in beklediği sql.NullString formatı.
    const payload: CreateUserPayload = {
      username: formValue.username,
      description: {
        String: formValue.description || '',
        Valid: !!formValue.description
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

  async onDeleteUser(user: User): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Kullanıcıyı Sil',
      message: `'${user.username}' sistem kullanıcısı kalıcı olarak silinecek. Bu kullanıcıya bağlı kurallar da etkilenir.`,
      confirmText: 'Sil',
      danger: true,
    });
    if (!ok) return;

    this.apiClient.deleteUser(user.id).subscribe({
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
