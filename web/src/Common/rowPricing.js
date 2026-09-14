// Whether a line prices by area or by quantity, and the factor that follows from it.
// Mirror of the API's Helpers/RowPricing.js - the two must agree, or a job priced in the
// browser and the same job re-totalled on the server disagree about what is owed.
//
// Every row on a job, quotation, entry or purchase invoice is one of two kinds. The Add row
// popup sets which one it starts as; RowSizeCells can switch it at any time after:
//
//   by dimension - qty * length * width * rate   (a 2x3 banner)
//   by quantity  - qty * rate                    (a design charge, a delivery)
//
// The kind is stored, not inferred. Inferring it cannot work: length "1" and width "1" are
// both the schema default AND a legitimate 1x1 sheet, so a row with no dimensions and a row
// that genuinely measures one by one are indistinguishable by value.
//
// `hasDimensions` is absent on every row written before this existed, and absent must mean
// "by dimension" - that is what those rows are. Hence `!== false` rather than a truthiness
// test, and no backfill migration.
export const hasDimensions = (row) => !row || row.hasDimensions !== false;

// The area multiplier: 1 for a by-quantity row, which collapses qty * factor * rate back to
// qty * rate without a second code path.
//
// `blank` is what an empty or unparseable side counts as, and callers disagree on purpose:
//
//   0 - job and quotation rows, whose length and width are validated > 0 before the row can
//       be saved, so a blank there is a real zero.
//   1 - entries and the invoice templates. Entries predate that validation and legitimately
//       carry "" for flat-rate lines; counting those as zero would re-price invoices that
//       have already gone to customers.
//
// Collapsing the two would be tidier and wrong.
export const dimensionFactor = (row, blank = 0) => {
    if (!hasDimensions(row)) return 1;
    const side = (value) => Number(value) || blank;
    return side(row && row.length) * side(row && row.width);
};
