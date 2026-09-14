import { describe, it, expect } from "vitest";
import { rowNetAmount, describeGoods, describeSize, taxLines, totalTax, hsnTaxRows, hasMixedRates, rowGstPct } from "./invoiceFormat";

const item = (over = {}) => ({ qty: 2, rate: 100, length: 1, width: 1, discount: 0, charges: 0, cgst: 9, sgst: 9, igst: 0, ...over });

describe("rowNetAmount", () => {
    it("multiplies qty, rate and both dimensions", () => {
        expect(rowNetAmount(item({ qty: 2, rate: 50, length: 3, width: 4 }))).toBe(1200);
    });

    it("treats missing dimensions as 1 so flat-rate lines still total", () => {
        expect(rowNetAmount({ qty: 2, rate: 50 })).toBe(100);
    });

    it("nets discount and charges before tax", () => {
        expect(rowNetAmount(item({ qty: 1, rate: 1000, discount: 100, charges: 50 }))).toBe(950);
    });

    it("is zero for an empty line rather than NaN", () => {
        expect(rowNetAmount({})).toBe(0);
    });
});

describe("describeGoods", () => {
    it("joins product and job-card description with a colon", () => {
        expect(describeGoods({ material: "Product 1", description: "Kanan" })).toBe("Product 1: Kanan");
    });

    it("shows the product alone when there's no description", () => {
        expect(describeGoods({ material: "Product 1", description: "" })).toBe("Product 1");
    });

    it("shows the description alone when there's no product", () => {
        expect(describeGoods({ description: "Kanan" })).toBe("Kanan");
    });

    // The classic template used to read item.product, which is undefined on anything created
    // through the Jobs flow - kept as a fallback for older records.
    it("falls back to the legacy product field", () => {
        expect(describeGoods({ product: "Legacy", description: "note" })).toBe("Legacy: note");
    });

    it("prefers material over the legacy field", () => {
        expect(describeGoods({ material: "New", product: "Old" })).toBe("New");
    });

    it("trims stray whitespace rather than emitting a dangling colon", () => {
        expect(describeGoods({ material: " Product 1 ", description: "  " })).toBe("Product 1");
    });

    it("is an empty string for an empty item", () => {
        expect(describeGoods({})).toBe("");
        expect(describeGoods()).toBe("");
    });
});

describe("taxLines", () => {
    it("sums one CGST and one SGST line, carrying the rate", () => {
        const lines = taxLines([item({ qty: 1, rate: 1000 }), item({ qty: 1, rate: 500 })]);
        expect(lines.map((l) => l.label)).toEqual(["CGST 9%", "SGST 9%"]);
        expect(lines[0].amount).toBeCloseTo(135); // 9% of 1500
        expect(lines[1].amount).toBeCloseTo(135);
    });

    // Reversed deliberately: the reader expects to see which slab was applied, and CGST/SGST
    // are always half the combined rate.
    it("states the rate in the label", () => {
        expect(taxLines([item()]).every((l) => l.label.includes("%"))).toBe(true);
    });

    // Previously collapsed into one line per tax type, with the breakdown left to the Tax
    // Details table (hsnTaxRows). Changed on request: a single "CGST" line has to state SOME
    // percentage, and any one it states is wrong for part of a mixed-slab invoice.
    it("breaks mixed rates out per slab", () => {
        const lines = taxLines([
            item({ qty: 1, rate: 1000, cgst: 9, sgst: 9 }),
            item({ qty: 1, rate: 1000, cgst: 6, sgst: 6 }),
        ]);
        expect(lines.map((l) => l.label)).toEqual(["CGST 6%", "CGST 9%", "SGST 6%", "SGST 9%"]);
        // Same money, stated per slab: 60 + 90 on each side.
        expect(lines.reduce((sum, l) => sum + l.amount, 0)).toBeCloseTo(300);
    });

    it("handles IGST", () => {
        const lines = taxLines([item({ qty: 1, rate: 1000, cgst: 0, sgst: 0, igst: 18 })]);
        expect(lines).toHaveLength(1);
        expect(lines[0].label).toBe("IGST 18%");
        expect(lines[0].amount).toBeCloseTo(180);
    });

    it("skips zero-rated components instead of printing empty lines", () => {
        expect(taxLines([item({ cgst: 9, sgst: 9, igst: 0 })]).map((l) => l.label)).toEqual(["CGST 9%", "SGST 9%"]);
    });

    it("returns nothing for a zero-rated invoice", () => {
        expect(taxLines([item({ cgst: 0, sgst: 0, igst: 0 })])).toEqual([]);
    });

    it("returns nothing for no entries", () => {
        expect(taxLines([])).toEqual([]);
        expect(taxLines()).toEqual([]);
    });

    it("applies tax to the discounted base, not the gross", () => {
        const lines = taxLines([item({ qty: 1, rate: 1000, discount: 100, cgst: 10, sgst: 0 })]);
        expect(lines[0].amount).toBeCloseTo(90); // 10% of 900
    });
});

