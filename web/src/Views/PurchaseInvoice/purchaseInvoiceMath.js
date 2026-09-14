import { dimensionFactor } from "../../Common/rowPricing";

// Purchase invoice rows are qty*rate, net of discount/charges, inclusive of GST - same shape
// as jobMath.js otherwise. They can now also be priced by dimension (you buy 4 sheets of 2x3),
// so the area factor is in the formula; it is 1 for a by-quantity row, which every purchase
// row created before this is. See Common/rowPricing.js.
export const rowAmount = (row) => Number(row.qty || 0) * dimensionFactor(row) * Number(row.rate || 0);

export const rowNetAmount = (row) => rowAmount(row) - Number(row.discount || 0) + Number(row.charges || 0);

export const rowTotal = (row) => {
    const gstPct = Number(row.gst || 0) / 100;
    return rowNetAmount(row) * (1 + gstPct);
};

export const purchaseInvoiceGrandTotal = (rows) => (rows || []).reduce((sum, row) => sum + rowTotal(row), 0);

// Defaults to by-QUANTITY, the opposite of a job or quotation row - that is what a purchase
// line has always been, and it keeps the Add row popup's default matching the common case.
export const emptyPurchaseInvoiceRow = (hasDims = false) => ({
    description: "",
    material: "",
    hsn: "",
    gst: 0,
    hasDimensions: hasDims,
    length: "1",
    width: "1",
    rate: 0,
    qty: 1,
    // The unit nearly every purchase line uses, so it is the default rather than a blank set
    // on every row. Spelled exactly as the Units list stores it - the select matches on the
    // string, so "SQ. FT" would render as an unmatched value.
    unit: "SQ. Ft",
    discount: 0,
    charges: 0,
});
