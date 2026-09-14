#!/usr/bin/env node
/**
 * Ask both services the same question and prove the answers are identical.
 *
 * This is the only thing that makes moving 209 routes tractable. A route is not "done" when it
 * is implemented - it is done when Node and Go answer identically for real sessions against
 * real data, because the failure mode of this migration is not a crash, it is a number that is
 * quietly different on one screen.
 *
 *   node scripts/parity.js --token <SESSION-TOKEN> [--route "post /sheet/only"] [--tab <TAB-ID>]
 *
 * With no --route it runs every route listed as verified in ROUTES.md plus anything in
 * ROUTES.parity.json, so it doubles as a regression suite: re-run it after any change and a
 * route that has drifted says so.
 *
 * Comparison rules, and why each exists:
 *
 *   - Object KEY ORDER is ignored. JSON objects are unordered; Go emits struct fields in
 *     declaration order and JavaScript in insertion order, and no client can tell.
 *   - ARRAY order is NOT ignored. These lists are sorted deliberately - newest day first,
 *     oldest open job first - so a different order is a real difference.
 *   - `undefined` and a missing key are the same thing; `null` is not. Node distinguishes
 *     them on purpose in places (a day with no sheet sends _id: null).
 *   - Numbers compare by value, so 0 and -0 and 1 and 1.0 agree.
 */

const http = require("http");

const NODE_PORT = Number(process.env.NODE_PORT || 5001);
const GO_PORT = Number(process.env.GO_PORT || 5002);

function arg(name, fallback) {
    const i = process.argv.indexOf(`--${name}`);
    return i === -1 ? fallback : process.argv[i + 1];
}

const TOKEN = arg("token");
const TAB = arg("tab", "");
const ONLY = arg("route");

if (!TOKEN) {
    console.error("A session token is required: --token <SESSION-TOKEN>");
    console.error("Mint a throwaway one against the dev database rather than using your own.");
    process.exit(2);
}

function request(port, method, path, body) {
    return new Promise((resolve, reject) => {
        const headers = { "SESSION-TOKEN": TOKEN };
        if (TAB) headers["TAB-ID"] = TAB;
        let payload;
        if (body && Object.keys(body).length) {
            // The frontend posts FormData, so parity has to be tested the way the client
            // actually calls - a JSON body would exercise a path nothing uses.
            const boundary = "----parity" + Date.now();
            payload = Buffer.from(
                Object.entries(body)
                    .map(([k, v]) => `--${boundary}\r\nContent-Disposition: form-data; name="${k}"\r\n\r\n${v}\r\n`)
                    .join("") + `--${boundary}--\r\n`
            );
            headers["Content-Type"] = `multipart/form-data; boundary=${boundary}`;
            headers["Content-Length"] = payload.length;
        }
        const started = process.hrtime.bigint();
        const req = http.request({ host: "127.0.0.1", port, path, method, headers }, (res) => {
            const chunks = [];
            res.on("data", (c) => chunks.push(c));
            res.on("end", () => {
                const ms = Number(process.hrtime.bigint() - started) / 1e6;
                const raw = Buffer.concat(chunks).toString();
                try {
                    resolve({ status: res.statusCode, body: JSON.parse(raw), ms });
                } catch (e) {
                    resolve({ status: res.statusCode, body: { __unparseable: raw.slice(0, 400) }, ms });
                }
            });
        });
        req.on("error", reject);
        if (payload) req.write(payload);
        req.end();
    });
}

// A stable rendering of a value: objects sorted by key, arrays left alone. Comparing these
// strings is what makes key order irrelevant and element order significant.
function canon(v) {
    if (v === null) return "null";
    if (Array.isArray(v)) return "[" + v.map(canon).join(",") + "]";
    if (typeof v === "object") {
        return (
            "{" +
            Object.keys(v)
                .filter((k) => v[k] !== undefined)
                .sort()
                .map((k) => JSON.stringify(k) + ":" + canon(v[k]))
                .join(",") +
            "}"
        );
    }
    if (typeof v === "number") return String(v === 0 ? 0 : v); // fold -0 into 0
    return JSON.stringify(v);
}

