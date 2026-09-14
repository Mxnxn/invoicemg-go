// User-chosen font and text size, applied as CSS variables on <html>.
//
// Stored in localStorage next to `theme` rather than on the User: it is a per-device reading
// preference, the same as the theme toggle, and someone on a small laptop should not have
// their phone re-sized to match.
//
// Only fonts the app actually ships are offered. A free-text font box would let someone pick a
// face the browser has to fall back from, which looks like a bug rather than a choice.
//
// Two sources, both bundled: the first group is loaded by index.html from Google Fonts, and the
// nine below it are declared in styles/uiFonts.css against the very .ttf files the PDF renderer
// already uses. That second group was added so a company could set its DOCUMENTS in something
// other than Open Sans; the files are in the bundle either way, so offering the same faces for
// the interface costs nothing but the @font-face rules - and a browser fetches one only when
// something is actually rendered in it.

export const FONT_OPTIONS = [
	{ id: "jetbrains", group: "Interface", label: "JetBrains Mono", stack: '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace' },
	{ id: "inter", group: "Interface", label: "Inter", stack: '"Inter", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "outfit", group: "Interface", label: "Outfit", stack: '"Outfit", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "google-sans", group: "Interface", label: "Google Sans", stack: '"Google Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "ibm-plex", group: "Interface", label: "IBM Plex Sans", stack: '"IBM Plex Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "open-sans", group: "Interface", label: "Open Sans", stack: '"Open Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "reddit-sans", group: "Interface", label: "Reddit Sans", stack: '"Reddit Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "system", group: "Interface", label: "System default", stack: 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif' },

	// The document families. Same faces, same files, as Configure > Templates offers for
	// invoices - so what is chosen here and what prints can be compared honestly.
	{ id: "poppins", group: "Document families", label: "Poppins", stack: '"Poppins", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "roboto", group: "Document families", label: "Roboto", stack: '"Roboto", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "sora", group: "Document families", label: "Sora", stack: '"Sora", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "josefin-sans", group: "Document families", label: "Josefin Sans", stack: '"Josefin Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "pliant", group: "Document families", label: "Pliant", stack: '"Pliant", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "geist", group: "Document families", label: "Geist", stack: '"Geist", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "archivo", group: "Document families", label: "Archivo", stack: '"Archivo", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "fira-sans", group: "Document families", label: "Fira Sans", stack: '"Fira Sans", system-ui, -apple-system, "Segoe UI", sans-serif' },
	{ id: "dm-mono", group: "Document families", label: "DM Mono", stack: '"DM Mono", ui-monospace, SFMono-Regular, Menlo, monospace' },
];

// Three independent scales rather than one: wanting bigger figures on a stat card is not
// the same as wanting a denser table, and the old single slider moved everything together.
export const SCALE_ROLES = [
	{ id: "heading", label: "Headings", hint: "Page titles, stat card figures", basePx: 23.5 },
	{ id: "body", label: "Body & labels", hint: "Form labels and sidebar", basePx: 14 },
	{ id: "tableHead", label: "Table headers", hint: "Column titles", basePx: 11 },
	{ id: "tableRow", label: "Table rows", hint: "Every data cell", basePx: 13 },
	{ id: "button", label: "Buttons", hint: "Any button, with or without an icon", basePx: 13 },
	// Cards are their own surface now - the job-ids on a board, the documents on a deck, the
	// days on the dashboard - and they are read at a different distance from a form label,
	// so they do not ride on Body.
	//
	// Split the way the table is split, and for the same reason: what a card is CALLED and
	// what a card SAYS are read differently. The number is found by scanning a deck; the money
	// and the products are read once you have found it. One slider for both meant making the
	// job-id legible across a workshop also inflated four chips nobody reads from there.
	{ id: "cardTitle", label: "Card titles", hint: "Job-id and document numbers", basePx: 13 },
	{ id: "card", label: "Card text", hint: "Party, date, money, products", basePx: 12.5 },
];

// A role covers several sizes - "headings" is a 31.5px stat figure, a 23.5px page title and a
// 16px brand mark - so it is stored as a multiplier, not one absolute size. The input is in px
// against the role's representative element (the one the preview shows), and everything else
// in that role moves proportionally. So the number typed is the number rendered for that
// element, which is what makes a px box honest rather than approximate.
const roleOf = (id) => SCALE_ROLES.find((r) => r.id === id) || SCALE_ROLES[0];

export const scaleToPx = (id, scale) => Math.round(roleOf(id).basePx * clampScale(scale) * 10) / 10;

export const pxToScale = (id, px) => {
	const n = Number(px);
	if (!Number.isFinite(n) || n <= 0) return 1;
	return clampScale(n / roleOf(id).basePx);
};

// What the box will accept, in px, so the field can advertise its own limits.
export const pxRange = (id) => ({
	min: Math.round(roleOf(id).basePx * SCALE_MIN * 10) / 10,
	max: Math.round(roleOf(id).basePx * SCALE_MAX * 10) / 10,
});

// The accent colour. One token pair (--xan-blue / --xan-blue-bg) drives every accent in the
// app - links, the active sidebar item, focus rings, badges, the primary button - so the
// whole thing follows from one choice. The name stays --xan-blue because hundreds of rules
// read it; renaming the variable would be a much larger change than repainting it.
//
// A fixed palette rather than a colour picker: these are checked against both themes and
// against --text-on-primary (white), where an arbitrary hex can land on an unreadable
// button without the person choosing it ever noticing.
export const ACCENT_OPTIONS = [
	{ id: "blue", label: "Blue", hex: "#3b82f6", hover: "#2563eb" },
	{ id: "indigo", label: "Indigo", hex: "#6366f1", hover: "#4f46e5" },
	{ id: "violet", label: "Violet", hex: "#8b5cf6", hover: "#7c3aed" },
	{ id: "teal", label: "Teal", hex: "#14b8a6", hover: "#0d9488" },
	{ id: "emerald", label: "Emerald", hex: "#10b981", hover: "#059669" },
	{ id: "amber", label: "Amber", hex: "#d97706", hover: "#b45309" },
	{ id: "rose", label: "Rose", hex: "#f43f5e", hover: "#e11d48" },
	{ id: "slate", label: "Slate", hex: "#475569", hover: "#334155" },
];

// The same idea in two tones. `base` is what the flat accents (text, borders, focus rings)
// use - a gradient cannot paint a 1px border or a letter - so a gradient stays coherent with
// the rest of the UI instead of only existing on buttons.
export const GRADIENT_OPTIONS = [
	{ id: "none", label: "None", base: "" },
	{ id: "ocean", label: "Ocean", from: "#3b82f6", to: "#06b6d4", base: "#3b82f6", hover: "#2563eb" },
	{ id: "grape", label: "Grape", from: "#8b5cf6", to: "#d946ef", base: "#8b5cf6", hover: "#7c3aed" },
	{ id: "sunset", label: "Sunset", from: "#f97316", to: "#f43f5e", base: "#f43f5e", hover: "#e11d48" },
	{ id: "forest", label: "Forest", from: "#10b981", to: "#14b8a6", base: "#10b981", hover: "#059669" },
	{ id: "dusk", label: "Dusk", from: "#6366f1", to: "#8b5cf6", base: "#6366f1", hover: "#4f46e5" },
];

export const accentOf = (id) => ACCENT_OPTIONS.find((a) => a.id === id) || ACCENT_OPTIONS[0];
export const gradientOf = (id) => GRADIENT_OPTIONS.find((g) => g.id === id) || GRADIENT_OPTIONS[0];

// #rrggbb -> "rgba(r, g, b, alpha)". The tinted backgrounds (--xan-blue-bg) are the accent at
// 10%, so they have to be derived rather than listed a second time and left to drift.
export const hexToRgba = (hex, alpha) => {
	const clean = String(hex || "").replace("#", "");
	if (clean.length !== 6) return `rgba(59, 130, 246, ${alpha})`;
	const n = parseInt(clean, 16);
	return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${alpha})`;
};

// The colours actually applied, once the gradient choice has had its say over the flat one.
export function resolveAccent(appearance = {}) {
	const gradient = gradientOf(appearance.gradient);
	if (gradient.id !== "none") {
		return {
			hex: gradient.base,
			hover: gradient.hover,
			image: `linear-gradient(135deg, ${gradient.from} 0%, ${gradient.to} 100%)`,
		};
	}
	const accent = accentOf(appearance.accent);
	return { hex: accent.hex, hover: accent.hover, image: "" };
}

export const DEFAULT_APPEARANCE = {
	// Only the default for someone who has never chosen: readAppearance returns a stored
	// font whenever there is one, so changing this cannot restyle anyone already using the
	// app. It is loaded by index.html, so it renders rather than falling back.
	font: "google-sans",
	scales: { heading: 1, body: 1, tableHead: 1, tableRow: 1, button: 1, cardTitle: 1, card: 1 },
	tracking: 0.1,
	accent: "blue",
	gradient: "none",
	rowsPerPage: 25,
};

// How many rows a table shows before it paginates.
//
// 0 is "All" rather than "none" - a table showing zero rows is not a setting anyone wants,
// and "show me everything" is a real preference on a short list or before a print. It is
// first because that is the behaviour every table had before this existed.
//
// A per-device preference like the rest of this file: the row count that suits a laptop is
// not the one that suits the 27" screen in the shop.
export const ROWS_PER_PAGE_OPTIONS = [
	{ value: 0, label: "All" },
	{ value: 10, label: "10" },
	{ value: 15, label: "15" },
	{ value: 25, label: "25" },
	{ value: 50, label: "50" },
	{ value: 100, label: "100" },
	{ value: 200, label: "200" },
];

export const clampRowsPerPage = (value) => {
	const n = Number(value);
	if (!Number.isFinite(n)) return DEFAULT_APPEARANCE.rowsPerPage;
	// Snap to an offered option rather than accepting any number: the value reaches this
	// from localStorage, which a previous release - or a person with devtools - can leave
	// anything at all in.
	return ROWS_PER_PAGE_OPTIONS.some((o) => o.value === n) ? n : DEFAULT_APPEARANCE.rowsPerPage;
};

// Letter spacing for table rows, in px. Rows are dense and mostly codes and figures - job
// numbers, GSTINs, amounts - which run together without a little tracking. How much is a
// matter of the font you picked and your eyes, so it is a setting rather than a constant.
// Beyond about 1px the columns start to feel unrelated to each other; below -0.5px the
// characters touch.
export const TRACKING_MIN = -0.5;
export const TRACKING_MAX = 1;

export const clampTracking = (value) => {
	const n = Number(value);
	if (!Number.isFinite(n)) return DEFAULT_APPEARANCE.tracking;
	return Math.min(TRACKING_MAX, Math.max(TRACKING_MIN, Math.round(n * 100) / 100));
};

// Bounds chosen so the shell still holds together: below 0.85 the table chrome collapses,
// above 1.3 the sidebar labels wrap.
export const SCALE_MIN = 0.85;
export const SCALE_MAX = 1.3;

const STORAGE_KEY = "appearance";

export const clampScale = (value) => {
	const n = Number(value);
	if (!Number.isFinite(n)) return DEFAULT_APPEARANCE.scale;
	// Four decimals, not two: the size boxes are in px and convert through this multiplier,
	// so coarse rounding meant typing 14 and rendering 13.97. The stored value is never read
	// by a human, so the precision costs nothing.
	return Math.min(SCALE_MAX, Math.max(SCALE_MIN, Math.round(n * 10000) / 10000));
};

// The options in the order they are offered, gathered under their group heading. Kept here
// rather than in the picker so the list has one source: adding a font is one line above.
export const FONT_GROUPS = FONT_OPTIONS.reduce((groups, font) => {
	const group = groups.find((g) => g.label === font.group);
	if (group) group.fonts.push(font);
	else groups.push({ label: font.group, fonts: [font] });
	return groups;
}, []);

export const fontStack = (id) => (FONT_OPTIONS.find((f) => f.id === id) || FONT_OPTIONS[0]).stack;

export function readAppearance(storage = window.localStorage) {
	try {
		const raw = storage.getItem(STORAGE_KEY);
		if (!raw) return { ...DEFAULT_APPEARANCE };
		const parsed = JSON.parse(raw);
		// `scale` was a single value before the roles split - carry it across so an existing
		// preference is honoured rather than silently reset to 100%.
		const legacy = parsed.scale === undefined ? null : clampScale(parsed.scale);
		const stored = { ...(parsed.scales || {}) };
		// `table` was one control before headers and rows were separated - carry it onto both
		// so a stored preference is honoured rather than reset.
		if (stored.table !== undefined) {
			if (stored.tableHead === undefined) stored.tableHead = stored.table;
			if (stored.tableRow === undefined) stored.tableRow = stored.table;
		}
		// Cards sized off the BODY scale until they got a control of their own. Inheriting it
		// is what makes this change invisible: someone who had set body to 105% keeps cards at
		// 105% rather than watching every card on every screen shrink to 100% overnight.
		if (stored.card === undefined && stored.body !== undefined) stored.card = stored.body;
		// And the title follows whatever the card was, for the one build where Cards was a
		// single control - same argument one step later.
		if (stored.cardTitle === undefined) {
			if (stored.card !== undefined) stored.cardTitle = stored.card;
			else if (stored.body !== undefined) stored.cardTitle = stored.body;
		}
		const scales = {};
		SCALE_ROLES.forEach(({ id }) => {
			scales[id] = clampScale(stored[id] !== undefined ? stored[id] : legacy !== null ? legacy : 1);
		});
		return {
			font: FONT_OPTIONS.some((f) => f.id === parsed.font) ? parsed.font : DEFAULT_APPEARANCE.font,
			scales,
			// Absent on anything stored before this setting existed - those users had the
			// fixed 0.1px, which is the default, so nothing shifts under them.
			tracking: parsed.tracking === undefined ? DEFAULT_APPEARANCE.tracking : clampTracking(parsed.tracking),
			// Unknown ids fall back to the defaults, so a palette entry removed in a later
			// release cannot leave someone stuck on a colour that no longer exists.
			accent: ACCENT_OPTIONS.some((a) => a.id === parsed.accent) ? parsed.accent : DEFAULT_APPEARANCE.accent,
			gradient: GRADIENT_OPTIONS.some((g) => g.id === parsed.gradient) ? parsed.gradient : DEFAULT_APPEARANCE.gradient,
			// Absent on anything stored before this setting existed. Those tables showed
			// every row, but defaulting them to "All" would leave the setting looking like
			// it does nothing until touched - so they get the same 25 a new install gets.
			rowsPerPage: parsed.rowsPerPage === undefined ? DEFAULT_APPEARANCE.rowsPerPage : clampRowsPerPage(parsed.rowsPerPage),
		};
	} catch (error) {
		// Corrupt or unreadable (private mode) - the defaults are always usable.
		return { ...DEFAULT_APPEARANCE };
	}
}

export function writeAppearance(next, storage = window.localStorage) {
	const scales = {};
	SCALE_ROLES.forEach(({ id }) => {
		scales[id] = clampScale((next.scales || {})[id]);
	});
	const value = {
		font: next.font,
		scales,
		tracking: clampTracking(next.tracking),
		accent: accentOf(next.accent).id,
		gradient: gradientOf(next.gradient).id,
		rowsPerPage: clampRowsPerPage(next.rowsPerPage),
	};
	try {
		storage.setItem(STORAGE_KEY, JSON.stringify(value));
	} catch (error) {
		// Not fatal - the setting just will not survive a reload.
	}
	return value;
}

// The variables every typography rule reads. --font-scale multiplies the fixed px sizes;
// the family vars are what the shell, the tables and the legacy views resolve.
//
// --font-mono is switched too, so table cells follow the choice rather than staying pinned
// to JetBrains Mono. Columns of figures still line up: .cell-mono declares
// font-variant-numeric: tabular-nums, which is what actually does that work - a monospace
// face was never the thing keeping them aligned.
export function applyAppearance(appearance, root = document.documentElement) {
	const merged = { ...DEFAULT_APPEARANCE, ...(appearance || {}) };
	const stack = fontStack(merged.font);
	SCALE_ROLES.forEach(({ id }) => {
		const value = clampScale((merged.scales || {})[id]);
		// heading -> --font-scale-heading, tableHead -> --font-scale-table-head
		const cssName = id.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);
		root.style.setProperty(`--font-scale-${cssName}`, String(value));
		// Buttons size from an absolute variable rather than a multiplier, because
		// .shell-btn multiplies it by the body scale as well - see shell.css.
		if (id === "button") root.style.setProperty("--button-font-size", `${scaleToPx("button", value)}px`);
	});
	root.style.setProperty("--font-family", stack);
	root.style.setProperty("--font-outfit", stack);
	root.style.setProperty("--font-mono", stack);
	root.style.setProperty("--letter-spacing-table", `${clampTracking(merged.tracking)}px`);

	// One choice, every accent. --xan-blue and its 10% tint are what the shell, the tables and
	// the dark-mode overrides for the legacy Argon primitives all read; --bg-primary is the
	// primary button. Setting them on <html> beats both the :root defaults and the
	// [data-theme="dark"] block, so the accent survives a theme switch.
	const accent = resolveAccent(merged);
	root.style.setProperty("--xan-blue", accent.hex);
	root.style.setProperty("--xan-blue-bg", hexToRgba(accent.hex, 0.1));
	root.style.setProperty("--bg-primary", accent.hex);
	root.style.setProperty("--bg-primary-hover", accent.hover);
	// Only buttons and other filled surfaces can carry a gradient; everything flat keeps the
	// base colour above. Removed rather than set to "none" so the CSS fallback chain works.
	if (accent.image) root.style.setProperty("--accent-gradient", accent.image);
	else root.style.removeProperty("--accent-gradient");
}
