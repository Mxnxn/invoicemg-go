import { describe, it, expect } from "vitest";
import { groupJobsByDate, groupTotals } from "./clientJobGroups";

const job = (over = {}) => ({ _id: "j1", receivedDate: "2026-09-13", total: 1000, advance: 400, ...over });

describe("groupJobsByDate", () => {
    it("gathers a day's job-ids and sums them", () => {
        const groups = groupJobsByDate([job(), job({ _id: "j2", total: 500, advance: 100 })]);
        expect(groups).toHaveLength(1);
        expect(groups[0].jobs).toHaveLength(2);
        expect(groups[0].total).toBe(1500);
        expect(groups[0].due).toBe(1000);
    });

    it("puts the newest day first by default", () => {
        const groups = groupJobsByDate([
            job({ _id: "a", receivedDate: "2026-09-01" }),
            job({ _id: "b", receivedDate: "2026-09-13" }),
            job({ _id: "c", receivedDate: "2026-09-07" }),
        ]);
        expect(groups.map((g) => g.date)).toEqual(["2026-09-13", "2026-09-07", "2026-09-01"]);
    });

    it("turns round on request", () => {
        const groups = groupJobsByDate(
            [job({ _id: "a", receivedDate: "2026-09-01" }), job({ _id: "b", receivedDate: "2026-09-13" })],
            { oldestFirst: true }
        );
        expect(groups.map((g) => g.date)).toEqual(["2026-09-01", "2026-09-13"]);
    });

    // "YYYY-MM-DD" sorts chronologically as a string. Going through Date would shift the day in
    // any timezone behind UTC - the bug that had the dashboard showing every sheet a day early.
    it("sorts across a year boundary without parsing a date", () => {
        const groups = groupJobsByDate([
            job({ _id: "a", receivedDate: "2025-12-31" }),
            job({ _id: "b", receivedDate: "2026-01-01" }),
        ]);
        expect(groups.map((g) => g.date)).toEqual(["2026-01-01", "2025-12-31"]);
    });

    // Real work someone can see elsewhere. Hiding it would make this page disagree with the
    // table it replaced.
    it("keeps undated work, at the end, either way round", () => {
        const jobs = [job({ _id: "a", receivedDate: "" }), job({ _id: "b", receivedDate: "2026-09-13" })];
        expect(groupJobsByDate(jobs).map((g) => g.date)).toEqual(["2026-09-13", ""]);
        expect(groupJobsByDate(jobs, { oldestFirst: true }).map((g) => g.date)).toEqual(["2026-09-13", ""]);
    });

    it("survives an empty list", () => {
        expect(groupJobsByDate([])).toEqual([]);
        expect(groupJobsByDate(null)).toEqual([]);
    });
});

describe("groupTotals", () => {
    it("adds the days up", () => {
        const groups = groupJobsByDate([job(), job({ _id: "j2", receivedDate: "2026-09-12", total: 500, advance: 500 })]);
        expect(groupTotals(groups)).toEqual({ total: 1500, advance: 900, due: 600, jobs: 2, days: 2 });
    });

    it("is zeros rather than NaN when there is nothing", () => {
        expect(groupTotals([])).toEqual({ total: 0, advance: 0, due: 0, jobs: 0, days: 0 });
    });
});
