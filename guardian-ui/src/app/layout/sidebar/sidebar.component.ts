
import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
import { ApiClientService } from '../../core/services/api-client.service';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faClockRotateLeft, faGavel, faServer, faUsers, faKey, faRightFromBracket, faChartBar, faGear, faInbox, faUserShield, faClipboardList } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-sidebar',
  standalone: true,
  imports: [CommonModule, RouterLink, RouterLinkActive, FaIconComponent],
  templateUrl: './sidebar.component.html',
  styleUrl: './sidebar.component.scss'
})
export class SidebarComponent {
  faSessions = faClockRotateLeft;
  faRules = faGavel;
  faServers = faServer;
  faUsers = faUsers;
  faKeys = faKey;
  faLogout = faRightFromBracket;
  faDashboard = faChartBar;
  faSettings = faGear;
  faRequests = faInbox;
  faAdmins = faUserShield;
  faAudit = faClipboardList;

  constructor(private authService: AuthService, private api: ApiClientService) {}

  get username(): string {
    return this.authService.session?.display_name || this.authService.session?.username || '';
  }

  get roleLabel(): string {
    switch (this.authService.role) {
      case 'admin': return 'Yönetici';
      case 'operator': return 'Operatör';
      case 'viewer': return 'İzleyici';
      default: return '';
    }
  }

  isAdmin(): boolean {
    return this.authService.hasRole('admin');
  }

  logout(): void {
    // Sunucu tarafındaki oturumu da geçersiz kıl; sonucu beklemeden çık.
    this.api.logout().subscribe({ next: () => {}, error: () => {} });
    this.authService.logout();
  }
}
