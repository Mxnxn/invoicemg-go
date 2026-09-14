import React, { useCallback, useEffect, useState } from "react";
import { Row, Col } from "reactstrap";
import { Trash2, Search } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import ReceiptDestinations from "../../../Common/receipts/ReceiptDestinations";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { supplierPaymentBackend } from "../supplier_payment_backend";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { notifyError } from "../../../global/toast";
import { useUndoDelete } from "../../../Common/undoDelete";
import SupplierPaymentFormCard from "./SupplierPaymentFormCard";
import MonthRangePicker from "../../../Common/MonthRangePicker";
import { filterByMonthRange, EMPTY_MONTH_RANGE } from "../../../Common/monthRange";
import SearchField from "../../../Common/SearchField";
import Blank from "../../../Common/DataTable/Blank";

// Bank Transfers > Paid - the form beside a list of every payment made to suppliers. Mirrors
// BatchReceiveForm's layout so the two tabs read as one screen in two directions.
const SupplierPaymentForm = () => {
    const [payments, setPayments] = useState([]);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();
    const [search, setSearch] = useState("");
    const [monthRange, setMonthRange] = useState(EMPTY_MONTH_RANGE);

    const getPayments = useCallback(() => {
        supplierPaymentBackend.listPayments().then((res) => setPayments(res.data));
    }, []);

    useEffect(() => {
        getPayments();
    }, [getPayments]);

    const confirmDelete = () => {
        if (!deleteTarget) return;
        const removed = payments.find((p) => p._id === deleteTarget);
        const index = payments.findIndex((p) => p._id === deleteTarget);
        setPayments((prev) => prev.filter((p) => p._id !== deleteTarget));
        setDeleteTarget(null);
        scheduleDelete({
            label: "payment",
            commit: () => {
                const formData = new FormData();
                formData.set("payment_id", deleteTarget);
                return supplierPaymentBackend.deletePayment(formData);
            },
            undo: () => setPayments((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
        });
    };

    const visiblePayments = filterByMonthRange(payments, monthRange).filter((p) => {
        if (!search) return true;
        const needle = search.toUpperCase();
        return (
            (p.supplier_id?.firm || "").toUpperCase().includes(needle) ||
            (p.supplier_id?.name || "").toUpperCase().includes(needle) ||
            (p.bank_id?.name || "").toUpperCase().includes(needle) ||
            (p.note || "").toUpperCase().includes(needle) ||
            (p.allocations || []).some((a) => (a.purchase_invoice_id?.invoiceNumber || "").toUpperCase().includes(needle))
        );
    });

    return (
        <>
            <Row className="mt-2">
                <Col xl="6">
                    <SupplierPaymentFormCard onCreated={(payment) => setPayments((prev) => [payment, ...prev])} />
                </Col>
                <Col xl="6">
                    <div className="shell-card">
                        <div className="shell-card-header" style={{ flexWrap: "wrap", gap: 12 }}>
                            <SearchField style={{ flex: 1, minWidth: 200 }} placeholder="Search supplier, bank, invoice, or reference..." value={search} onChange={(e) => setSearch(e.target.value)} />
                            <MonthRangePicker value={monthRange} onChange={setMonthRange} />
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Date</th>
                                    <th scope="col">Supplier</th>
                                    <th scope="col">Bank</th>
                                    <th scope="col">Amount</th>
                                    <th scope="col">Against</th>
                                    <th scope="col" />
                                </tr>
                            </thead>
                            <tbody>
                                {visiblePayments.length === 0 ? (
                                    <tr>
                                        <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                            No payments recorded yet.
                                        </td>
                                    </tr>
                                ) : (
                                    visiblePayments.map((payment, index) => (
                                        <tr key={payment._id}>
                                            <td className="cell-mono">{payment.date}</td>
                                            <td>{payment.supplier_id?.firm || payment.supplier_id?.name || <Blank />}</td>
                                            <td>{payment.bank_id?.name || <Blank />}</td>
                                            <td className="cell-mono">₹{RoundOff(payment.amount)}</td>
                                            {/* Was a comma-joined string of invoice numbers, which lost how much
                                                of the payment reached each one - the part that matters when a
                                                single payment is split across several bills. */}
                                            <td>
                                                <ReceiptDestinations destinations={payment.destinations} />
                                            </td>
                                            <td>
                                                <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                                    <RowActionMenuItem
                                                        icon={Trash2}
                                                        variant="danger"
                                                        onClick={() => {
                                                            setDeleteTarget(payment._id);
                                                            setMoreMenu(-1);
                                                        }}
                                                    >
                                                        Delete
                                                    </RowActionMenuItem>
                                                </RowActionMenu>
                                            </td>
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </DataTable>
                    </div>
                </Col>
            </Row>

            <ConfirmDialog
                open={Boolean(deleteTarget)}
                message="Delete this payment? The invoices it was allocated to will go back to showing the full amount outstanding."
                confirmLabel="Delete"
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default SupplierPaymentForm;
