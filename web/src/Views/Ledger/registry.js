import LedgerDoc from "./LedgerDoc";
import { DESIGNS, resolveDesignKey } from "../../Common/pdf/designs";

// Same shared design set as the invoice and the quotation - see
// Views/Invoice/Template/registry.js. Stored as Company.ledgerTemplate.
//
// This list used to hold exactly one entry while the other two held four.
export const LEDGER_TEMPLATES = DESIGNS.map(({ key, label, blurb }) => ({
    key,
    label,
    blurb,
    Component: (props) => <LedgerDoc {...props} designKey={key} />,
}));

export const getLedgerTemplate = (key) => {
    const resolved = resolveDesignKey(key);
    return (props) => <LedgerDoc {...props} designKey={resolved} />;
};
