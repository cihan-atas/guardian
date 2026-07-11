// Süre biçimlendirme yardımcıları — dashboard, kurallar ve oturumlar
// ekranlarının ortak kullandığı, Türkçe kısa formatlar.

/** Kalan süreyi "45 dk" / "2s 15dk" / "3g 4s" biçiminde verir. */
export function formatRemaining(ms: number): string {
  if (ms <= 0) return 'doldu';
  const m = Math.floor(ms / 60000);
  if (m < 60) return `${m} dk`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}s ${m % 60}dk`;
  return `${Math.floor(h / 24)}g ${h % 24}s`;
}

/** İki zaman arası süreyi "45 sn" / "12 dk" / "1s 05dk" biçiminde verir. */
export function formatDuration(startMs: number, endMs: number): string {
  const ms = Math.max(0, endMs - startMs);
  const totalSec = Math.floor(ms / 1000);
  if (totalSec < 60) return `${totalSec} sn`;
  const m = Math.floor(totalSec / 60);
  if (m < 60) return `${m} dk`;
  return `${Math.floor(m / 60)}s ${m % 60}dk`;
}

/** Bir zaman penceresinin yüzde kaçının geçtiği (ilerleme çubuğu için, 0-100). */
export function windowPercent(fromIso: string, untilIso: string, nowMs: number): number {
  const from = new Date(fromIso).getTime();
  const until = new Date(untilIso).getTime();
  if (until <= from) return 100;
  const pct = ((nowMs - from) / (until - from)) * 100;
  return Math.max(0, Math.min(100, pct));
}
