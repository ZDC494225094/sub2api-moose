import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";

import { parseChangelog } from "./src/lib/release";

const webDir = dirname(fileURLToPath(import.meta.url));
const localVersion = readFileSync(resolve(webDir, "../VERSION"), "utf8").trim() || "dev";
const localChangelog = readFileSync(resolve(webDir, "../CHANGELOG.md"), "utf8");

// Expose /plugins/index.json with local plugin files from public/plugins.
// The frontend can discover and list them when enabled; development reads the directory live, while builds emit a static registry.
function localPluginsManifest(): Plugin {
    const pluginsDir = resolve(webDir, "public/plugins");
    const listLocalPlugins = () => {
        try {
            return readdirSync(pluginsDir)
                .filter((file) => file.endsWith(".js"))
                .sort()
                .map((file) => `/plugins/${file}`);
        } catch {
            return [];
        }
    };
    return {
        name: "local-plugins-manifest",
        configureServer(server) {
            server.middlewares.use("/plugins/index.json", (_req, res) => {
                res.setHeader("Content-Type", "application/json");
                res.end(JSON.stringify(listLocalPlugins()));
            });
        },
        generateBundle() {
            this.emitFile({ type: "asset", fileName: "plugins/index.json", source: JSON.stringify(listLocalPlugins()) });
        },
    };
}

// Keep ONNX runtime modules on this origin, including deployments below /canvas/.
function depthRuntimeAssets(): Plugin {
    const runtimeDir = resolve(webDir, "node_modules/@huggingface/transformers/dist");
    const files = ["ort-wasm-simd-threaded.jsep.mjs", "ort-wasm-simd-threaded.jsep.wasm"];
    return {
        name: "depth-runtime-assets",
        configureServer(server) {
            server.middlewares.use((req, res, next) => {
                const file = files.find((name) => req.url?.split("?")[0].endsWith(`/depth-runtime/${name}`));
                if (!file) return next();
                res.setHeader("Content-Type", file.endsWith(".wasm") ? "application/wasm" : "text/javascript");
                res.end(readFileSync(resolve(runtimeDir, file)));
            });
        },
        generateBundle() {
            for (const file of files) this.emitFile({ type: "asset", fileName: `depth-runtime/${file}`, source: readFileSync(resolve(runtimeDir, file)) });
        },
    };
}

export default defineConfig({
    base: process.env.VITE_BASE || "/",
    plugins: [react(), localPluginsManifest(), depthRuntimeAssets()],
    worker: { format: "es" },
    resolve: {
        alias: {
            "@": resolve(webDir, "src"),
        },
    },
    define: {
        __APP_VERSION__: JSON.stringify(localVersion),
        __APP_RELEASES__: JSON.stringify(parseChangelog(localChangelog)),
    },
});
