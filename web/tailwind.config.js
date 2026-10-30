/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: '#090a0f',
        card: '#12141c',
        border: '#1f2433',
        primary: {
          50: '#f0fdf4',
          500: '#10b981',
          600: '#059669',
        },
        danger: {
          500: '#f43f5e',
          600: '#e11d48',
        },
      },
    },
  },
  plugins: [],
}
