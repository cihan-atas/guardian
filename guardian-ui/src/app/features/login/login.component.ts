 import { Component } from '@angular/core'; 
import { CommonModule } from '@angular/common'; 
import { ReactiveFormsModule, FormsModule, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { AuthService } from '../../core/services/auth.service'; 
import { ApiClientService } from '../../core/services/api-client.service'; 
import { ToastrService } from 'ngx-toastr'; 

 import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner } from '@fortawesome/free-solid-svg-icons';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, FaIconComponent],
  templateUrl: './login.component.html',
  styleUrl: './login.component.scss'
})
export class LoginComponent {
  faSpinner = faSpinner;
  
  loginForm: FormGroup;
  isChecking = false;
  // İkinci adım: parola doğru ama hesapta 2FA açık → doğrulama kodu istenir.
  totpRequired = false;
  totpCode = '';

  constructor(
    private fb: FormBuilder,
    private authService: AuthService,
    private apiClient: ApiClientService,
    private toastr: ToastrService
  ) {
    this.loginForm = this.fb.group({
      username: ['', Validators.required],
      password: ['', Validators.required]
    });
  }

  onSubmit(): void {
    if (this.loginForm.invalid) {
      return;
    }
    if (this.totpRequired && !this.totpCode.trim()) {
      return;
    }

    this.isChecking = true;
    const { username, password } = this.loginForm.value;

    this.apiClient.login(username, password, this.totpCode).subscribe({
      next: (res) => {
        this.isChecking = false;
        if (res.totp_required) {
          // 2FA gerekli — ikinci adıma geç.
          this.totpRequired = true;
          return;
        }
        if (!res.token) {
          this.toastr.error('Beklenmeyen yanıt.', 'Giriş Başarısız');
          return;
        }
        this.authService.login({
          token: res.token,
          username: res.username!,
          role: res.role!,
          display_name: res.display_name!,
        });
      },
      error: (err: any) => {
        this.isChecking = false;
        if (this.totpRequired) {
          this.toastr.error('Doğrulama kodu hatalı.', 'Giriş Başarısız');
          this.totpCode = '';
        } else {
          this.toastr.error('Kullanıcı adı veya parola hatalı.', 'Giriş Başarısız');
        }
      }
    });
  }

  /** Kullanıcı/parola adımına geri dön (2FA'dan vazgeç). */
  cancelTotp(): void {
    this.totpRequired = false;
    this.totpCode = '';
  }
}