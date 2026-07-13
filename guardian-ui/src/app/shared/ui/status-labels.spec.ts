import { statusLabel, statusTone } from './status-labels';

describe('status-labels', () => {
  describe('statusLabel', () => {
    it('bilinen durumları Türkçeye çevirir', () => {
      expect(statusLabel('active')).toBe('Aktif');
      expect(statusLabel('terminated_by_admin')).toBe('Admin Sonlandırdı');
      expect(statusLabel('awaiting_approval')).toBe('Onay Bekliyor');
    });

    it('bilinmeyen durumu olduğu gibi döndürür', () => {
      expect(statusLabel('bilinmeyen_durum')).toBe('bilinmeyen_durum');
    });
  });

  describe('statusTone', () => {
    it('active → live', () => {
      expect(statusTone('active')).toBe('live');
    });

    it('ended → success', () => {
      expect(statusTone('ended')).toBe('success');
    });

    it('error* ve lost_contact → danger', () => {
      expect(statusTone('error')).toBe('danger');
      expect(statusTone('error_ssh_session')).toBe('danger');
      expect(statusTone('lost_contact')).toBe('danger');
    });

    it('yasak/red durumları → danger', () => {
      expect(statusTone('revoked')).toBe('danger');
      expect(statusTone('terminated_by_ban')).toBe('danger');
      expect(statusTone('rejected')).toBe('danger');
    });

    it('sonlandırma/bekleyen durumları → warning', () => {
      expect(statusTone('terminated_by_admin')).toBe('warning');
      expect(statusTone('timed_out')).toBe('warning');
      expect(statusTone('pending')).toBe('warning');
      expect(statusTone('awaiting_approval')).toBe('warning');
    });

    it('expired → neutral', () => {
      expect(statusTone('expired')).toBe('neutral');
    });

    it('bilinmeyen → neutral', () => {
      expect(statusTone('xyz')).toBe('neutral');
    });
  });
});
