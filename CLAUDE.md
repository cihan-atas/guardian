# Guardian — Proje Kuralları

## Commit kuralları
- **Commit mesajlarına hiçbir AI imzası eklenmeyecek.** `Co-Authored-By: Claude ...`, `Generated with Claude Code` veya benzeri Claude/Anthropic imza ve trailer'ları KULLANILMAZ. Commit'ler yalnızca repo sahibinin git kimliğiyle atılır.
- Commit mesajları kısa ve konuya odaklı; gövde gerekiyorsa madde işaretli.

## Proje notları
- İlerleme takibi ve yol haritası `ilerleme.md` dosyasında tutulur; büyük bir iş bitince oraya işlenir.
- Deploy sudo gerektirir; sudo komutları kullanıcıya verilir, asistan kendisi sudo çalıştırmaz.
- UI production build: `guardian-ui/` içinde `npx ng build --configuration production` → çıktı `dist/guardian-ui/browser`, nginx `/var/www/guardian-ui`'dan servis eder.
- Backend/agent derleme çıktıları (`guardian-server-new`, `guardian-agent-new` vb.) commit'lenmez.
