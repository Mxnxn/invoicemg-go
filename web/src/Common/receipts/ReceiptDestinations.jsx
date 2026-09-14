import React from "react";
import { FileText, Layers, ShoppingCart } from "react-feather";
import { RoundOff } from "../DateAndTime/RoundOff";
import "./receipts.css";

// Where a payment went, rendered as chips under the row it belongs to.
//
// A receipt row on its own reads "₹500 on 26-08-2026", which is the least useful half of the
// record: the question anyone actually has later is *what did this pay for*. Every payment
// model already stores the answer - it must, so deletion can reverse the allocation - and
// Helpers/ReceiptDestinations.js on the API reads it back out as a uniform `destinations`
// array, so one component can render receipts, bank transfers and supplier payments alike.
//
// A transfer that settled three jobs shows three chips with the amount that reached each,
// because "₹500" against three jobs is not the same fact as ₹500 against one.

const ICONS = {
    invoice: FileText,
    job: Layers,
    "purchase-invoice": ShoppingCart,
};

const KIND_LABEL = {
    invoice: "Invoice",
    job: "Job",
    "purchase-invoice": "Purchase invoice",
};

/**
 * @param destinations  [{ kind, id, label, amount }] from the API
 * @param onOpen        optional (destination) => void; makes each chip a button
 * @param showAmounts   split amounts per destination - off when there is only one, where the
 *                      row's own amount already says it
 */
const ReceiptDestinations = ({ destinations = [], onOpen, showAmounts = true }) => {
    if (destinations.length === 0) {
        return (
            <span className="text-body-small receipt-dest-empty">
                Not linked to an invoice or job
            </span>
        );
    }

    const withAmounts = showAmounts && destinations.length > 1;

    return (
        <span className="receipt-dest-list">
            {destinations.map((dest) => {
                const Icon = ICONS[dest.kind] || FileText;
                const title = `${KIND_LABEL[dest.kind] || "Linked to"} ${dest.label}`;
                const body = (
                    <>
                        <Icon size={12} aria-hidden="true" />
                        <span className="receipt-dest-label">{dest.label}</span>
                        {withAmounts && <span className="receipt-dest-amount">₹{RoundOff(dest.amount)}</span>}
                    </>
                );

                // A chip is only a button when the caller can actually open the thing it names -
                // a chip that looks clickable and does nothing is worse than a plain label.
                return onOpen ? (
                    <button
                        key={`${dest.kind}-${dest.id}`}
                        type="button"
                        className="receipt-dest receipt-dest-btn"
                        title={title}
                        onClick={(evt) => {
                            evt.stopPropagation();
                            onOpen(dest);
                        }}
                    >
                        {body}
                    </button>
                ) : (
                    <span key={`${dest.kind}-${dest.id}`} className="receipt-dest" title={title}>
                        {body}
                    </span>
                );
            })}
        </span>
    );
};

export default ReceiptDestinations;
