import { RoundOff } from "../../Common/DateAndTime/RoundOff";

// Presentation logic for the Purchase Report, kept out of the component so it can be tested -
// the same split duesMath.js, jobMath.js and queueConstants.js use.

// The firm is how a supplier is known on their invoice; their contact name is the fallback.
export const supplierLabel = (row) => row.supplierFirm || row.supplierName || "Unknown supplier";

// A negative balance means we've paid a supplier more than they've billed - an advance
// sitting with them, not a debt. Say so rather than showing a bare minus.
export const formatOwed = (due) => (due < 0 ? `₹${RoundOff(Math.abs(due))} advance` : `₹${RoundOff(due)}`);

export const outstandingSuppliers = (rows) => rows.filter((row) => row.due > 0);

export const settledSupplierCount = (rows) => rows.length - outstandingSuppliers(rows).length;

// How much of an invoice is still owed. Purchase invoices created before payments existed
// have no `amount` field at all, so this must not assume one.
export const invoiceDue = (invoice) => Number(((Number(invoice.total) || 0) - (Number(invoice.amount) || 0)).toFixed(2));

// Greedy oldest-first fill, mirroring allocateOldestFirst in Helpers/SupplierDues.js so the
// form's preview matches what the server will actually do. Duplicated by hand rather than
// shared, following the precedent set by BatchReceiveFormCard's computeAutoPreview.
export const previewAutoAllocation = (openInvoices, amount) => {
    let remaining = Number(amount) || 0;
    const result = [];
    for (const invoice of openInvoices) {
        if (remaining <= 0) break;
        const due = invoiceDue(invoice);
        if (due <= 0) continue;
        const applied = Math.min(due, remaining);
        result.push({ ...invoice, applied: Number(applied.toFixed(2)) });
        remaining = Number((remaining - applied).toFixed(2));
    }
    return { allocations: result, remaining: Number(Math.max(0, remaining).toFixed(2)) };
};

// The server rejects a payment that can't be fully allocated, so the form should say why
// before it's submitted rather than after.
export const allocationError = (allocated, amount) => {
    const total = Number(amount) || 0;
    const sum = Number(allocated) || 0;
    if (total <= 0) return "Enter an amount greater than 0.";
    if (sum > total + 0.01) return `Allocated ₹${RoundOff(sum)} is more than the ₹${RoundOff(total)} being paid.`;
    if (sum < total - 0.01) return `₹${RoundOff(total - sum)} of this payment is still unallocated.`;
    return "";
};
