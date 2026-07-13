import { formatRemaining, formatDuration, windowPercent } from './time-utils';

describe('time-utils', () => {
  describe('formatRemaining', () => {
    it('0 veya negatif için "doldu" döner', () => {
      expect(formatRemaining(0)).toBe('doldu');
      expect(formatRemaining(-5000)).toBe('doldu');
    });

    it('60 dakikadan az için "N dk" döner', () => {
      expect(formatRemaining(45 * 60000)).toBe('45 dk');
      expect(formatRemaining(1 * 60000)).toBe('1 dk');
    });

    it('24 saatten az için "Ns Mdk" döner', () => {
      expect(formatRemaining((2 * 60 + 15) * 60000)).toBe('2s 15dk');
    });

    it('24 saatten fazla için "Ng Ms" döner', () => {
      expect(formatRemaining((3 * 24 * 60 + 4 * 60) * 60000)).toBe('3g 4s');
    });
  });

  describe('formatDuration', () => {
    it('60 saniyeden az için "N sn" döner', () => {
      expect(formatDuration(0, 45000)).toBe('45 sn');
    });

    it('negatif aralığı 0 sn olarak ele alır', () => {
      expect(formatDuration(10000, 0)).toBe('0 sn');
    });

    it('60 dakikadan az için "N dk" döner', () => {
      expect(formatDuration(0, 12 * 60000)).toBe('12 dk');
    });

    it('1 saatten fazla için "Ns Mdk" döner', () => {
      expect(formatDuration(0, (60 + 5) * 60000)).toBe('1s 5dk');
    });
  });

  describe('windowPercent', () => {
    const from = '2026-01-01T00:00:00Z';
    const until = '2026-01-01T10:00:00Z';

    it('başlangıçta 0 döner', () => {
      expect(windowPercent(from, until, new Date(from).getTime())).toBe(0);
    });

    it('ortada 50 döner', () => {
      const mid = new Date('2026-01-01T05:00:00Z').getTime();
      expect(windowPercent(from, until, mid)).toBe(50);
    });

    it('0-100 aralığına sıkıştırır', () => {
      const before = new Date('2025-12-31T00:00:00Z').getTime();
      const after = new Date('2026-01-02T00:00:00Z').getTime();
      expect(windowPercent(from, until, before)).toBe(0);
      expect(windowPercent(from, until, after)).toBe(100);
    });

    it('geçersiz pencere (until<=from) için 100 döner', () => {
      expect(windowPercent(until, from, Date.now())).toBe(100);
    });
  });
});
