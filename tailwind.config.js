export default {
  content: ['./frontend/index.html', './frontend/src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        border: 'hsl(var(--border))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        muted: 'hsl(var(--muted))',
        panel: 'hsl(var(--panel))',
        primary: 'hsl(var(--primary))',
        accent: 'hsl(var(--accent))',
        danger: 'hsl(var(--danger))'
      }
    }
  },
  plugins: []
};
