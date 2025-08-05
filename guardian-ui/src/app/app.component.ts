
import { Component } from '@angular/core';
import { Router, RouterOutlet, NavigationEnd } from '@angular/router';
import { SidebarComponent } from './layout/sidebar/sidebar.component';
import { CommonModule } from '@angular/common';
import { Observable } from 'rxjs';
import { AuthService } from './core/services/auth.service';
import { filter, map } from 'rxjs/operators';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, SidebarComponent, CommonModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss'
})
export class AppComponent {
  title = 'guardian-ui';

  isAuthenticated$: Observable<boolean>;
  isFullScreen$: Observable<boolean>; 

  constructor(private authService: AuthService, private router: Router) {
    this.isAuthenticated$ = this.authService.isAuthenticated$;

 
    this.isFullScreen$ = this.router.events.pipe(
       filter((event): event is NavigationEnd => event instanceof NavigationEnd),
    
      map((event: NavigationEnd) => {
         return event.urlAfterRedirects.startsWith('/live') || event.urlAfterRedirects.startsWith('/replay');
      })
    );
   }
}