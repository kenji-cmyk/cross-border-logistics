/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: { fontFamily: { display: ['"Instrument Serif"', "serif"], sans: ["Inter", "sans-serif"] } } },
  plugins: [],
};
