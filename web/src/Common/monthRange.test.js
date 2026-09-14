import { describe, it, expect } from "vitest";
import { monthOf, isMonthRangeActive, inMonthRange, filterByMonthRange, EMPTY_MONTH_RANGE } from "./monthRange";

describe("monthOf", () => {
    it("reduces a date to its YYYY-MM prefix", () => {
        expect(monthOf("2026-08-17")).toBe("2026-08");
    });

    it("is empty for a missing date", () => {
        expect(monthOf("")).toBe("");
        expect(monthOf(null)).toBe("");
        expect(monthOf(undefined)).toBe("");
    });
});

describe("isMonthRangeActive", () => {
    it("is false for an empty range", () => {
        expect(isMonthRangeActive(EMPTY_MONTH_RANGE)).toBe(false);
        expect(isMonthRangeActive(undefined)).toBe(false);
    });

    it("is true with either bound set", () => {
        expect(isMonthRangeActive({ from: "2026-01", to: "" })).toBe(true);
        expect(isMonthRangeActive({ from: "", to: "2026-01" })).toBe(true);
    });
});

describe("inMonthRange", () => {
    it("passes everything when no bound is set", () => {
        expect(inMonthRange("2026-08-17", EMPTY_MONTH_RANGE)).toBe(true);
    });

    it("includes both bounds", () => {
        const range = { from: "2026-06", to: "2026-08" };
        expect(inMonthRange("2026-06-01", range)).toBe(true);
        expect(inMonthRange("2026-08-31", range)).toBe(true);
    });

    it("excludes months outside the range", () => {
        const range = { from: "2026-06", to: "2026-08" };
        expect(inMonthRange("2026-05-31", range)).toBe(false);
        expect(inMonthRange("2026-09-01", range)).toBe(false);
    });

    // The bug in the original single-month filter: it matched on month number alone.
    it("does not match the same month in a different year", () => {
        expect(inMonthRange("2025-08-17", { from: "2026-08", to: "2026-08" })).toBe(false);
    });

    it("treats a from-only range as open-ended forward", () => {
        expect(inMonthRange("2030-01-01", { from: "2026-06", to: "" })).toBe(true);
        expect(inMonthRange("2026-05-01", { from: "2026-06", to: "" })).toBe(false);
    });

    it("treats a to-only range as open-ended backward", () => {
        expect(inMonthRange("2001-01-01", { from: "", to: "2026-06" })).toBe(true);
        expect(inMonthRange("2026-07-01", { from: "", to: "2026-06" })).toBe(false);
    });

    it("orders across a year boundary correctly", () => {
        const range = { from: "2025-11", to: "2026-02" };
        expect(inMonthRange("2025-12-15", range)).toBe(true);
        expect(inMonthRange("2026-03-01", range)).toBe(false);
    });

    it("excludes a dateless record once a range is set", () => {
        expect(inMonthRange("", { from: "2026-01", to: "" })).toBe(false);
    });
});

describe("filterByMonthRange", () => {
    const rows = [{ date: "2026-05-01" }, { date: "2026-07-15" }, { date: "2026-09-30" }];

    it("returns the same list untouched when inactive", () => {
        expect(filterByMonthRange(rows, EMPTY_MONTH_RANGE)).toBe(rows);
    });

    it("keeps only rows inside the range", () => {
        expect(filterByMonthRange(rows, { from: "2026-06", to: "2026-08" })).toEqual([{ date: "2026-07-15" }]);
    });

    it("reads a custom date field", () => {
        const jobs = [{ receivedDate: "2026-07-01" }, { receivedDate: "2026-01-01" }];
        expect(filterByMonthRange(jobs, { from: "2026-06", to: "" }, (j) => j.receivedDate)).toHaveLength(1);
    });

    it("handles an empty list", () => {
        expect(filterByMonthRange([], { from: "2026-01", to: "" })).toEqual([]);
        expect(filterByMonthRange(undefined, { from: "2026-01", to: "" })).toEqual([]);
    });
});

// --- default ranges -------------------------------------------------------------------

import { defaultMonthRange } from "./monthRange";

describe("defaultMonthRange", () => {
    it("spans last month to this month", () => {
        expect(defaultMonthRange(new Date(2026, 8, 12))).toEqual({ from: "2026-08", to: "2026-09" });
    });

    it("crosses the year boundary", () => {
        expect(defaultMonthRange(new Date(2026, 0, 5))).toEqual({ from: "2025-12", to: "2026-01" });
    });
});
