// The six document designs, shared by invoices, quotations and client ledgers.
//
// A design used to be a whole React component per document type, which is why the Ledger tab
// offered one design while Invoice and Quotation offered four: adding a design meant writing
// it three times, so in practice it got written once. A design is now data - colour and size
// tokens, plus three structural choices - and each document type has a single renderer that
// implements those choices once (see Views/Invoice/Template/InvoiceDoc.jsx and its siblings).
// Adding a seventh design is an object in this file, not three new components.
//
// `header`, `table` and `totals` are a closed vocabulary. The renderers switch on them, so a
// value none of them implements renders nothing rather than failing - designs.test.js pins
// the allowed set for that reason.
//
//   header  plain  logo and document meta on one line, no fill
//           band   full-bleed accent band carrying the firm name and logo
//           split  bordered box: firm on the left, document meta on the right
//   table   ruled  hairline rule under each row, no outer border
//           boxed  full grid - outer border and a line between every row and column
//           zebra  no rules; alternating row fills
//   totals  plain  right-aligned stack, a rule above the grand total
//           bar    right-aligned stack, grand total in a filled accent bar
//           boxed  bordered block matching a `boxed` table
const design = (key, label, blurb, header, table, totals, tokens) => ({
    key,
    label,
    blurb,
    spec: {
        header,
        table,
        totals,
        tokens: {
            // Defaults chosen so a design only states what makes it different.
            accent: "#111111",
            ink: "#111111",
            muted: "#777777",
            rule: "#DDDDDD",
            fill: "#F1F1F1",
            pagePadding: 28,
            baseSize: 9,
            titleSize: 20,
            labelSize: 7.5,
            rowGap: 5,
            ...tokens,
        },
    },
});

export const DESIGNS = [
    // The four below are ports of the components that shipped before this file existed;
    // their tokens are lifted from those components so an already-issued look is preserved.
    design("classic", "Classic", "Plain letterhead, generous spacing, violet grand total.", "plain", "ruled", "plain", {
        accent: "#6C3CE9",
        ink: "#000000",
        muted: "#999999",
        rule: "#CCCCCC",
        pagePadding: 30,
        baseSize: 11,
        titleSize: 24,
        labelSize: 8,
        rowGap: 6,
    }),
    design("detailed", "Detailed", "Every column ruled and boxed - the dense, tax-office look.", "split", "boxed", "boxed", {
        accent: "#000000",
        ink: "#222222",
        muted: "#555555",
        rule: "#000000",
        fill: "#EEEEEE",
        pagePadding: 24,
        baseSize: 8.5,
        titleSize: 12,
        labelSize: 8,
        rowGap: 3,
    }),
    design("modern", "Modern", "Full-width teal header band and a filled total bar.", "band", "ruled", "bar", {
        accent: "#0F766E",
        ink: "#0F172A",
        muted: "#64748B",
        rule: "#E2E8F0",
        fill: "#D1FAE5",
        pagePadding: 32,
        baseSize: 9,
        titleSize: 16,
        labelSize: 7.5,
        rowGap: 6,
    }),
    design("compact", "Compact", "Small type and tight rows - the most lines to a page.", "plain", "zebra", "plain", {
        accent: "#111111",
        ink: "#111111",
        muted: "#777777",
        rule: "#E4E4E4",
        fill: "#F1F1F1",
        pagePadding: 24,
        baseSize: 7.5,
        titleSize: 12,
        labelSize: 6.5,
        rowGap: 3,
    }),

    // New with this change.
    design("editorial", "Editorial", "Wide margins, large quiet headings, hairline rules.", "split", "ruled", "plain", {
        accent: "#7C2D12",
        ink: "#1C1917",
        muted: "#78716C",
        rule: "#E7E5E4",
        fill: "#FAF9F7",
        pagePadding: 42,
        baseSize: 9.5,
        titleSize: 22,
        labelSize: 7,
        rowGap: 8,
    }),
    design("monoline", "Monoline", "Black and white only, uppercase micro-labels, no fills.", "plain", "ruled", "boxed", {
        accent: "#000000",
        ink: "#000000",
        muted: "#8A8A8A",
        rule: "#111111",
        fill: "#FFFFFF",
        pagePadding: 34,
        baseSize: 8.5,
        titleSize: 14,
        labelSize: 6.5,
        rowGap: 5,
    }),
];

export const DESIGN_KEYS = DESIGNS.map((d) => d.key);

const DEFAULT_KEY = "classic";

// Companies configured before the rename have "gst-detailed" stored against them. The name
// went because the set is shared with the ledger now, and a running balance statement has no
// per-line GST breakdown to be detailed about. scripts/migrate-design-keys.js rewrites the
// stored value, but a document has to render correctly whether or not that has run, so the
// old name is understood here too - permanently, since a restored backup can reintroduce it.
const LEGACY_KEYS = { "gst-detailed": "detailed" };

export const resolveDesignKey = (key) => {
    const candidate = LEGACY_KEYS[key] || key;
    return DESIGN_KEYS.includes(candidate) ? candidate : DEFAULT_KEY;
};

export const resolveDesign = (key) => DESIGNS.find((d) => d.key === resolveDesignKey(key));

// One size preference across every design.
//
// Each design carries its own base size, chosen for its own proportions - "compact" sets body
// text at 7.5pt and "bold" at 11. That is what makes them different designs, but it also means
// switching design silently resizes the whole document, and a firm that finds one template
// comfortable finds the next one too small.
//
// The scale multiplies rather than replaces: a design's internal proportions - title against
// body against label - survive, and the whole document moves together. Applied at resolve
// time, so every renderer gets scaled sizes without knowing this exists.
export const DOCUMENT_SCALES = [
    { id: "compact", label: "Compact", factor: 0.9 },
    { id: "normal", label: "Normal", factor: 1 },
    { id: "large", label: "Large", factor: 1.12 },
    { id: "xlarge", label: "Extra large", factor: 1.25 },
];

export const DEFAULT_DOCUMENT_SCALE = "normal";

export const scaleFactor = (id) =>
    (DOCUMENT_SCALES.find((s) => s.id === id) || DOCUMENT_SCALES.find((s) => s.id === DEFAULT_DOCUMENT_SCALE)).factor;

// The size tokens, and only those: colours and the structural choices are untouched. Rounded
// to a tenth of a point because @react-pdf lays out on fractional sizes and an unrounded
// multiply produces values like 9.450000000000001 in the styles.
const SIZE_TOKENS = ["baseSize", "titleSize", "labelSize"];

export const resolveDesignScaled = (key, scaleId) => {
    const design = resolveDesign(key);
    const factor = scaleFactor(scaleId);
    if (factor === 1) return design;
    const tokens = { ...design.spec.tokens };
    SIZE_TOKENS.forEach((t) => {
        if (typeof tokens[t] === "number") tokens[t] = Math.round(tokens[t] * factor * 10) / 10;
    });
    return { ...design, spec: { ...design.spec, tokens } };
};
