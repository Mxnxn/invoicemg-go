import { round2 } from "../../Common/money";
import { hasDimensions, dimensionFactor } from "../../Common/rowPricing";
// Shared pricing formula for a job row - same shape Entry/Quotation rows use: area-based
// amount, discount/charges applied before tax. Jobs additionally carry IGST (Quotation rows
// don't) since a converted-to-Entry row needs to be able to express either intra- or
// inter-state tax.
// Every figure is rounded to paise as it is produced. Area x rate x tax leaves tails like
// 67.79999999999998, and rounding only at render meant the stored total and the displayed
// total could disagree by a paisa. Rounding at the source keeps them identical.
export const rowAmount = (row) =>
    round2(Number(row.qty || 0) * dimensionFactor(row) * Number(row.rate || 0));

export const rowNetAmount = (row) => round2(rowAmount(row) - Number(row.discount || 0) + Number(row.charges || 0));

export const rowTotal = (row) => {
    const taxPct = Number(row.cgst || 0) / 100 + Number(row.sgst || 0) / 100 + Number(row.igst || 0) / 100;
    return round2(rowNetAmount(row) * (1 + taxPct));
};

export const jobGrandTotal = (rows) => round2((rows || []).reduce((sum, row) => sum + rowTotal(row), 0));

export const emptyJobRow = (hasDims = true) => ({
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

// --- Conditionally required row fields ----------------------------------------------------
// A blank row is not an error - every job card starts with one, and the user may leave a
// spare at the bottom. Picking a material is what turns a row into a real line item, and
// from that point the four factors of rowAmount (qty x length x width x rate) must each be
// non-zero, or the line silently contributes nothing to the total.
//
// Rows already converted to an Entry (`entry_id`) are locked and exempt: their values are
// historical and the inputs are disabled.
export const ROW_REQUIRED_FIELDS = [
    ["length", "Length"],
    ["width", "Width"],
    ["qty", "Qty"],
    ["rate", "Rate"],
];

export const rowNeedsField = (row, field) => {
    if (!row || !row.material || row.entry_id) return false;
    // A by-quantity row has no length or width to require. Demanding them would disable
    // submit on a row that is complete, locking the user out of the flow entirely.
    if ((field === "length" || field === "width") && !hasDimensions(row)) return false;
    return !(Number(row[field]) > 0);
};

// The human labels for everything still missing across a set of rows, for MissingFieldsHint.
export const missingRowFields = (rows) =>
    ROW_REQUIRED_FIELDS.filter(([field]) => (rows || []).some((row) => rowNeedsField(row, field))).map(
        ([, label]) => label
    );

// The row's physical size, rendered as the "<length> x <width>" spec line used on the
// customer job page and every invoice template - one format everywhere, so a row reads the
// same wherever it is shown.
// A by-dimension row prints its size even when both sides are the schema default of 1 - a
// 1x1 sheet is a real size, and blanking it would look like missing data. A by-quantity row
// prints nothing, because it genuinely has no size; that distinction is carried by the stored
// flag, not guessed from the values, since "1" and "1" cannot tell the two apart.
// Sides stay as written rather than being coerced to numbers - they are free-text on JobRow,
// and "2.50" is a deliberate way to write a size.
export const rowDimensions = (row) => {
    // A by-quantity row has no size, and must render as nothing rather than as "1 x 1" -
    // printing a size a row does not have is the bug this whole feature exists to fix.
    if (!hasDimensions(row)) return "";
    const side = (value) => {
        const text = String(value ?? "").trim();
        return text === "" ? "1" : text;
    };
    return `${side(row?.length)} x ${side(row?.width)}`;
};