describe("totalTax", () => {
    it("adds every tax line", () => {
        expect(totalTax([item({ qty: 1, rate: 1000 })])).toBeCloseTo(180);
    });

    it("is zero with no entries", () => {
        expect(totalTax([])).toBe(0);
    });
});

describe("hsnTaxRows", () => {
    it("groups lines sharing an HSN and rate", () => {
        const rows = hsnTaxRows([
            item({ hsn: "4901", qty: 1, rate: 1000 }),
            item({ hsn: "4901", qty: 1, rate: 500 }),
        ]);
        expect(rows).toHaveLength(1);
        expect(rows[0].taxable).toBe(1500);
    });

    // The bug: keying on HSN alone taxed the second line at the first line's rate.
    it("splits one HSN carrying two different rates", () => {
        const rows = hsnTaxRows([
            item({ hsn: "4901", qty: 1, rate: 1000, cgst: 9, sgst: 9 }),
            item({ hsn: "4901", qty: 1, rate: 1000, cgst: 6, sgst: 6 }),
        ]);
        expect(rows).toHaveLength(2);
        expect(rows.map((r) => r.cgst).sort()).toEqual([6, 9]);
        expect(rows.every((r) => r.taxable === 1000)).toBe(true);
    });

    it("keeps different HSNs apart even at the same rate", () => {
        const rows = hsnTaxRows([item({ hsn: "4901" }), item({ hsn: "3926" })]);
        expect(rows).toHaveLength(2);
    });

    it("labels a missing HSN rather than dropping the line", () => {
        const rows = hsnTaxRows([item({ hsn: "" })]);
        expect(rows[0].hsn).toBe("-");
    });

    it("is empty for no entries", () => {
        expect(hsnTaxRows([])).toEqual([]);
        expect(hsnTaxRows()).toEqual([]);
    });
});

describe("hasMixedRates", () => {
    it("is false when every line shares a rate", () => {
        expect(hasMixedRates([item(), item({ rate: 999 })])).toBe(false);
    });

    it("is true when rates differ", () => {
        expect(hasMixedRates([item({ cgst: 9, sgst: 9 }), item({ cgst: 6, sgst: 6 })])).toBe(true);
    });

    it("is false for a single line or none", () => {
        expect(hasMixedRates([item()])).toBe(false);
        expect(hasMixedRates([])).toBe(false);
    });
});

describe("rowGstPct", () => {
    it("adds the components into the headline rate", () => {
        expect(rowGstPct({ cgst: 9, sgst: 9 })).toBe(18);
        expect(rowGstPct({ igst: 12 })).toBe(12);
        expect(rowGstPct({})).toBe(0);
    });
});

