import { describe, it, expect } from "vitest";
import { isIgstJob, totalTaxPercent, toIgstRow, toGstRow, applyIgstEdit, taxFromMaterial } from "./jobTax";

const gstRow = (over = {}) => ({ material: "Flex", cgst: 9, sgst: 9, igst: 0, ...over });
const igstRow = (over = {}) => ({ material: "Flex", cgst: 0, sgst: 0, igst: 18, ...over });

describe("isIgstJob", () => {
	it("is interstate as soon as any single row is", () => {
		expect(isIgstJob([gstRow(), igstRow()])).toBe(true);
	});

	it("is intrastate when no row carries IGST", () => {
		expect(isIgstJob([gstRow(), gstRow()])).toBe(false);
		expect(isIgstJob([])).toBe(false);
		expect(isIgstJob(undefined)).toBe(false);
	});

	// A blank field reads as "not set", not as an IGST job with a zero rate - otherwise
	// clearing the box would leave the GST column disabled with nothing to fill it.
	it("does not count a zero or blank IGST", () => {
		expect(isIgstJob([{ igst: 0 }])).toBe(false);
		expect(isIgstJob([{ igst: "" }])).toBe(false);
	});
});

describe("switching sides", () => {
	it("carries the rate across rather than zeroing it", () => {
		expect(toIgstRow(gstRow())).toMatchObject({ igst: 18, cgst: 0, sgst: 0 });
		expect(toGstRow(igstRow())).toMatchObject({ igst: 0, cgst: 9, sgst: 9 });
	});

	it("round-trips without losing the rate", () => {
		expect(toGstRow(toIgstRow(gstRow()))).toMatchObject({ cgst: 9, sgst: 9, igst: 0 });
	});

	it("reads the rate from whichever side currently holds it", () => {
		expect(totalTaxPercent(gstRow())).toBe(18);
		expect(totalTaxPercent(igstRow())).toBe(18);
		expect(totalTaxPercent({})).toBe(0);
	});

	it("leaves everything else on the row alone", () => {
		const row = gstRow({ material: "Vinyl", qty: 4, rate: 120, discount: 10 });
		expect(toIgstRow(row)).toMatchObject({ material: "Vinyl", qty: 4, rate: 120, discount: 10 });
	});
});

describe("applyIgstEdit", () => {
	it("converts EVERY row when one row is set to IGST", () => {
		const rows = [gstRow(), gstRow({ cgst: 6, sgst: 6 })];
		const next = applyIgstEdit(rows, 0, 18);
		expect(next[0]).toMatchObject({ igst: 18, cgst: 0, sgst: 0 });
		// The second row keeps its own 12%, moved to the other side - not overwritten with
		// the edited row's rate.
		expect(next[1]).toMatchObject({ igst: 12, cgst: 0, sgst: 0 });
	});

	it("stays interstate while any other row still carries IGST", () => {
		const rows = [igstRow(), igstRow({ igst: 12 })];
		const afterFirst = applyIgstEdit(rows, 0, "");
		expect(isIgstJob(afterFirst)).toBe(true);
		expect(afterFirst[1]).toMatchObject({ igst: 12 });
	});

	// Switching to IGST and back must not cost the rates - otherwise trying it once means
	// re-picking every product.
	it("restores the original GST when the job returns from IGST", () => {
		const rows = [gstRow(), gstRow({ cgst: 6, sgst: 6 })];
		const asIgst = applyIgstEdit(rows, 0, 18);
		expect(asIgst.map((r) => r.igst)).toEqual([18, 12]);

		// Both have to be cleared: while any row still carries IGST the job is still
		// interstate, which is what the test above covers.
		const back = applyIgstEdit(applyIgstEdit(asIgst, 0, ""), 1, "");
		expect(isIgstJob(back)).toBe(false);
		expect(back[0]).toMatchObject({ cgst: 9, sgst: 9, igst: 0 });
		expect(back[1]).toMatchObject({ cgst: 6, sgst: 6, igst: 0 });
	});

	it("does not carry the scratch field into the saved row shape", () => {
		const back = applyIgstEdit(applyIgstEdit([gstRow()], 0, 18), 0, "");
		expect(back[0]).not.toHaveProperty("_prevGst");
	});

	it("never leaves a row taxed on both sides at once", () => {
		const rows = [gstRow(), gstRow()];
		applyIgstEdit(rows, 0, 18).forEach((row) => {
			const bothSides = Number(row.igst) > 0 && gstPercentOf(row) > 0;
			expect(bothSides).toBe(false);
		});
	});
});

