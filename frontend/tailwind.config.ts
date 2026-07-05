import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        "bg-primary": "#0a0a0a",
        "bg-secondary": "#1a1a1a",
        "text-primary": "#ffffff",
        accent: "#18e2ff",
      },
    },
  },
  plugins: [],
};

export default config;
