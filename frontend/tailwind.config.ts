import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: {
          0: "#0a0a0c",
          1: "#111114",
          2: "#18181b",
          3: "#1f1f23",
        },
        line: {
          DEFAULT: "#26262b",
          strong: "#35353b",
        },
        content: {
          DEFAULT: "#ededef",
          secondary: "#8b8b96",
          muted: "#5c5c66",
        },
        accent: {
          DEFAULT: "#6366f1",
          hover: "#818cf8",
          dim: "rgba(99,102,241,0.10)",
        },
        positive: {
          DEFAULT: "#22c55e",
          dim: "rgba(34,197,94,0.10)",
        },
        negative: {
          DEFAULT: "#ef4444",
          dim: "rgba(239,68,68,0.10)",
        },
      },
      fontFamily: {
        sans: [
          "Inter",
          "-apple-system",
          "BlinkMacSystemFont",
          "system-ui",
          "sans-serif",
        ],
      },
      fontSize: {
        "2xs": ["0.6875rem", { lineHeight: "1rem" }],
      },
    },
  },
  plugins: [],
} satisfies Config;