// Walks both sides together and reports the first few real differences with their paths,
// because "not identical" on a 200-row payload is not an actionable message.
function differences(a, b, path = "", found = []) {
    if (found.length >= 8) return found;
    const ta = a === null ? "null" : Array.isArray(a) ? "array" : typeof a;
    const tb = b === null ? "null" : Array.isArray(b) ? "array" : typeof b;

    if (ta !== tb) {
        found.push({ path: path || "(root)", node: `${ta} ${JSON.stringify(a)?.slice(0, 60)}`, go: `${tb} ${JSON.stringify(b)?.slice(0, 60)}` });
        return found;
    }
    if (ta === "array") {
        if (a.length !== b.length) {
            found.push({ path: path || "(root)", node: `${a.length} items`, go: `${b.length} items` });
        }
        for (let i = 0; i < Math.min(a.length, b.length); i++) differences(a[i], b[i], `${path}[${i}]`, found);
        return found;
    }
    if (ta === "object") {
        for (const k of new Set([...Object.keys(a), ...Object.keys(b)])) {
            if (!(k in a)) found.push({ path: `${path}.${k}`, node: "(missing)", go: JSON.stringify(b[k])?.slice(0, 60) });
            else if (!(k in b)) found.push({ path: `${path}.${k}`, node: JSON.stringify(a[k])?.slice(0, 60), go: "(missing)" });
            else differences(a[k], b[k], `${path}.${k}`, found);
        }
        return found;
    }
    if (canon(a) !== canon(b)) {
        found.push({ path: path || "(root)", node: JSON.stringify(a), go: JSON.stringify(b) });
    }
    return found;
}

function loadCases() {
    const fs = require("fs");
    const path = require("path");

    // Bodies a route needs, when it needs one. Kept beside the inventory rather than in here,
    // so adding a route to the suite is editing data, not code.
    let bodies = {};
    const bodiesPath = path.join(__dirname, "..", "ROUTES.parity.json");
    if (fs.existsSync(bodiesPath)) bodies = JSON.parse(fs.readFileSync(bodiesPath, "utf8"));

    if (ONLY) return [{ key: ONLY, body: bodies[ONLY] || {} }];

    // Everything ticked in ROUTES.md - the inventory is the single source of what is claimed
    // done, so this cannot drift from it.
    const md = fs.readFileSync(path.join(__dirname, "..", "ROUTES.md"), "utf8");
    const cases = [];
    for (const line of md.split("\n")) {
        const m = line.match(/^- \[x\] `([A-Z]+) (\/\S+)`/);
        if (m) {
            const key = `${m[1].toLowerCase()} ${m[2]}`;
            cases.push({ key, body: bodies[key] || {} });
        }
    }
    return cases;
}

(async () => {
    const cases = loadCases();
    if (!cases.length) {
        console.error("Nothing to check. Tick a route in ROUTES.md or pass --route.");
        process.exit(2);
    }

    let failed = 0;
    for (const { key, body } of cases) {
        const [method, routePath] = key.split(" ");
        let node, go;
        try {
            [node, go] = await Promise.all([
                request(NODE_PORT, method.toUpperCase(), routePath, body),
                request(GO_PORT, method.toUpperCase(), routePath, body),
            ]);
        } catch (e) {
            console.log(`FAIL  ${key}\n      could not reach both services: ${e.message}`);
            failed++;
            continue;
        }

        const diffs = differences(node.body, go.body);
        const speed = `node ${node.ms.toFixed(1)}ms  go ${go.ms.toFixed(1)}ms`;

        if (node.status !== go.status) {
            console.log(`FAIL  ${key}\n      HTTP status differs: node ${node.status}, go ${go.status}`);
            failed++;
        } else if (diffs.length) {
            console.log(`FAIL  ${key}   (${speed})`);
            for (const d of diffs) console.log(`      ${d.path}\n        node: ${d.node}\n        go:   ${d.go}`);
            failed++;
        } else {
            console.log(`ok    ${key}   (${speed})`);
        }
    }

    console.log(`\n${cases.length - failed} of ${cases.length} identical`);
    process.exit(failed ? 1 : 0);
})();
