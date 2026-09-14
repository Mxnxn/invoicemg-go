import { describe, expect, it, vi } from "vitest";

// The .ttf imports resolve to URLs under Vite; in the test environment they are stubbed so
// this suite can assert over the registry's shape without loading two megabytes of font.
vi.mock("../../Views/Invoice/font/OSR400.ttf", () => ({ default: "osr400" }));
vi.mock("../../Views/Invoice/font/OS700.ttf", () => ({ default: "os700" }));
vi.mock("../../Views/Invoice/font/OS400.ttf", () => ({ default: "os400" }));
vi.mock("../../Views/Invoice/font/GS400.ttf", () => ({ default: "gs400" }));
vi.mock("../../Views/Invoice/font/GS700.ttf", () => ({ default: "gs700" }));
vi.mock("./fonts/Lato-Regular.ttf", () => ({ default: "lato400" }));
vi.mock("./fonts/Lato-Bold.ttf", () => ({ default: "lato700" }));
vi.mock("./fonts/PTSerif-Regular.ttf", () => ({ default: "pt400" }));
vi.mock("./fonts/PTSerif-Bold.ttf", () => ({ default: "pt700" }));

const register = vi.fn();
vi.mock("@react-pdf/renderer", () => ({ Font: { register: (...a) => register(...a) } }));

import { FONTS, FONT_KEYS, DEFAULT_FONT_KEY, resolveFont, money, registerDocumentFont } from "./fonts";

describe("font registry", () => {
    it("every font has a family, a label and both weights", () => {
        FONTS.forEach((font) => {
            expect(font.label, `${font.key} label`).toBeTruthy();
            expect(font.family, `${font.key} family`).toBeTruthy();
            expect(font.regular, `${font.key} regular`).toBeTruthy();
            expect(font.bold, `${font.key} bold`).toBeTruthy();
        });
    });

    it("keys are unique and the default is one of them", () => {
        expect(new Set(FONT_KEYS).size).toBe(FONT_KEYS.length);
        expect(FONT_KEYS).toContain(DEFAULT_FONT_KEY);
    });

    // react-pdf 1.x has no fallback chain: a glyph the registered family lacks renders as
    // .notdef, not as the same glyph from another font. Open Sans has no U+20B9, which is
    // why this flag exists and why money() reads it.
    // Google Sans was removed as a document face; resolveFont falls any company still set to
    // it through to open-sans, which carries no rupee glyph - so their documents move from
    // "₹" to "Rs ", which money() handles rather than printing an empty box.
    it("records which families actually carry the rupee glyph", () => {
        const byKey = Object.fromEntries(FONTS.map((f) => [f.key, f.rupee]));
        expect(byKey["open-sans"]).toBe(false);
        expect(byKey.lato).toBe(true);
        expect(byKey["pt-serif"]).toBe(true);
    });

    // Changing the default would silently restyle every document already being issued.
    it("defaults to the family documents already use", () => {
        expect(DEFAULT_FONT_KEY).toBe("open-sans");
    });
});

describe("resolveFont", () => {
    it("passes a known key through", () => {
        expect(resolveFont("lato").key).toBe("lato");
    });

    it("falls back to the default for an unknown, empty or missing key", () => {
        expect(resolveFont("no-such-font").key).toBe(DEFAULT_FONT_KEY);
        expect(resolveFont("").key).toBe(DEFAULT_FONT_KEY);
        expect(resolveFont(undefined).key).toBe(DEFAULT_FONT_KEY);
    });
});

describe("money", () => {
    it("uses the rupee sign when the font has the glyph", () => {
        expect(money(1234.5, resolveFont("lato"))).toBe("₹1,234.50");
    });

    // Better a readable "Rs" than the empty box Open Sans renders for ₹ today.
    it("falls back to Rs when the font does not", () => {
        expect(money(1234.5, resolveFont("open-sans"))).toBe("Rs 1,234.50");
    });

    it("groups in the Indian system", () => {
        expect(money(1234567, resolveFont("lato"))).toBe("₹12,34,567.00");
    });

    it("keeps the sign in front of a negative amount", () => {
        expect(money(-500, resolveFont("lato"))).toBe("-₹500.00");
    });

    it("treats a missing or unparseable amount as zero", () => {
        expect(money(undefined, resolveFont("lato"))).toBe("₹0.00");
        expect(money(NaN, resolveFont("lato"))).toBe("₹0.00");
        expect(money("", resolveFont("lato"))).toBe("₹0.00");
    });

    it("accepts a numeric string", () => {
        expect(money("2500", resolveFont("lato"))).toBe("₹2,500.00");
    });
});

describe("registerDocumentFont", () => {
    it("registers both weights under the family name the styles use", () => {
        register.mockClear();
        const font = registerDocumentFont("lato");
        expect(register).toHaveBeenCalledWith({
            family: font.family,
            fonts: [
                { src: "lato400", fontWeight: 400 },
                { src: "lato700", fontWeight: 700 },
            ],
        });
        expect(font.key).toBe("lato");
    });

    it("returns the resolved font for an unknown key without throwing", () => {
        expect(registerDocumentFont("nonsense").key).toBe(DEFAULT_FONT_KEY);
    });
});
