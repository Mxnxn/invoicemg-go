import InvoiceDoc from "./InvoiceDoc";
import { DESIGNS, resolveDesignKey } from "../../../Common/pdf/designs";

// Selectable per-Company in Configure > Templates, stored as Company.invoiceTemplate.
//
// The list is the shared design set (Common/pdf/designs.js), so the Invoice, Quotation and
// Client Ledger tabs offer the same names - that is the point of it. There is one component
// behind every entry now; the design key is what differs, not the component.
export const INVOICE_TEMPLATES = DESIGNS.map(({ key, label, blurb }) => ({
    key,
    label,
    blurb,
    Component: (props) => <InvoiceDoc {...props} designKey={key} />,
}));

export const getInvoiceTemplate = (key) => {
    const resolved = resolveDesignKey(key);
    return (props) => <InvoiceDoc {...props} designKey={resolved} />;
};
