import { describe, it, expect } from "vitest";
import { tickBudget, compactMoney, valueAxisTicks, categoryAxisTicks } from "./axisTicks";

describe("tickBudget", () => {
    it("scales with the axis length", () => {
        expect(tickBudget(150)).toBe(3);
        expect(tickBudget(220)).toBe(5);
        expect(tickBudget(300)).toBe(7);
    });

    // Never fewer than three: a floor, a ceiling and something in between is the minimum that
    // still lets you read a magnitude off the axis.
    it("never drops below three", () => {
        expect(tickBudget(40)).toBe(3);
        expect(tickBudget(1)).toBe(3);
    });

    it("caps at eight so a tall chart isn't a ruler", () => {
        expect(tickBudget(2000)).toBe(8);
    });

    it("handles missing or nonsense input", () => {
        expect(tickBudget(0)).toBe(3);
        expect(tickBudget(-100)).toBe(3);
        expect(tickBudget(undefined)).toBe(3);
        expect(tickBudget("abc")).toBe(3);
    });
});

describe("compactMoney", () => {
    it("leaves small amounts alone", () => {
        expect(compactMoney(950)).toBe("₹950");
    });

    it("abbreviates thousands, lakhs and crores", () => {
        expect(compactMoney(25576)).toBe("₹25.6k");
        expect(compactMoney(250000)).toBe("₹2.50L");
        expect(compactMoney(35000000)).toBe("₹3.50Cr");
    });

    // A negative net cashflow is a real value on this axis.
    it("keeps the sign outside the symbol", () => {
        expect(compactMoney(-4500)).toBe("-₹4.5k");
    });

    it("treats junk as zero rather than NaN", () => {
        expect(compactMoney(undefined)).toBe("₹0");
        expect(compactMoney(null)).toBe("₹0");
    });
});

describe("valueAxisTicks", () => {
    it("carries the budget and starts at zero", () => {
        const ticks = valueAxisTicks({ lengthPx: 150, fontColor: "#fff" });
        expect(ticks.maxTicksLimit).toBe(3);
        expect(ticks.beginAtZero).toBe(true);
        expect(ticks.autoSkip).toBe(true);
        expect(ticks.fontColor).toBe("#fff");
    });

    it("formats as compact money by default", () => {
        expect(valueAxisTicks({ lengthPx: 200 }).callback(25576)).toBe("₹25.6k");
    });

    it("accepts a custom formatter for non-money axes", () => {
        const ticks = valueAxisTicks({ lengthPx: 200, format: (v) => `${v}d` });
        expect(ticks.callback(12)).toBe("12d");
    });
});

describe("categoryAxisTicks", () => {
    // Skipping here would silently drop a client from its own row.
    it("never skips labels", () => {
        expect(categoryAxisTicks({ fontColor: "#000" }).autoSkip).toBe(false);
    });
});
