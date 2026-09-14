import { dimensionFactor } from "../../Common/rowPricing";
import { combinedGst } from "../../Common/gst";

// Shared pricing formula for a quotation row - same shape Entry/Job use: discount/charges
// applied before tax. A row prices by area or by quantity; dimensionFactor is 1 for the
// latter, so one expression covers both. See Common/rowPricing.js.
export const rowAmount = (row) => Number(row.qty || 0) * dimensionFactor(row) * Number(row.rate || 0);

export const rowNetAmount = (row) => rowAmount(row) - Number(row.discount || 0) + Number(row.charges || 0);

// A row is taxed on one side or the other, never both: an intrastate row splits its rate
// across cgst + sgst, an interstate one carries the whole rate on igst. Summing all three is
// therefore the row's rate whichever way it was recorded - and leaving igst out priced an
// interstate row as though it were untaxed, on screen and in the PDF alike.
export const rowTotal = (row) => {
    const taxPct = combinedGst(row) / 100;
    return rowNetAmount(row) * (1 + taxPct);
};

export const quotationGrandTotal = (rows) => (rows || []).reduce((sum, row) => sum + rowTotal(row), 0);

export const emptyQuotationRow = (hasDims = true) => ({
    material: "",
    description: "",
    hasDimensions: hasDims,
    length: "1",
    width: "1",
    qty: 1,
    rate: 0,
    cgst: 0,
    sgst: 0,
    igst: 0,
    discount: 0,
    charges: 0,
});
