import { Component, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faUserShield, faPlus, faPen, faTrash, faUserSlash, faUserCheck } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, AdminUser, Role } from '../../core/services/api-client.service';
import { AuthService } from '../../core/services/auth.service';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';

interface EditModel {
  id: number | null;      // null → yeni
  username: string;
  display_name: string;
  role: Role;
  password: string;
}

@Component({
  selector: 'app-admin-users',
  standalone: true,
  imports: [CommonModule, FormsModule, FaIconComponent],
  templateUrl: './admin-users.component.html',
})
export class AdminUsersComponent implements OnInit {
  private api = inject(ApiClientService);
  private auth = inject(AuthService);
  private toastr = inject(ToastrService);
  private confirm = inject(ConfirmDialogService);

  faSpinner = faSpinner;
  faUserShield = faUserShield;
  faPlus = faPlus;
  faPen = faPen;
  faTrash = faTrash;
  faUserSlash = faUserSlash;
  faUserCheck = faUserCheck;

  isLoading = true;
  isSaving = false;
  users: AdminUser[] = [];

  showForm = false;
  model: EditModel = this.emptyModel();

  readonly roles: { value: Role; label: string; hint: string }[] = [
    { value: 'viewer', label: 'İzleyici', hint: 'Yalnızca görüntüleme (salt-okunur).' },
    { value: 'operator', label: 'Operatör', hint: 'İzleme + erişim talebi + oturum sonlandırma.' },
    { value: 'admin', label: 'Yönetici', hint: 'Tam yetki: kural/anahtar/kullanıcı yönetimi, onaylar, ayarlar.' },
  ];

  ngOnInit(): void { this.load(); }

  roleLabel(role: string): string {
    return this.roles.find(r => r.value === role)?.label ?? role;
  }

  isSelf(u: AdminUser): boolean {
    return u.username === this.auth.session?.username;
  }

  load(): void {
    this.isLoading = true;
    this.api.getAdminUsers().subscribe({
      next: (rows) => { this.users = rows; this.isLoading = false; },
      error: () => { this.isLoading = false; this.toastr.error('Yöneticiler yüklenemedi.'); },
    });
  }

  openCreate(): void {
    this.model = this.emptyModel();
    this.showForm = true;
  }

  openEdit(u: AdminUser): void {
    this.model = { id: u.id, username: u.username, display_name: u.display_name, role: u.role, password: '' };
    this.showForm = true;
  }

  save(): void {
    if (this.model.id === null) {
      // Oluştur
      if (!this.model.username.trim() || this.model.password.length < 6) {
        this.toastr.warning('Kullanıcı adı gerekli ve parola en az 6 karakter olmalı.');
        return;
      }
      this.isSaving = true;
      this.api.createAdminUser({
        username: this.model.username.trim(),
        password: this.model.password,
        role: this.model.role,
        display_name: this.model.display_name,
      }).subscribe({
        next: () => { this.isSaving = false; this.showForm = false; this.toastr.success('Yönetici oluşturuldu.'); this.load(); },
        error: (err) => { this.isSaving = false; this.toastr.error(err?.error || 'Oluşturulamadı.'); },
      });
    } else {
      // Güncelle
      if (this.model.password && this.model.password.length < 6) {
        this.toastr.warning('Parola en az 6 karakter olmalı.');
        return;
      }
      this.isSaving = true;
      this.api.updateAdminUser(this.model.id, {
        role: this.model.role,
        display_name: this.model.display_name,
        ...(this.model.password ? { password: this.model.password } : {}),
      }).subscribe({
        next: () => { this.isSaving = false; this.showForm = false; this.toastr.success('Yönetici güncellendi.'); this.load(); },
        error: (err) => { this.isSaving = false; this.toastr.error(err?.error || 'Güncellenemedi.'); },
      });
    }
  }

  async toggleDisabled(u: AdminUser): Promise<void> {
    const disabling = !u.disabled;
    const ok = await this.confirm.confirm({
      title: disabling ? 'Hesabı Devre Dışı Bırak' : 'Hesabı Etkinleştir',
      message: `"${u.username}" hesabı ${disabling ? 'devre dışı bırakılacak ve oturumları düşürülecek' : 'yeniden etkinleştirilecek'}.`,
      confirmText: disabling ? 'Devre Dışı Bırak' : 'Etkinleştir',
      danger: disabling,
    });
    if (!ok) return;
    this.api.updateAdminUser(u.id, { disabled: disabling }).subscribe({
      next: () => { this.toastr.success('Güncellendi.'); this.load(); },
      error: (err) => this.toastr.error(err?.error || 'Güncellenemedi.'),
    });
  }

  async remove(u: AdminUser): Promise<void> {
    const ok = await this.confirm.confirm({
      title: 'Yöneticiyi Sil',
      message: `"${u.username}" hesabı kalıcı olarak silinecek.`,
      confirmText: 'Sil',
      danger: true,
    });
    if (!ok) return;
    this.api.deleteAdminUser(u.id).subscribe({
      next: () => { this.toastr.success('Silindi.'); this.load(); },
      error: (err) => this.toastr.error(err?.error || 'Silinemedi.'),
    });
  }

  private emptyModel(): EditModel {
    return { id: null, username: '', display_name: '', role: 'viewer', password: '' };
  }
}
