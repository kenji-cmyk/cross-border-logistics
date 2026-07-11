/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: { extend: {
    colors: { canvas: "#F7F9FC", ink: "#0B1220", muted: "#667085", subtle: "#98A2B3", brand: "#2563EB", "brand-soft": "#EFF6FF", success: "#12B76A", "success-dark": "#067647", "success-soft": "#ECFDF3", warning: "#F79009", "warning-dark": "#B54708", "warning-soft": "#FFFAEB", danger: "#F04438", "danger-dark": "#B42318", "danger-soft": "#FEF3F2" },
    boxShadow: { soft: "0 12px 40px rgba(15,23,42,.06)", card: "0 18px 60px rgba(15,23,42,.08)", primary: "0 14px 40px rgba(11,18,32,.18)" },
    fontFamily: { display: ['"Instrument Serif"', "serif"], sans: ["Inter", "sans-serif"] },
  } }, plugins: [],
};
