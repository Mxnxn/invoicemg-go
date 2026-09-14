import QuotationDoc from "./QuotationDoc";
import { DESIGNS, resolveDesignKey } from "../../Common/pdf/designs";

// Same shared design set as the invoice and the ledger - see Views/Invoice/Template/registry.js.
// Stored as Company.quotationTemplate.
export const QUOTATION_TEMPLATES = DESIGNS.map(({ key, label, blurb }) => ({
    key,
    label,
    blurb,
    Component: (props) => <QuotationDoc {...props} designKey={key} />,
}));

export const getQuotationTemplate = (key) => {
    const resolved = resolveDesignKey(key);
    return (props) => <QuotationDoc {...props} designKey={resolved} />;
};
