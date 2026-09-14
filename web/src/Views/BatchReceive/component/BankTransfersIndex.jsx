import React, { useState } from "react";
import LiteHeader from "../../../Common/Header/LiteHeader";
import BatchReceiveForm from "./BatchReceiveForm";
import SupplierPaymentForm from "../../SupplierPayment/component/SupplierPaymentForm";
import ExpenseForm from "../../BankReport/component/ExpenseForm";

// Used to live as a card inside the "More" hub (/admin/more/batch-receive) - pulled out to
// its own top-level sidebar item and renamed Bank Transfers.
//
// Three tabs, one per kind of bank movement: Received (customer receipts, allocated to
// jobs), Paid (supplier payments, allocated to purchase invoices) and Expense (money out
// that is not a supplier payment - rent, salaries, fuel). They're backed by entirely
// separate collections - BatchReceive, SupplierPayment and Expense - and share no state; the
// tabs only put every direction in the one place a user thinks of as "the bank".
const TABS = [
    { id: "received", label: "Received", body: BatchReceiveForm },
    { id: "paid", label: "Paid", body: SupplierPaymentForm },
    { id: "expense", label: "Expense", body: ExpenseForm },
];

const BankTransfersIndex = () => {
    const [tab, setTab] = useState("received");
    const ActiveBody = TABS.find((t) => t.id === tab).body;

    return (
        <>
            <LiteHeader bg="primary" />
            <div style={{ padding: "20px 24px" }}>
                {/* A segmented control rather than two buttons: these are two views of one
                    thing, not two actions. Sized like a half-width field so it lines up with
                    the form beneath instead of floating at content width. */}
                <div className="segmented segmented--field" role="tablist" aria-label="Bank movement">
                    {TABS.map((t) => (
                        <button
                            key={t.id}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.id}
                            className={["segmented-option", tab === t.id ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setTab(t.id)}
                        >
                            <span>{t.label}</span>
                        </button>
                    ))}
                </div>
                <ActiveBody />
            </div>
        </>
    );
};

export default BankTransfersIndex;
