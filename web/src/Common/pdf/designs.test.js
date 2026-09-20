import { describe, expect, it } from "vitest";
import { DESIGNS, DESIGN_KEYS, DOCUMENT_SIZES, resolveDesign, resolveDesignKey, resolveDesignScaled } from "./designs";

// The design set is shared by invoice, quotation and ledger - "same names in all three tabs"
// is the whole point of it, so the invariants that keep the three lists identical are worth
// asserting rather than trusting.
describe("design registry", () => {
    it("offers six designs", () => {
        expect(DESIGNS).toHaveLength(6);
    });

    it("keys are unique", () => {
        expect(new Set(DESIGN_KEYS).size).toBe(DESIGN_KEYS.length);
    });

    it("every design carries a label, a blurb and a full spec", () => {
        DESIGNS.forEach((design) => {
            expect(design.label, `${design.key} label`).toBeTruthy();
            expect(design.blurb, `${design.key} blurb`).toBeTruthy();
            expect(design.spec.tokens.accent, `${design.key} accent`).toMatch(/^#[0-9A-Fa-f]{6}$/);
            expect(design.spec.tokens.baseSize, `${design.key} baseSize`).toBeGreaterThan(0);
        });
    });

    // The renderers switch on these three keys. A design naming a variant no renderer
    // implements would fall through to a blank document rather than fail loudly, so the
    // vocabulary is closed and checked here.
    it("only uses structural variants the renderers implement", () => {
        DESIGNS.forEach(({ key, spec }) => {
            expect(["plain", "band", "split"], `${key} header`).toContain(spec.header);
            expect(["ruled", "boxed", "zebra"], `${key} table`).toContain(spec.table);
            expect(["plain", "bar", "boxed"], `${key} totals`).toContain(spec.totals);
        });
    });

    it("keeps the four keys that companies already have stored", () => {
        expect(DESIGN_KEYS).toEqual(expect.arrayContaining(["classic", "modern", "compact", "detailed"]));
    });
});

describe("resolveDesignKey", () => {
    it("passes a known key through", () => {
        expect(resolveDesignKey("modern")).toBe("modern");
    });

    // Companies configured before the rename still have "gst-detailed" in Mongo. The
    // migration script rewrites them, but a document must render correctly whether or not
    // it has run yet - so the fallback lives in the resolver, not only in the migration.
    it("maps the legacy gst-detailed key onto detailed", () => {
        expect(resolveDesignKey("gst-detailed")).toBe("detailed");
    });

    it("falls back to classic for an unknown, empty or missing key", () => {
        expect(resolveDesignKey("no-such-design")).toBe("classic");
        expect(resolveDesignKey("")).toBe("classic");
        expect(resolveDesignKey(undefined)).toBe("classic");
        expect(resolveDesignKey(null)).toBe("classic");
    });
});

describe("resolveDesign", () => {
    it("returns the whole design for a key", () => {
        expect(resolveDesign("compact").key).toBe("compact");
    });

    it("returns classic's design for a legacy or unknown key", () => {
        expect(resolveDesign("gst-detailed").key).toBe("detailed");
        expect(resolveDesign("whatever").key).toBe("classic");
    });
});

// One text size across every design. Each design sets its own base size for its own
// proportions, so changing design used to resize the whole document without anyone asking.
describe("document text size", () => {
    it("scales a design up without changing its proportions", () => {
        const normal = resolveDesign("classic");
        const large = resolveDesignScaled("classic", "large");

        expect(large.spec.tokens.baseSize).toBeGreaterThan(normal.spec.tokens.baseSize);
        expect(large.spec.tokens.titleSize).toBeGreaterThan(normal.spec.tokens.titleSize);
        // Title still leads body by the same ratio - that ratio IS the design.
        const ratio = (d) => d.spec.tokens.titleSize / d.spec.tokens.baseSize;
        expect(Math.abs(ratio(large) - ratio(normal))).toBeLessThan(0.02);
    });

    it("leaves colour and structure alone", () => {
        const normal = resolveDesign("classic");
        const large = resolveDesignScaled("classic", "large");
        expect(large.spec.header).toBe(normal.spec.header);
        expect(large.spec.table).toBe(normal.spec.table);
        expect(large.spec.tokens.accent).toBe(normal.spec.tokens.accent);
    });

    it("treats normal as the identity and an unknown id as the default", () => {
        const normal = resolveDesign("classic");
        expect(resolveDesignScaled("classic", "normal")).toBe(normal);
        expect(resolveDesignScaled("classic", "nonsense")).toEqual(normal);
        expect(resolveDesignScaled("classic", undefined)).toEqual(normal);
    });

    // @react-pdf lays out on fractional sizes, and an unrounded multiply puts values like
    // 9.450000000000001 into the styles.
    it("rounds every scaled size to a tenth of a point", () => {
        DESIGN_KEYS.forEach((key) =>
            DOCUMENT_SIZES.forEach((size) => {
                const t = resolveDesignScaled(key, size).spec.tokens;
                ["baseSize", "titleSize", "labelSize"].forEach((token) => {
                    expect(Math.round(t[token] * 10) / 10).toBe(t[token]);
                });
            })
        );
    });

    // A numeric point size sets the body text to exactly that size (bar the tenth-of-a-point
    // rounding), whatever base the design itself carries - that is what "8-18pt" means.
    it("brings any design's body text to the chosen point size", () => {
        DESIGN_KEYS.forEach((key) => {
            [8, 11, 14, 18].forEach((size) => {
                const base = resolveDesignScaled(key, size).spec.tokens.baseSize;
                expect(Math.abs(base - size)).toBeLessThanOrEqual(0.05);
            });
        });
    });

    // Values saved before the switch to point sizes still resolve to what they meant then.
    it("still honours the legacy label scales", () => {
        const normal = resolveDesign("classic");
        expect(resolveDesignScaled("classic", "large").spec.tokens.baseSize).toBe(
            Math.round(normal.spec.tokens.baseSize * 1.12 * 10) / 10
        );
    });
});
