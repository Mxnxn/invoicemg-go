import { describe, it, expect } from "vitest";
import {
    pendingRowsCount,
    isReadyForInvoice,
    isFullyConverted,
    isFullyInvoiced,
    isJobPaid,
    jobStatusCounts,
} from "./clientJobsMath";

// A row as the Jobs tab sees it - entry_id is populated once the row has been converted.
const row = (over = {}) => ({ queue: "Done", entry_id: null, ...over });
const converted = (over = {}) => row({ entry_id: { _id: "e1", has_issued: false, total: 100 }, ...over });

describe("pendingRowsCount", () => {
    it("counts rows not yet Done", () => {
        expect(pendingRowsCount({ rows: [row({ queue: "Printing" }), row(), row({ queue: "Created" })] })).toBe(2);
    });

    it("is zero for a job with no rows", () => {
        expect(pendingRowsCount({ rows: [] })).toBe(0);
    });

    it("tolerates a job with no rows array at all", () => {
        expect(pendingRowsCount({})).toBe(0);
    });
});

describe("isReadyForInvoice", () => {
    it("is true once every row is Done", () => {
        expect(isReadyForInvoice({ rows: [row(), row()] })).toBe(true);
    });

    it("is false while any row is still in the queue", () => {
        expect(isReadyForInvoice({ rows: [row(), row({ queue: "Printing" })] })).toBe(false);
    });

    // An empty job is not "ready" - there's nothing to bill.
    it("is false for a job with no rows", () => {
        expect(isReadyForInvoice({ rows: [] })).toBe(false);
    });
});

describe("isFullyConverted / isFullyInvoiced", () => {
    it("is converted once every row has an entry", () => {
        expect(isFullyConverted({ rows: [converted(), converted()] })).toBe(true);
    });

    it("is not converted while one row still has none", () => {
        expect(isFullyConverted({ rows: [converted(), row()] })).toBe(false);
    });

    it("is invoiced only when every entry has been issued", () => {
        const issued = converted({ entry_id: { _id: "e1", has_issued: true, total: 0 } });
        expect(isFullyInvoiced({ rows: [issued, issued] })).toBe(true);
        expect(isFullyInvoiced({ rows: [issued, converted()] })).toBe(false);
    });

    it("cannot be invoiced without being converted", () => {
        expect(isFullyInvoiced({ rows: [row(), row()] })).toBe(false);
    });
});

describe("isJobPaid", () => {
    it("is paid when the advance covers the job total", () => {
        expect(isJobPaid({ total: 1000, advance: 1000, rows: [row()] })).toBe(true);
    });

    it("is not paid on a partial advance", () => {
        expect(isJobPaid({ total: 1000, advance: 400, rows: [row()] })).toBe(false);
    });

    // The downstream case: an invoice was closed, zeroing the entries, without the job's own
    // advance ever moving.
    it("is paid when every converted entry has settled to zero", () => {
        const settled = converted({ entry_id: { _id: "e1", has_issued: true, total: 0 } });
        expect(isJobPaid({ total: 1000, advance: 0, rows: [settled, settled] })).toBe(true);
    });

    it("is not paid when one converted entry still carries a balance", () => {
        const settled = converted({ entry_id: { _id: "e1", has_issued: true, total: 0 } });
        expect(isJobPaid({ total: 1000, advance: 0, rows: [settled, converted()] })).toBe(false);
    });
});

describe("jobStatusCounts", () => {
    const pendingJob = { rows: [row({ queue: "Printing" })] };
    const readyJob = { rows: [row()] };
    const invoicedJob = { rows: [converted({ entry_id: { _id: "e1", has_issued: true, total: 0 } })] };

    it("buckets each job exactly once", () => {
        expect(jobStatusCounts([pendingJob, readyJob, invoicedJob])).toEqual({
            pending: 1,
            readyForInvoice: 1,
            invoiced: 1,
            total: 3,
        });
    });

    // The buckets have to partition the list, or the cards won't add up to Total Jobs.
    it("always sums to the total", () => {
        const jobs = [pendingJob, readyJob, invoicedJob, readyJob, pendingJob];
        const counts = jobStatusCounts(jobs);
        expect(counts.pending + counts.readyForInvoice + counts.invoiced).toBe(counts.total);
    });

    it("counts an invoiced job as invoiced, not as ready for invoice", () => {
        expect(jobStatusCounts([invoicedJob])).toMatchObject({ invoiced: 1, readyForInvoice: 0 });
    });

    it("treats a job with no rows as pending rather than ready", () => {
        expect(jobStatusCounts([{ rows: [] }])).toMatchObject({ pending: 1, readyForInvoice: 0 });
    });

    it("handles an empty list", () => {
        expect(jobStatusCounts([])).toEqual({ pending: 0, readyForInvoice: 0, invoiced: 0, total: 0 });
    });

    it("defaults to an empty list when called with nothing", () => {
        expect(jobStatusCounts().total).toBe(0);
    });
});
