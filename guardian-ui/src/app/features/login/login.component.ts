 import { Component } from '@angular/core'; 
import { CommonModule } from '@angular/common'; 
import { ReactiveFormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms'; 
import { AuthService } from '../../core/services/auth.service'; 
import { ApiClientService } from '../../core/services/api-client.service'; 
import { ToastrService } from 'ngx-toastr'; 

 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FaIconComponent], 
  templateUrl: './login.component.html',
  styleUrl: './login.component.scss'
})
export class LoginComponent {
  faSpinner = faSpinner;
  
  loginForm: FormGroup;
  isChecking = false;

  constructor(
    private fb: FormBuilder,
    private authService: AuthService,
    private apiClient: ApiClientService,
    private toastr: ToastrService
  ) {
    this.loginForm = this.fb.group({
      token: ['', Validators.required]
    });
  }

  onSubmit(): void {
    if (this.loginForm.invalid || !this.loginForm.value.token) {
      return;
    }

    this.isChecking = true;
    const token = this.loginForm.value.token;

    this.apiClient.checkAuth(token).subscribe({
      next: () => {
        this.isChecking = false;
        this.authService.login(token);
      },
       error: (err: any) => { 
        this.isChecking = false;
        console.error('Login hatası:', err);
        this.toastr.error('Girilen token geçersiz veya yetkisiz.', 'Giriş Başarısız');
      }
    });
  }
}