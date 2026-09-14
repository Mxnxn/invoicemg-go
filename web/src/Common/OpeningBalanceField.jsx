import React from "react";
import { Input } from "reactstrap";

// Opening balance, for a business migrating from other software.
//
// Stored as one signed number, but never SHOWN as one: "-2500" tells you nothing about who is
// out of pocket, and a minus sign typed into the wrong field is a silent, expensive mistake.
// The amount is always entered positive and the direction is picked in words.
//
// The words differ by side, which is the point:
//   customer  - "They owe us" (we are owed) / "We owe them" (they are in credit)
//   supplier  - "We owe them" (we are behind) / "They owe us" (we paid ahead)
//   bank      - "In the account" / "Overdrawn"
//
// `side` is "customer" | "supplier" | "bank". The sign convention follows each model: for a customer,
// positive means they owe us; for a supplier, positive means we owe them.
const LABELS = {
    customer: {
        positive: "They owe us",
        negative: "We owe them",
        hint: "What this customer already owed you when you moved to InvoiceMG.",
    },
    supplier: {
        positive: "We owe them",
        negative: "They owe us",
        hint: "What you already owed this supplier when you moved to InvoiceMG.",
    },
    // A bank has no counterparty, so "owes" makes no sense here: the question is simply how
    // much was in the account, and whether it was overdrawn.
    bank: {
        positive: "In the account",
        negative: "Overdrawn",
        hint: "What this account held when you moved to InvoiceMG. The Bank Report opens from it.",
    },
};

const OpeningBalanceField = ({ value = 0, onChange, side = "customer", label = "Opening balance" }) => {
    const words = LABELS[side] || LABELS.customer;
    const amount = Math.abs(Number(value) || 0);
    const isPositive = (Number(value) || 0) >= 0;

    const emit = (nextAmount, nextPositive) => {
        const magnitude = Math.abs(Number(nextAmount) || 0);
        onChange(nextPositive ? magnitude : -magnitude);
    };

    return (
        <div className="opening-balance">
            <label className="form-control-label pp fs-12">
                {label} <span style={{ color: "var(--text-tertiary)" }}>· optional</span>
            </label>
            <div className="opening-balance-row">
                <Input
                    type="number"
                    step="0.01"
                    min="0"
                    placeholder="0.00"
                    value={amount === 0 ? "" : amount}
                    onChange={(e) => emit(e.target.value, isPositive)}
                />
                <div className="segmented" role="radiogroup" aria-label="Which way the balance goes">
                    <button
                        type="button"
                        role="radio"
                        aria-checked={isPositive}
                        className={["segmented-option", isPositive ? "active" : ""].filter(Boolean).join(" ")}
                        onClick={() => emit(amount, true)}
                    >
                        {words.positive}
                    </button>
                    <button
                        type="button"
                        role="radio"
                        aria-checked={!isPositive}
                        className={["segmented-option", !isPositive ? "active" : ""].filter(Boolean).join(" ")}
                        onClick={() => emit(amount, false)}
                    >
                        {words.negative}
                    </button>
                </div>
            </div>
            <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                {words.hint}
            </span>
        </div>
    );
};

export default OpeningBalanceField;
