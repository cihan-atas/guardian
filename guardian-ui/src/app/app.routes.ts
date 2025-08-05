import { Routes } from '@angular/router';
import { ServersComponent } from './features/servers/servers.component';
import { UsersComponent } from './features/users/users.component';
import { KeysComponent } from './features/keys/keys.component';
import { RulesComponent } from './features/rules/rules.component';
import { SessionsComponent } from './features/sessions/sessions.component';
import { LoginComponent } from './features/login/login.component';
import { authGuard } from './core/guards/auth.guard';
import { LiveSessionComponent } from './features/live-session/live-session.component';
import { ReplaySessionComponent } from './features/replay-session/replay-session.component';
import { DashboardComponent } from './features/dashboard/dashboard.component'; 

export const routes: Routes = [
    { path: 'login', component: LoginComponent },
    
    
    { path: 'live/:id', component: LiveSessionComponent, canActivate: [authGuard] },
    { path: 'replay/:id', component: ReplaySessionComponent, canActivate: [authGuard] },

    { 
      path: '', 
      canActivate: [authGuard],
      children: [
        { path: '', redirectTo: 'dashboard', pathMatch: 'full' },
        { path: 'dashboard', component: DashboardComponent },  
        { path: 'servers', component: ServersComponent },
        { path: 'users', component: UsersComponent },
        { path: 'keys', component: KeysComponent },
        { path: 'rules', component: RulesComponent },
        { path: 'sessions', component: SessionsComponent },
      ]
    },

    { path: '**', redirectTo: 'login' }
];