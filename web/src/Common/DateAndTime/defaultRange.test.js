import { describe, expect, it } from "vitest";
import { defaultDateRange, sameDayLastMonth, isoDay } from "./defaultRange";

describe("defaultDateRange", () => {
    it("runs from the same day last month to today", () => {
        expect(defaultDateRange(new Date(2026, 8, 7))).toEqual({ from: "2026-08-07", to: "2026-09-07" });
    });

    // setMonth(month - 1) does NOT clamp - it overflows forward, so 31 March would become
    // 3 March and the range would silently be 28 days while claiming to be a month.
    it("clamps a day the previous month does not have", () => {
        expect(isoDay(sameDayLastMonth(new Date(2026, 2, 31)))).toBe("2026-02-28"); // 31 Mar -> 28 Feb
        expect(isoDay(sameDayLastMonth(new Date(2026, 4, 31)))).toBe("2026-04-30"); // 31 May -> 30 Apr
    });

    it("uses 29 February in a leap year", () => {
        expect(isoDay(sameDayLastMonth(new Date(2024, 2, 31)))).toBe("2024-02-29");
    });

    it("crosses a year boundary", () => {
        expect(defaultDateRange(new Date(2026, 0, 15))).toEqual({ from: "2025-12-15", to: "2026-01-15" });
    });

    // Built from local date parts, not toISOString - that converts to UTC first, so west of
    // UTC a late-evening date comes back as the following day.
    it("uses the local date, not UTC", () => {
        expect(isoDay(new Date(2026, 8, 7, 23, 30))).toBe("2026-09-07");
        expect(isoDay(new Date(2026, 8, 7, 0, 30))).toBe("2026-09-07");
    });

    it("zero-pads single digits", () => {
        expect(isoDay(new Date(2026, 0, 5))).toBe("2026-01-05");
    });
});
