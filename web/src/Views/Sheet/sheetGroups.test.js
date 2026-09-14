import { describe, it, expect } from "vitest";
import { groupJobsByClient, sheetTotals, filterGroups, clientKey } from "./sheetGroups";

const job = (over = {}) => ({
    _id: "j1",
    challanNumber: "JOB/26-27/000001",
    receivedDate: "2026-09-13",
    total: 1000,
    advance: 400,
    client_id: { _id: "c1", clientFirm: "Himani Wa", clientName: "Himani" },
    rows: [{ _id: "r1", material: "Vinyl" }],
    ...over,
});

describe("groupJobsByClient", () => {
    it("keeps only the job-ids of that day", () => {
        const groups = groupJobsByClient([job(), job({ _id: "j2", receivedDate: "2026-09-12" })], "2026-09-13");
        expect(groups).toHaveLength(1);
        expect(groups[0].jobs).toHaveLength(1);
    });

    // The date is compared as a STRING deliberately: running either side through new Date()
    // shifts the day in any timezone behind UTC, which shows yesterday's work on today's sheet.
    it("does not shift the day in a timezone behind UTC", () => {
        const groups = groupJobsByClient([job({ receivedDate: "2026-01-01" })], "2026-01-01");
        expect(groups).toHaveLength(1);
    });

    it("gathers one customer's job-ids together and sums them", () => {
        const groups = groupJobsByClient(
            [job(), job({ _id: "j2", challanNumber: "JOB/2", total: 500, advance: 100 })],
            "2026-09-13"
        );
        expect(groups).toHaveLength(1);
        expect(groups[0].jobs).toHaveLength(2);
        expect(groups[0].total).toBe(1500);
        expect(groups[0].advance).toBe(500);
        expect(groups[0].due).toBe(1000);
    });

    it("separates customers and orders them by firm", () => {
        const groups = groupJobsByClient(
            [
                job({ _id: "j2", client_id: { _id: "c2", clientFirm: "Zenith", clientName: "Z" } }),
                job(),
                job({ _id: "j3", client_id: { _id: "c3", clientFirm: "Acme", clientName: "A" } }),
            ],
            "2026-09-13"
        );
        expect(groups.map((g) => g.firm)).toEqual(["Acme", "Himani Wa", "Zenith"]);
    });

    // A bare id and a populated document are the same customer. Grouping them apart would
    // show one firm twice, each with half its work.
    it("treats a bare client id and a populated one as the same customer", () => {
        expect(clientKey({ client_id: "c1" })).toBe("c1");
        expect(clientKey({ client_id: { _id: "c1" } })).toBe("c1");
        const groups = groupJobsByClient([job(), job({ _id: "j2", client_id: "c1" })], "2026-09-13");
        expect(groups).toHaveLength(1);
        expect(groups[0].jobs).toHaveLength(2);
    });

    it("survives a day with nothing on it", () => {
        expect(groupJobsByClient([], "2026-09-13")).toEqual([]);
        expect(groupJobsByClient(null, "2026-09-13")).toEqual([]);
    });
});

describe("sheetTotals", () => {
    it("adds the day up across customers", () => {
        const groups = groupJobsByClient(
            [job(), job({ _id: "j2", client_id: { _id: "c2", clientFirm: "Acme" }, total: 500, advance: 500 })],
            "2026-09-13"
        );
        expect(sheetTotals(groups)).toEqual({ total: 1500, advance: 900, due: 600, jobs: 2, customers: 2 });
    });

    it("is all zeros for an empty day rather than NaN", () => {
        expect(sheetTotals([])).toEqual({ total: 0, advance: 0, due: 0, jobs: 0, customers: 0 });
    });
});

describe("filterGroups", () => {
    const groups = () =>
        groupJobsByClient(
            [
                job(),
                job({
                    _id: "j2",
                    challanNumber: "JOB/26-27/000002",
                    rows: [{ _id: "r2", material: "Foam board" }],
                }),
                job({
                    _id: "j3",
                    challanNumber: "JOB/26-27/000003",
                    client_id: { _id: "c2", clientFirm: "Acme Signs", clientName: "Ravi" },
                    rows: [{ _id: "r3", material: "Acrylic" }],
                }),
            ],
            "2026-09-13"
        );

    it("returns everything for an empty search", () => {
        expect(filterGroups(groups(), "")).toHaveLength(2);
    });

    it("finds a customer by firm, and keeps all of their job-ids", () => {
        const found = filterGroups(groups(), "acme");
        expect(found).toHaveLength(1);
        expect(found[0].firm).toBe("Acme Signs");
    });

    it("finds a job-id by its number", () => {
        const found = filterGroups(groups(), "000002");
        expect(found).toHaveLength(1);
        expect(found[0].jobs.map((j) => j.challanNumber)).toEqual(["JOB/26-27/000002"]);
    });

    // The point of narrowing: a customer card must not claim two job-ids while showing one.
    it("narrows a customer to the job-ids that carried the product, and re-sums them", () => {
        const found = filterGroups(groups(), "foam");
        expect(found).toHaveLength(1);
        expect(found[0].jobs).toHaveLength(1);
        expect(found[0].total).toBe(1000);
        expect(found[0].due).toBe(600);
    });

    it("drops a customer with nothing matching at all", () => {
        expect(filterGroups(groups(), "zzzz")).toEqual([]);
    });
});
