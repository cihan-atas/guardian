import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators, FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, Server, CreateServerPayload, UpdateServerPayload } from '../../core/services/api-client.service';

import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faServer, faSync, faMagnifyingGlass, faClockRotateLeft } from '@fortawesome/free-solid-svg-icons';
import { PaginationComponent } from '../../shared/ui/pagination/pagination.component';
import { CopyButtonComponent } from '../../shared/ui/copy-button/copy-button.component';
import { ConfirmDialogService } from '../../shared/ui/confirm-dialog/confirm-dialog.service';

@Component({
  selector: 'app-servers',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, RouterLink, FaIconComponent, PaginationComponent, CopyButtonComponent],
  templateUrl: './servers.component.html',
  styleUrl: './servers.component.scss'
})
export class ServersComponent implements OnInit, OnDestroy {
  faPlus = faPlus;
  faServer = faServer;
  faSync = faSync;
  faSearch = faMagnifyingGlass;
  faSessions = faClockRotateLeft;

  servers: Server[] = [];
  isLoading = true;
  error: string | null = null;

  isModalOpen = false;
  serverForm: FormGroup;

  editingServer: Server | null = null;
  isEditMode = false;

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
    private confirmDialog: ConfirmDialogService
  ) {
    this.serverForm = this.fb.group({
      hostname: ['', Validators.required],
      ip_address: ['', [Validators.required, Validators.pattern(/^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/)]],
      description: ['']
    });
  }

  ngOnInit(): void {
    this.loadServers(1);
    this.searchSub = this.search$.pipe(debounceTime(300)).subscribe(() => this.loadServers(1));
  }

  ngOnDestroy(): void {
    this.searchSub?.unsubscribe();
  }

  onSearchInput(): void {
    this.search$.next(this.searchTerm);
  }

  loadServers(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getServers(this.currentPage, this.limit, this.searchTerm).subscribe({
      next: (response) => {
        this.servers = response.data ?? [];
        this.totalRecords = response.total_records;
        this.isLoading = false;
      },
      error: () => {
        this.error = 'Sunucu verileri yüklenemedi. API sunucusunun çalıştığından emin olun.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

  isExampleServer(server: Server): boolean {
    return server.hostname.startsWith('example-');
  }

  openModal(serverToEdit: Server | null = null): void {
    if (serverToEdit) {
      this.isEditMode = true;
      this.editingServer = serverToEdit;
      this.serverForm.setValue({
        hostname: serverToEdit.hostname,
        ip_address: serverToEdit.ip_address,
        description: serverToEdit.description || ''
      });
    } else {
      this.isEditMode = false;
      this.editingServer = null;
      this.serverForm.reset();
    }
    this.isModalOpen = true;
  }

  closeModal(): void {
    this.isModalOpen = false;
  }

  onSubmit(): void {
    if (this.serverForm.invalid) {
      this.toastr.warning('Lütfen tüm zorunlu alanları doğru şekilde doldurun.');
      return;
    }
    if (this.isEditMode) {
      this.handleUpdateServer();
    } else {
      this.handleCreateServer();
    }
  }

  handleCreateServer(): void {
    const payload: CreateServerPayload = this.serverForm.value;
    this.apiClient.createServer(payload).subscribe({
      next: () => {
        this.toastr.success(`Sunucu başarıyla oluşturuldu.`);
        this.closeModal();
        this.loadServers(1);
      },
      error: (err) => this.toastr.error(err.error || 'Oluşturma Başarısız', 'Hata')
    });
  }

  handleUpdateServer(): void {
    if (!this.editingServer) return;
    const payload: UpdateServerPayload = this.serverForm.value;
    this.apiClient.updateServer(this.editingServer.id, payload).subscribe({
      next: (updatedServer) => {
        this.toastr.success(`Sunucu '${updatedServer.hostname}' başarıyla güncellendi.`);
        this.closeModal();
        const index = this.servers.findIndex(s => s.id === updatedServer.id);
        if (index !== -1) {
          this.servers[index] = updatedServer;
        } else {
          this.loadServers(this.currentPage);
        }
      },
      error: (err) => this.toastr.error(err.error || 'Güncelleme Başarısız', 'Hata')
    });
  }

  async onDeleteServer(server: Server): Promise<void> {
    const ok = await this.confirmDialog.confirm({
      title: 'Sunucuyu Sil',
      message: `'${server.hostname}' (${server.ip_address}) kalıcı olarak silinecek. Bu sunucuya bağlı kurallar da etkilenir.`,
      confirmText: 'Sil',
      danger: true,
    });
    if (!ok) return;

    this.apiClient.deleteServer(server.id).subscribe({
      next: () => {
        this.toastr.success(`Sunucu '${server.hostname}' başarıyla silindi.`);
        if (this.servers.length === 1 && this.currentPage > 1) {
          this.loadServers(this.currentPage - 1);
        } else {
          this.loadServers(this.currentPage);
        }
      },
      error: (err) => this.toastr.error(err.error || 'Silme Başarısız', 'Hata')
    });
  }
}
