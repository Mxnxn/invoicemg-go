// What state a purchase order is in, in the words the UI uses for it.
//
// Converted outranks approved. Once an order has become a purchase invoice, whether it was
// approved is history - showing "Approved" on it invites someone to send or re-order against
// work that has already been billed.
//
// `status` values come from Common/DataTable/StatusBadge: only the ones with a rule in
// dataTable.css are coloured, anything else renders as a bare pill.
//
// Lives here rather than in the list, because the detail panel says the same thing about the
// same order - and a panel that opened out of a card marked "Approved" and then said nothing
// looked like the information had been lost on the way in.
export const poState = (po) => {
    if (!po) return { label: "", status: "neutral" };
    if (po.purchaseInvoice_id) return { label: "Invoiced", status: "neutral" };
    if (po.approval?.state === "approved") return { label: "Approved", status: "green" };
    return { label: "Draft", status: "amber" };
};

export default poState;
