/** @type {import('tailwindcss').Config} */

// Tailwind'in varsayılan renklerini import ederek kendi paletimize ekleyebiliriz.
const colors = require('tailwindcss/colors');

module.exports = {
  content: [
    "./src/**/*.{html,ts}",
  ],
  theme: {
    extend: {
      // Font ailesi tanımınız mükemmel, aynen kalıyor.
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
        // Terminal/SSH ürünü olduğumuz için sayısal veri ve durum
        // okumalarında (dashboard metrikleri, zaman damgaları) bir
        // "terminal readout" hissi vermek üzere eklendi.
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'monospace'],
      },

      // --- BAŞLANGIÇ: YENİ KARANLIK TEMA RENK PALETİ ---
      colors: {
        'dark': {
          'main': '#0d1117',        // Ana Arka Plan (Body)
          'card': '#161b22',        // Kart, Modal, Sidebar Arka Planı
          'border': '#30363d',      // Ayırıcılar, Kenarlıklar
          'interactive': '#21262d'   // Hover/Aktif Butonlar, Etkileşimli Alanlar
        },
        'accent': {
          'DEFAULT': '#2f81f7',     // Ana Vurgu Rengi (Butonlar, Linkler)
          'hover': '#388bfd'       // Vurgu Rengi Hover
        },
        'live': {
          'DEFAULT': '#39c5cf',     // Canlı/aktif durum vurgusu (dashboard nabız göstergesi)
          'dim': '#164d52'
        },
        'text': {
          'main': '#c9d1d9',        // Ana Metin Rengi
          'secondary': '#8b949e'    // İkincil, soluk metin rengi
        },
        // --- BİTİŞ: YENİ KARANLIK TEMA RENK PALETİ ---
        
        // Mevcut renklerinizi de koruyoruz, gerekirse kullanılabilir.
        'primary': colors.blue,
        'slate': colors.slate,
        'success': colors.green,
        'warning': colors.amber,
        'danger': colors.red,
      },

      // Animasyonlar için keyframe'leri tanımlıyoruz (Aynen kalıyor).
      keyframes: {
        'fade-in': {
          '0%': {
            opacity: '0',
            transform: 'scale(0.95)'
          },
          '100%': {
            opacity: '1',
            transform: 'scale(1)'
          },
        },
      },
      // Tanımladığımız keyframe'leri kullanacak animasyon class'ını oluşturuyoruz (Aynen kalıyor).
      animation: {
        'fade-in': 'fade-in 0.2s ease-out',
      },
    },
  },
  plugins: [],
};