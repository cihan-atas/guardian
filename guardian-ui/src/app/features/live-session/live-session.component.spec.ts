/*==================================================
Dosya: guardian-ui/src/app/features/live-session/live-session.component.spec.ts (DÜZELTİLMİŞ)
==================================================*/
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { LiveSessionComponent } from './live-session.component'; // <-- Düzeltme: Doğru component adını import et

describe('LiveSessionComponent', () => { // <-- Düzeltme: Describe bloğu adını da güncelleyelim
  let component: LiveSessionComponent; // <-- Düzeltme: Doğru tipi kullan
  let fixture: ComponentFixture<LiveSessionComponent>; // <-- Düzeltme: Doğru tipi kullan

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [LiveSessionComponent] // <-- Düzeltme: Doğru component adını kullan
    })
    .compileComponents();

    fixture = TestBed.createComponent(LiveSessionComponent); // <-- Düzeltme: Doğru component adını kullan
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});