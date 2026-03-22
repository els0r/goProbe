module.exports = {
  content: [
    './src/**/*.{js,ts,jsx,tsx,html}'
  ],
  theme: {
    extend: {
      fontSize: {
        // Named scale for data-dense UI — avoids scattered text-[Npx] arbitrary values
        'data':    ['12px', { lineHeight: '1.4' }],
        'data-sm': ['11px', { lineHeight: '1.35' }],
        'data-xs': ['10px', { lineHeight: '1.3' }],
      },
      colors: {
        surface: {
          DEFAULT: '#0f1115',
          100: '#1a1d23',
          200: '#242a32'
        },
        primary: {
          DEFAULT: '#3d8bff',
          50: '#e1f0ff',
          100: '#b3d6ff',
          200: '#84bbff',
          300: '#559fff',
          400: '#2684ff',
          500: '#0d6ce6',
          600: '#084fb4',
          700: '#053282',
          800: '#021451',
          900: '#000720'
        }
      }
    }
  },
  plugins: []
};
