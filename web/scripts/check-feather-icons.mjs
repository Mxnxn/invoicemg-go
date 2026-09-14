// Guards against importing an icon react-feather does not export.
//
// A wrong name is NOT a build error - it resolves to undefined, and the component only dies
// at render with "Element type is invalid". That is how `Landmark` and `Wallet` (lucide
// names, not feather ones) took out the Reports page after they were added.
//
// Lives here rather than in vitest because vite.config.js's nodePolyfills() stubs node:fs
// for the browser, so a filesystem scan cannot run under the vitest environment.
//
//   node scripts/check-feather-icons.mjs
import fs from "node:fs";
import path from "node:path";
import * as feather from "react-feather";

const SRC = path.resolve(process.cwd(), "src");

function walk(dir) {
    return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) return e.name === "node_modules" ? [] : walk(p);
        return /\.(js|jsx)$/.test(e.name) ? [p] : [];
    });
}

const missing = [];
let checked = 0;
for (const file of walk(SRC)) {
    const source = fs.readFileSync(file, "utf8");
    const re = /import\s*\{([^}]*)\}\s*from\s*["']react-feather["']/g;
    let m;
    while ((m = re.exec(source))) {
        for (const raw of m[1].split(",")) {
            const name = raw.trim().split(/\s+as\s+/)[0].trim();
            if (!name) continue;
            checked += 1;
            if (!feather[name]) missing.push(`${name}  ${path.relative(SRC, file)}`);
        }
    }
}

if (missing.length > 0) {
    console.error(`react-feather: ${missing.length} icon(s) do not exist:\n  ${missing.join("\n  ")}`);
    process.exit(1);
}
console.log(`react-feather: all ${checked} icon imports exist.`);
