import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ReplaySessionComponent } from './replay-session.component';

describe('ReplaySessionComponent', () => {
  let component: ReplaySessionComponent;
  let fixture: ComponentFixture<ReplaySessionComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ReplaySessionComponent]
    })
    .compileComponents();

    fixture = TestBed.createComponent(ReplaySessionComponent);
    
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
