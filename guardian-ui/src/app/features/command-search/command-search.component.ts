import { Component, OnDestroy, OnInit, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { Subject, Subscription, debounceTime } from 'rxjs';
import { FaIconComponent } from '@fortawesome/angular-fontawesome';
import { faSpinner, faMagnifyingGlass, faTerminal, faTriangleExclamation, faPlay } from '@fortawesome/free-solid-svg-icons';
import { ToastrService } from 'ngx-toastr';

import { ApiClientService, CommandMatch } from '../../core/services/api-client.service';
import { isRiskyCommand } from '../replay-session/replay-session.component';
import { StatusBadgeComponent } from '../../shared/ui/status-badge/status-badge.component';

@Component({
  selector: 'app-command-search',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink, FaIconComponent, StatusBadgeComponent],
  templateUrl: './command-search.component.html',
})
export class CommandSearchComponent implements OnInit, OnDestroy {
  private api = inject(ApiClientService);
  private toastr = inject(ToastrService);

  faSpinner = faSpinner;
  faSearch = faMagnifyingGlass;
  faTerminal = faTerminal;
  faRisky = faTriangleExclamation;
  faPlay = faPlay;

  term = '';
  results: CommandMatch[] = [];
  isLoading = false;
  searched = false;

  private search$ = new Subject<string>();
  private sub?: Subscription;

  ngOnInit(): void {
    this.sub = this.search$.pipe(debounceTime(350)).subscribe((q) => this.run(q));
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
  }

  onInput(): void {
    this.search$.next(this.term);
  }

  private run(q: string): void {
    const query = q.trim();
    if (query.length < 2) {
      this.results = [];
      this.searched = false;
      return;
    }
    this.isLoading = true;
    this.api.searchCommands(query, 200).subscribe({
      next: (rows) => {
        this.results = rows;
        this.isLoading = false;
        this.searched = true;
      },
      error: () => {
        this.isLoading = false;
        this.searched = true;
        this.toastr.error('Arama başarısız.');
      },
    });
  }

  isRisky(cmd: string): boolean {
    return isRiskyCommand(cmd);
  }
}
