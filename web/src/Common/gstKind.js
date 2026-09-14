// Interstate or intrastate, decided from the tax actually charged.
//
// An Indian sale is one or the other, never both: within a state it splits into CGST + SGST,
// across a state line it is a single IGST. Which one an invoice carries changes where it
// appears on a GSTR filing, so the two are worth listing apart rather than reading a
// twelve-column table looking for a non-zero IGST cell.
//
// Decided per invoice from its rows rather than stored on the invoice, so an invoice built
// before this distinction existed classifies correctly too.

const num = (v) => Number(v) || 0;

export const rowIsIgst = (row) => num(row?.igst) > 0;

// One IGST row is enough. In practice they do not mix - the place of supply is a property of
// the sale, not of the line - but a job edited across a change of address could produce one,
// and the safe reading is the one that puts it in front of someone.
export const hasIgst = (rows = []) => rows.some(rowIsIgst);

export const invoiceGstKind = (invoice) => (hasIgst(invoice?.entries || []) ? "igst" : "gst");

// `kind` is "all" | "gst" | "igst".
export function filterInvoicesByGst(invoices = [], kind = "all") {
    if (kind === "all") return invoices;
    return invoices.filter((inv) => invoiceGstKind(inv) === kind);
}

export function countInvoicesByGst(invoices = []) {
    let gst = 0;
    let igst = 0;
    for (const inv of invoices) {
        if (invoiceGstKind(inv) === "igst") igst += 1;
        else gst += 1;
    }
    return { all: invoices.length, gst, igst };
}
