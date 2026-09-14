import { describe, expect, it } from "vitest";
import { seriesVar, SERIES_SLOTS, foldSeries } from "./chartTheme";

describe("seriesVar", () => {
    it("maps a slot to its custom property", () => {
        expect(seriesVar(0)).toBe("var(--series-1)");
        expect(seriesVar(7)).toBe("var(--series-8)");
    });

    it("never generates a ninth hue - it wraps to the fixed order", () => {
        // A generated colour would break "colour follows the entity": the palette
        // is eight validated slots, and a 9th series folds instead.
        expect(SERIES_SLOTS).toBe(8);
        expect(seriesVar(8)).toBe("var(--series-1)");
    });
});

describe("foldSeries", () => {
    const items = [
        { name: "A", value: 10 },
        { name: "B", value: 9 },
        { name: "C", value: 8 },
        { name: "D", value: 7 },
        { name: "E", value: 6 },
        { name: "F", value: 5 },
        { name: "G", value: 4 },
        { name: "H", value: 3 },
        { name: "I", value: 2 },
        { name: "J", value: 1 },
    ];

    it("keeps everything when it fits", () => {
        const out = foldSeries(items.slice(0, 3), 8);
        expect(out.shown).toHaveLength(3);
        expect(out.other).toBeNull();
    });

    it("folds the tail into Other rather than inventing colours", () => {
        const out = foldSeries(items, 8);
        expect(out.shown).toHaveLength(7);
        expect(out.other).toEqual({ name: "Other", value: 3 + 2 + 1 });
    });

    it("copes with nothing", () => {
        expect(foldSeries([], 8)).toEqual({ shown: [], other: null });
        expect(foldSeries(null, 8)).toEqual({ shown: [], other: null });
    });
});
