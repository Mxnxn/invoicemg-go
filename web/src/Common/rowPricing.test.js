import { describe, it, expect } from "vitest";
import { hasDimensions, dimensionFactor } from "./rowPricing";

describe("hasDimensions", () => {
    // The point of the flag: absent means by-dimension, so every row written before it
    // existed keeps its meaning with no migration.
    it("reads an absent flag as by-dimension", () => {
        expect(hasDimensions({})).toBe(true);
        expect(hasDimensions({ hasDimensions: undefined })).toBe(true);
    });

    it("honours an explicit flag", () => {
        expect(hasDimensions({ hasDimensions: true })).toBe(true);
        expect(hasDimensions({ hasDimensions: false })).toBe(false);
    });

    it("does not throw on a missing row", () => {
        expect(hasDimensions(null)).toBe(true);
        expect(hasDimensions(undefined)).toBe(true);
    });
});

describe("dimensionFactor", () => {
    it("multiplies the sides of a by-dimension row", () => {
        expect(dimensionFactor({ length: "2", width: "3" })).toBe(6);
        expect(dimensionFactor({ length: 2, width: 3 })).toBe(6);
        expect(dimensionFactor({ length: "2.5", width: "0.5" })).toBe(1.25);
    });

    it("keeps a genuine 1x1 distinct from having no dimensions", () => {
        expect(dimensionFactor({ length: "1", width: "1" })).toBe(1);
        expect(dimensionFactor({ hasDimensions: false, length: "1", width: "1" })).toBe(1);
        // Same factor, different meaning - which is exactly why the flag is stored and not
        // inferred from the values.
        expect(hasDimensions({ length: "1", width: "1" })).toBe(true);
        expect(hasDimensions({ hasDimensions: false, length: "1", width: "1" })).toBe(false);
    });

    it("ignores stale dimensions on a by-quantity row", () => {
        expect(dimensionFactor({ hasDimensions: false, length: "9", width: "9" })).toBe(1);
        expect(dimensionFactor({ hasDimensions: false })).toBe(1);
    });

    it("treats a blank side as zero by default, for job and quotation rows", () => {
        expect(dimensionFactor({ length: "", width: "3" })).toBe(0);
        expect(dimensionFactor({ length: "abc", width: "3" })).toBe(0);
        expect(dimensionFactor({})).toBe(0);
    });

    it("treats a blank side as one when asked, so historical entries keep their price", () => {
        expect(dimensionFactor({ length: "", width: "" }, 1)).toBe(1);
        expect(dimensionFactor({ length: "", width: "3" }, 1)).toBe(3);
        expect(dimensionFactor({ length: "abc", width: "3" }, 1)).toBe(3);
    });
});

describe("the formula these serve", () => {
    const price = (row, blank) => Number(row.qty) * dimensionFactor(row, blank) * Number(row.rate);

    it("prices both kinds through one expression", () => {
        expect(price({ qty: 4, rate: 120, length: "2", width: "3" })).toBe(2880);
        expect(price({ qty: 1, rate: 2500, hasDimensions: false })).toBe(2500);
        expect(price({ qty: 3, rate: 100, hasDimensions: false, length: "9", width: "9" })).toBe(300);
    });
});
