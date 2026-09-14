import { splitGst } from "../../Common/gst";

// Which tax regime a job is under.
//
// A sale is intrastate (CGST + SGST) or interstate (IGST) - never both. The place of supply
// is a property of the sale, not of one line on it, so this is decided for the whole job:
// one row being IGST makes every row IGST, and the GST% column stops being editable.
//
// Derived from the rows rather than stored as a flag on the job, so an existing job re-reads
// as whatever it actually contains - there is no second source of truth to fall out of step,
// and no migration for jobs raised before the rule existed.

// GST is stored split across cgst/sgst (9 + 9 for an 18% intrastate sale) but entered and
// read as one number.
export const gstPercent = (row) => (Number(row.cgst) || 0) + (Number(row.sgst) || 0);

export const isIgstJob = (rows) => (rows || []).some((row) => Number(row.igst) > 0);

// The row's whole tax rate, whichever side it currently sits on. Used when switching sides so
// the rate carries across instead of being silently zeroed - it is the same tax at the same
// percentage, only recorded on the other side.
export const totalTaxPercent = (row) => Number(row.igst) || gstPercent(row);

// The rate cannot live on both sides at once - rowGrossTotal sums cgst + sgst + igst, so a
// row holding both would be taxed twice. But zeroing the GST outright means switching to
// IGST and back wipes every rate and the products have to be picked again. So the old rate
// is stashed on the row instead, and restored when the job returns to GST.
//
// _prevGst never reaches the database: the API whitelists row fields (normalizeRow in
// routes/Lifecycle.js), so anything it does not name is dropped on the way in.
export const toIgstRow = (row) => {
    const previous = gstPercent(row);
    return {
        ...row,
        igst: totalTaxPercent(row),
        cgst: 0,
        sgst: 0,
        ...(previous > 0 ? { _prevGst: previous } : {}),
    };
};

export const toGstRow = (row) => {
    const { _prevGst, ...rest } = row;
    // What it was before the switch, falling back to whatever IGST now holds - a row that was
    // only ever IGST has no earlier GST to go back to.
    const rate = Number(_prevGst) || totalTaxPercent(row);
    return { ...rest, ...splitGst(rate), igst: 0 };
};

// The whole-table effect of editing one row's IGST field.
//
// Setting it converts every row; clearing the last one converts them all back. Clearing a row
// while another still carries IGST leaves the job interstate - only that row goes to zero.
export function applyIgstEdit(rows, index, value) {
    const next = Number(value) || 0;
    const edited = (rows || []).map((row, i) => (i === index ? { ...row, igst: value } : row));
    if (next > 0) return edited.map(toIgstRow);
    if (edited.some((row, i) => i !== index && Number(row.igst) > 0)) return edited;
    return edited.map(toGstRow);
}

// What picking a product should write, given the regime the job is in. On an IGST job the
// product's rate goes to IGST whole rather than being halved across CGST/SGST - an interstate
// sale has no intrastate split to report.
//
// Takes the DECISION, not the rows to re-derive it from. It used to take rows and call
// isIgstJob itself, which cannot see a regime nobody has typed a rate for: switch a blank job
// to IGST, pick a product, and isIgstJob still said false because every row's igst was 0 - so
// the product's tax was halved into CGST/SGST and the IGST column the user was looking at
// stayed at zero. The caller already knows the answer (it is drawing an IGST column), and one
// place knowing it is the only way the two cannot disagree.
export function taxFromMaterial(material, isIgst) {
    const tax = Number(material?.tax) || 0;
    // Tolerates the old rows-array form so a stale caller degrades to the previous behaviour
    // rather than silently treating a non-empty array as truthy and forcing IGST on.
    const interstate = Array.isArray(isIgst) ? isIgstJob(isIgst) : Boolean(isIgst);
    if (interstate) return { igst: tax, cgst: 0, sgst: 0 };
    return { ...splitGst(tax), igst: 0 };
}
