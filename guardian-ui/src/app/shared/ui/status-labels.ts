// Oturum/kural durum kodlarının Türkçe etiketleri ve renk tonları.
// Backend İngilizce snake_case durum stringleri döner; UI genelinde
// (dashboard, oturumlar, kurallar) bu tek eşleme kullanılır.

export const STATUS_LABELS: Record<string, string> = {
  active: 'Aktif',
  pending: 'Bekliyor',
  awaiting_approval: 'Onay Bekliyor',
  rejected: 'Reddedildi',
  expired: 'Süresi Doldu',
  revoked: 'İptal Edildi (Yasak)',
  ended: 'Normal Bitti',
  timed_out: 'Süre Doldu',
  error: 'Hata',
  lost_contact: 'Bağlantı Koptu',
  terminated_by_admin: 'Admin Sonlandırdı',
  terminated_by_expiry: 'Süre Doldu',
  terminated_by_ban: 'Yasak Nedeniyle',
  terminated_by_rule_deletion: 'Kural Silindi',
  error_pid_creation: 'Hata (PID)',
  error_ssh_session: 'Hata (SSH)',
  error_ws_connect: 'Hata (WS)',
  error_shell: 'Hata (Shell)',
};

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status;
}

export type StatusTone = 'live' | 'success' | 'warning' | 'danger' | 'neutral';

/** Durum kodunu görsel tona eşler (status-badge renkleri bu tondan türer). */
export function statusTone(status: string): StatusTone {
  if (status === 'active') return 'live';
  if (status === 'ended') return 'success';
  if (status.startsWith('error') || status === 'lost_contact') return 'danger';
  if (status === 'revoked' || status === 'terminated_by_ban') return 'danger';
  if (status === 'rejected') return 'danger';
  if (status.startsWith('terminated') || status === 'timed_out' || status === 'pending' || status === 'awaiting_approval') return 'warning';
  if (status === 'expired') return 'neutral';
  return 'neutral';
}
