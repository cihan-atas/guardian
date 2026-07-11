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
      username: ['', Validators.required],
      password: ['', Validators.required]
    });
  }

  onSubmit(): void {
    if (this.loginForm.invalid) {
      return;
    }

    this.isChecking = true;
    const { username, password } = this.loginForm.value;

    this.apiClient.login(username, password).subscribe({
      next: (res) => {
        this.isChecking = false;
        this.authService.login({
          token: res.token,
          username: res.username,
          role: res.role,
          display_name: res.display_name,
        });
      },
      error: (err: any) => {
        this.isChecking = false;
        console.error('Login hatası:', err);
        this.toastr.error('Kullanıcı adı veya parola hatalı.', 'Giriş Başarısız');
      }
    });
  }
}