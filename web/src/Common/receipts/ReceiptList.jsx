import React from "react";
import { getDate } from "../DateAndTime/getDate";
import { RoundOff } from "../DateAndTime/RoundOff";
import DataTable from "../DataTable/DataTable";
import StatusBadge from "../DataTable/StatusBadge";
import ReceiptDestinations from "./ReceiptDestinations";
import "./receipts.css";

// One table for every list of money that moved: invoice receipts, bank transfers in, and
// supplier payments out.
//
// They were three separate tables that each showed a date and an amount and nothing about
// where the money went, so the same question ("what did this pay for?") was unanswerable in
// three places. The `destinations` array the API now returns is identical across all three,
// so this renders any of them.
//
// `source` distinguishes how the money arrived, which matters on an invoice: closing an
// invoice writes an InvoiceReceived, while paying through Bank Transfers cascades down from a
// job and writes no receipt of its own. Both settle the invoice; only one used to be visible.

const SOURCE_LABEL = {
    invoice: "Receipt",
    "bank-transfer": "Bank transfer",
    supplier: "Payment",
};

const ReceiptList = ({
    rows = [],
    emptyMessage = "Nothing recorded yet.",
    onOpenDestination,
    showSource = true,
    amountKey = "appliedAmount",
    actionsFor,
}) => {
    if (rows.length === 0) {
        return (
            <p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0, padding: "12px 2px" }}>
                {emptyMessage}
            </p>
        );
    }

    return (
        <DataTable>
            <thead>
                <tr>
                    <th scope="col">Date</th>
                    <th scope="col">Amount</th>
                    {showSource && <th scope="col">Via</th>}
                    <th scope="col">Applied to</th>
                    {actionsFor && <th scope="col" aria-label="Actions" />}
                </tr>
            </thead>
            <tbody>
                {rows.map((row, index) => {
                    // appliedAmount is what reached the thing being looked at, which is not the
                    // same as what was banked when one transfer spans several invoices.
                    const amount = row[amountKey] !== undefined ? row[amountKey] : row.amount;
                    return (
                        <tr key={row._id || index}>
                            <td className="cell-mono">{getDate(row.date || row.createdAt)}</td>
                            <td className="cell-mono">₹{RoundOff(amount)}</td>
                            {showSource && (
                                <td>
                                    <StatusBadge status={row.source === "bank-transfer" ? "pending" : "paid"}>
                                        {SOURCE_LABEL[row.source] || "Receipt"}
                                    </StatusBadge>
                                    {row.bank_id?.name && (
                                        <span className="text-body-small" style={{ color: "var(--text-tertiary)", marginLeft: 6 }}>
                                            {row.bank_id.name}
                                        </span>
                                    )}
                                </td>
                            )}
                            <td>
                                <ReceiptDestinations destinations={row.destinations} onOpen={onOpenDestination} />
                                {row.note && (
                                    <div className="text-body-small" style={{ color: "var(--text-tertiary)", marginTop: 4 }}>
                                        {row.note}
                                    </div>
                                )}
                            </td>
                            {actionsFor && <td>{actionsFor(row, index)}</td>}
                        </tr>
                    );
                })}
            </tbody>
        </DataTable>
    );
};

export default ReceiptList;
