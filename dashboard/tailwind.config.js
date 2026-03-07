/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        kronveil: {
          bg: '#040811',
          surface: '#0c1526',
          border: 'rgba(99,179,237,0.12)',
          accent: '#63b3ed',
          accent2: '#4fd1c5',
          warn: '#f6ad55',
          danger: '#fc8181',
          success: '#68d391',
          muted: '#718096',
        },
      },
      fontFamily: {
        mono: ['Space Mono', 'monospace'],
        display: ['Syne', 'sans-serif'],
        body: ['Inter', 'sans-serif'],
      },
    },
  },
  plugins: [],
};
