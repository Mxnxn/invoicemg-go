import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { nodePolyfills } from "vite-plugin-node-polyfills";

// Existing source tree uses JSX inside plain .js files (CRA convention) —
// tell esbuild to parse .js under src/ as JSX rather than renaming every file.
export default defineConfig({
    plugins: [
        react(),
        // Tailwind 4 powers the marketing landing page only. Its utilities are
        // prefixed (tw:) and its preflight is never imported - see
        // src/Landing/landing.css - so it cannot reach the Argon/reactstrap
        // admin views.
        tailwindcss(),
        // @react-pdf/renderer's deps (blob-stream etc.) reference Node core
        // modules/globals (`global`, `util`, `stream`) that CRA/webpack
        // polyfilled automatically but Vite doesn't.
        nodePolyfills(),
    ],
    esbuild: {
        loader: "jsx",
        include: /src\/.*\.jsx?$/,
        exclude: [],
    },
    optimizeDeps: {
        esbuildOptions: {
            loader: { ".js": "jsx" },
        },
    },
    server: {
        port: 3000,
        host: true,
        watch: {
            // Bind-mounted from the Windows host into the container, so native fs
            // events (inotify) don't fire on changes made outside the container —
            // fall back to polling so HMR actually picks up edits.
            usePolling: true,
            interval: 300,
        },
    },
    css: {
        preprocessorOptions: {
            scss: {
                // Vendored Bootstrap 4 / Argon Dashboard SCSS (src/assets/scss) still
                // backs the legacy reactstrap views and isn't hand-edited — silence the
                // Dart Sass deprecation warnings it triggers rather than rewriting it.
                silenceDeprecations: [
                    "legacy-js-api",
                    "import",
                    "if-function",
                    "duplicate-var-flags",
                    "global-builtin",
                    "color-functions",
                    "slash-div",
                    "function-units",
                ],
                quietDeps: true,
            },
        },
    },
    build: {
        outDir: "build",
        // @react-pdf/renderer pulls in fontkit, which ships CommonJS containing
        // require("unicode-trie") and require("./trie.json") interleaved with ESM. Rollup's
        // commonjs plugin skips mixed modules by default, so those calls survived into the
        // production bundle and threw "require is not defined" at runtime. Dev never showed
        // it because esbuild pre-bundles dependencies there.
        commonjsOptions: {
            transformMixedEsModules: true,
        },
    },
    test: {
        globals: true,
        environment: "jsdom",
        setupFiles: ["./vitest.setup.js"],
        exclude: ["**/node_modules/**", "**/.claude/**"],
    },
});