const gstPercentOf = (row) => (Number(row.cgst) || 0) + (Number(row.sgst) || 0);

describe("taxFromMaterial", () => {
	it("halves the product's rate across CGST and SGST on an intrastate job", () => {
		expect(taxFromMaterial({ tax: 18 }, false)).toEqual({ cgst: 9, sgst: 9, igst: 0 });
	});

	it("puts the whole rate on IGST for an interstate job", () => {
		expect(taxFromMaterial({ tax: 18 }, true)).toEqual({ igst: 18, cgst: 0, sgst: 0 });
	});

	// The bug this signature exists to prevent: a job switched to IGST before any rate was
	// typed has igst 0 on every row, so deriving the regime from the rows reads intrastate and
	// the product's tax is halved into CGST/SGST while the IGST column shows nothing.
	it("honours an IGST job that has no rate on any row yet", () => {
		expect(taxFromMaterial({ tax: 18 }, true)).toEqual({ igst: 18, cgst: 0, sgst: 0 });
		// what the old rows-derived form would have produced for the same situation
		expect(taxFromMaterial({ tax: 18 }, [{ cgst: 0, sgst: 0, igst: 0 }])).toEqual({ cgst: 9, sgst: 9, igst: 0 });
	});

	it("writes zeros for a product with no tax on it", () => {
		expect(taxFromMaterial({}, true)).toEqual({ igst: 0, cgst: 0, sgst: 0 });
		expect(taxFromMaterial(null, false)).toEqual({ cgst: 0, sgst: 0, igst: 0 });
	});

	// A stale caller passing rows still gets the old behaviour rather than a non-empty array
	// being read as truthy and forcing every job interstate.
	it("still accepts the rows form it used to take", () => {
		expect(taxFromMaterial({ tax: 18 }, [igstRow()])).toEqual({ igst: 18, cgst: 0, sgst: 0 });
		expect(taxFromMaterial({ tax: 18 }, [gstRow()])).toEqual({ cgst: 9, sgst: 9, igst: 0 });
	});
});


// The column swap on the job form: IGST% pops out of Extra and takes the GST% column's place,
// and its X puts GST% back. The tax rate has to survive both directions - a product picked at
// 18% must still be taxed at 18% whichever side of the form it is showing on, or switching
// regime quietly zero-rates the job.
describe("the IGST column swap carries the rate", () => {
	const rowsAt18 = () => [
		{ material: "Vinyl", cgst: 9, sgst: 9, igst: 0 },
		{ material: "Flex", cgst: 9, sgst: 9, igst: 0 },
	];

	it("moves the product's tax onto IGST for every row when the column opens", () => {
		const igst = rowsAt18().map(toIgstRow);

		expect(igst.every((r) => Number(r.igst) === 18)).toBe(true);
		expect(igst.every((r) => Number(r.cgst) === 0 && Number(r.sgst) === 0)).toBe(true);
	});

	it("puts the same rate back on GST when the column is closed again", () => {
		const back = rowsAt18().map(toIgstRow).map(toGstRow);

		expect(back.every((r) => Number(r.cgst) === 9 && Number(r.sgst) === 9)).toBe(true);
		expect(back.every((r) => Number(r.igst) === 0)).toBe(true);
	});

	// A row typed straight into IGST has no earlier GST to restore, so its IGST rate becomes
	// the GST rate rather than collapsing to zero.
	it("keeps the rate for a row that was only ever IGST", () => {
		const back = toGstRow({ material: "Vinyl", cgst: 0, sgst: 0, igst: 18 });

		expect(Number(back.cgst) + Number(back.sgst)).toBe(18);
	});

	// An untaxed row stays untaxed - the swap must not invent a rate.
	it("leaves a zero-rated row at zero", () => {
		expect(Number(toIgstRow({ cgst: 0, sgst: 0, igst: 0 }).igst)).toBe(0);
		expect(Number(toGstRow({ cgst: 0, sgst: 0, igst: 0 }).cgst)).toBe(0);
	});

	// Odd rates halve and rejoin without drift.
	it("survives a rate that does not halve evenly", () => {
		const back = toGstRow(toIgstRow({ cgst: 2.5, sgst: 2.5, igst: 0 }));

		expect(Number(back.cgst) + Number(back.sgst)).toBe(5);
	});
});
