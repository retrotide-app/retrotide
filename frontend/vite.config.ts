import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

declare const process: { env: Record<string, string | undefined> };

const goServer = "http://127.0.0.1:4333";

export default defineConfig({
    define: {
        __APP_VERSION__: JSON.stringify(process.env.VITE_APP_VERSION ?? "dev"),
    },
    plugins: [react()],
    server: {
        port: 5174,
        strictPort: true,
        proxy: {
            "/api/v1/ws": {
                target: goServer,
                ws: true,
            },
            "/api": {
                target: goServer,
            },
            "/sitemap": {
                target: goServer,
            },
            "/uploads": {
                target: goServer,
            },
            "/robots.txt": {
                target: goServer,
            },
        },
    },
    build: {
        outDir: "../static",
        emptyOutDir: true,
        rolldownOptions: {
            output: {
                codeSplitting: {
                    groups: [
                        {
                            name: "vendor-react",
                            test: /[\\/]node_modules[\\/](react|react-dom|react-router|scheduler)[\\/]/,
                        },
                    ],
                },
            },
        },
    },
});