describe("taxLines rate labelling", () => {
	const row = (cgst, sgst, rate = 100) => ({ qty: 1, length: 1, width: 1, rate, cgst, sgst, igst: 0 });

	it("shows the rate beside CGST/SGST when every row is on the same slab", () => {
		const lines = taxLines([row(9, 9), row(9, 9)]);
		expect(lines.map((l) => l.label)).toEqual(["CGST 9%", "SGST 9%"]);
		// 18% of 200 = 36, split evenly.
		expect(lines[0].amount).toBeCloseTo(18, 5);
		expect(lines[1].amount).toBeCloseTo(18, 5);
	});

	it("keeps halves of an odd slab readable", () => {
		const lines = taxLines([row(2.5, 2.5)]);
		expect(lines.map((l) => l.label)).toEqual(["CGST 2.5%", "SGST 2.5%"]);
	});

	it("breaks tax out per rate when rows sit on different slabs", () => {
		const lines = taxLines([row(9, 9), row(2.5, 2.5)]);
		// One line per component per rate - a single "CGST" line would state a percentage
		// that is wrong for some of the goods.
		expect(lines.map((l) => l.label)).toEqual(["CGST 2.5%", "CGST 9%", "SGST 2.5%", "SGST 9%"]);
	});

	it("omits components that carry no tax", () => {
		const lines = taxLines([{ qty: 1, length: 1, width: 1, rate: 100, cgst: 0, sgst: 0, igst: 18 }]);
		expect(lines.map((l) => l.label)).toEqual(["IGST 18%"]);
	});

	it("totals the same either way", () => {
		const mixed = [row(9, 9), row(2.5, 2.5)];
		expect(totalTax(mixed)).toBeCloseTo(18 + 5, 5);
	});
});

describe("describeSize, and the size folded into describeGoods", () => {
    it("states a size when the row has one", () => {
        expect(describeSize({ length: "2", width: "3" })).toBe("2 x 3");
        expect(describeSize({ length: "2.5", width: "0.75" })).toBe("2.5 x 0.75");
    });

    it("states nothing for a by-quantity row, whatever its stale sides say", () => {
        expect(describeSize({ hasDimensions: false, length: "2", width: "3" })).toBe("");
        expect(describeSize({ hasDimensions: false })).toBe("");
    });

    it("states nothing when a side was never recorded", () => {
        // Printing "1 x 1" here would state a measurement nobody took. Pricing still treats
        // the blank as 1 - see the rowNetAmount case below.
        expect(describeSize({})).toBe("");
        expect(describeSize({ length: "", width: "" })).toBe("");
        expect(describeSize({ length: "2", width: "" })).toBe("");
    });

    it("keeps a genuine 1x1, which is a real size", () => {
        expect(describeSize({ length: "1", width: "1" })).toBe("1 x 1");
    });

    it("separates the size from the note with a middle dot", () => {
        expect(describeGoods({ material: "Vinyl", description: "Banner", length: "2", width: "3" }))
            .toBe("Vinyl: Banner · 2 x 3");
        expect(describeGoods({ material: "Vinyl", length: "2", width: "3" })).toBe("Vinyl · 2 x 3");
    });

    it("emits no dot when there is no size, and no leading dot when there is no note", () => {
        expect(describeGoods({ material: "Design charge", hasDimensions: false })).toBe("Design charge");
        expect(describeGoods({ length: "2", width: "3" })).toBe("2 x 3");
    });

    it("leaves a by-quantity line as description only", () => {
        expect(describeGoods({ material: "Design charge", hasDimensions: false, length: "1", width: "1" }))
            .toBe("Design charge");
    });
});

describe("rowNetAmount across both kinds of row", () => {
    it("prices a by-dimension row by area", () => {
        expect(rowNetAmount({ qty: 4, rate: 120, length: "2", width: "3" })).toBe(2880);
    });

    it("prices a by-quantity row by count, ignoring stale sides", () => {
        expect(rowNetAmount({ qty: 1, rate: 2500, hasDimensions: false, length: "9", width: "9" })).toBe(2500);
    });

    it("still bills a blank-sided legacy entry as 1 x 1, even though it prints no size", () => {
        // The pair that must not be collapsed: billed as 1, printed as nothing.
        const legacy = { qty: 2, rate: 50, length: "", width: "" };
        expect(rowNetAmount(legacy)).toBe(100);
        expect(describeSize(legacy)).toBe("");
    });

    it("applies discount and charges to both kinds", () => {
        expect(rowNetAmount({ qty: 1, rate: 1000, hasDimensions: false, discount: 100, charges: 50 })).toBe(950);
    });
});
