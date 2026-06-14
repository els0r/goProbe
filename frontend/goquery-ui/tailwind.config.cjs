module.exports = {
  content: [
    './src/**/*.{js,ts,jsx,tsx,html}'
  ],
  theme: {
    // gq shape language: 2px corner radius for rectangular elements
    borderRadius: {
      none: '0',
      sm: '2px',
      DEFAULT: '2px',
      md: '2px',
      lg: '2px',
      xl: '2px',
      '2xl': '2px',
      '3xl': '2px',
      full: '9999px',
    },
    extend: {
      fontFamily: {
        sans: ['FiraGO', 'Helvetica', 'sans-serif'],
      },
      fontSize: {
        // Named scale for data-dense UI — avoids scattered text-[Npx] arbitrary values
        'data':    ['12px', { lineHeight: '1.4' }],
        'data-sm': ['11px', { lineHeight: '1.35' }],
        'data-xs': ['10px', { lineHeight: '1.3' }],
      },
      colors: {
        // Surfaces - use gq-aligned tokens via rgb(var(...) / <alpha-value>)
        surface: {
          DEFAULT: 'rgb(var(--surface-page) / <alpha-value>)',
          100: 'rgb(var(--surface-100) / <alpha-value>)',
          200: 'rgb(var(--surface-200) / <alpha-value>)',
          300: 'rgb(var(--surface-300) / <alpha-value>)'
        },
        // Gray text tiers mapped to gq neutrals
        gray: {
          100: 'rgb(var(--gray-100) / <alpha-value>)',
          200: 'rgb(var(--gray-200) / <alpha-value>)',
          300: 'rgb(var(--gray-300) / <alpha-value>)',
          400: 'rgb(var(--gray-400) / <alpha-value>)',
          500: 'rgb(var(--gray-500) / <alpha-value>)'
        },
        // gq red ramp for error/danger states with alpha support
        red: {
          200: 'rgb(var(--red-200) / <alpha-value>)',
          300: 'rgb(var(--red-300) / <alpha-value>)',
          400: 'rgb(var(--red-400) / <alpha-value>)',
          500: 'rgb(var(--red-500) / <alpha-value>)'
        },
        // Primary scale converted to token-based format
        primary: {
          DEFAULT: 'rgb(var(--primary-DEFAULT) / <alpha-value>)',
          50: 'rgb(var(--primary-50) / <alpha-value>)',
          100: 'rgb(var(--primary-100) / <alpha-value>)',
          200: 'rgb(var(--primary-200) / <alpha-value>)',
          300: 'rgb(var(--primary-300) / <alpha-value>)',
          400: 'rgb(var(--primary-400) / <alpha-value>)',
          500: 'rgb(var(--primary-500) / <alpha-value>)',
          600: 'rgb(var(--primary-600) / <alpha-value>)',
          700: 'rgb(var(--primary-700) / <alpha-value>)',
          800: 'rgb(var(--primary-800) / <alpha-value>)',
          900: 'rgb(var(--primary-900) / <alpha-value>)'
        },
        // Status colors
        danger: 'rgb(var(--danger) / <alpha-value>)',
        success: 'rgb(var(--success) / <alpha-value>)',
        warning: 'rgb(var(--warning) / <alpha-value>)',
        // Semantic line/hairline tokens - alpha baked in, reference directly
        'line-soft': 'var(--line-soft)',
        'line': 'var(--line)',
        'line-strong': 'var(--line-strong)',
        // Accent foreground + text on colored fills
        accent: 'rgb(var(--accent) / <alpha-value>)',
        'on-accent': 'rgb(var(--on-accent) / <alpha-value>)',
        // Modal/overlay scrim - alpha baked in
        scrim: 'var(--scrim)'
      }
    }
  },
  plugins: []
};
