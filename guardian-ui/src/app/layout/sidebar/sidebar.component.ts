
import { Component } from '@angular/core';
import { RouterLink, RouterLinkActive } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faClockRotateLeft, faGavel, faServer, faUsers, faKey, faRightFromBracket,   faChartBar  } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-sidebar',
  standalone: true,
   imports: [RouterLink, RouterLinkActive, FaIconComponent], 
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
  
  constructor(private authService: AuthService) {}

  logout(): void {
    this.authService.logout();
  }
}