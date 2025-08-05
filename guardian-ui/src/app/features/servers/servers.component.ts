import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ToastrService } from 'ngx-toastr';
import { ApiClientService, Server, CreateServerPayload, UpdateServerPayload } from '../../core/services/api-client.service';

 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faPlus, faServer } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-servers',
  standalone: true,
   imports: [CommonModule, ReactiveFormsModule, FaIconComponent],
  templateUrl: './servers.component.html',
  styleUrl: './servers.component.scss'
})
export class ServersComponent implements OnInit {
  
   faPlus = faPlus;
  faServer = faServer;

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
  totalPages = 0;

  constructor(
    private apiClient: ApiClientService,
    private fb: FormBuilder,
    private toastr: ToastrService
  ) {
    this.serverForm = this.fb.group({
      hostname: ['', Validators.required],
      ip_address: ['', [Validators.required, Validators.pattern(/^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$/)]],
      description: ['']
    });
  }

  ngOnInit(): void {
    this.loadServers(this.currentPage);
  }

   loadServers(page: number = 1): void {
    this.isLoading = true;
    this.error = null;
    this.currentPage = page;

    this.apiClient.getServers(this.currentPage, this.limit).subscribe({
      next: (response) => {
        this.servers = response.data;
        this.totalRecords = response.total_records;
         this.totalPages = Math.ceil(this.totalRecords / this.limit) || 1;
        this.isLoading = false;
      },
      error: (err) => {
        console.error('Sunucu verisi alınırken hata oluştu:', err);
        this.error = 'Sunucu verileri yüklenemedi. API sunucusunun çalıştığından emin olun.';
        this.isLoading = false;
        this.toastr.error(this.error, 'Hata');
      }
    });
  }

   nextPage(): void {
    if (this.currentPage < this.totalPages) {
      this.loadServers(this.currentPage + 1);
    }
  }

  prevPage(): void {
    if (this.currentPage > 1) {
      this.loadServers(this.currentPage - 1);
    }
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

  onDeleteServer(server: Server): void {
    if (!confirm(`EMİN MİSİNİZ?\nSunucu '${server.hostname}' kalıcı olarak silinecek.`)) return;
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