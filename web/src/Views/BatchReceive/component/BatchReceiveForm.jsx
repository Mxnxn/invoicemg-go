import React, { useCallback, useEffect, useState } from "react";
import { Row, Col } from "reactstrap";
import { Trash2, Search } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import ReceiptDestinations from "../../../Common/receipts/ReceiptDestinations";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { batchReceiveBackend } from "../batch_receive_backend";
import { getDateForEntry } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { notifyError } from "../../../global/toast";
import { useUndoDelete } from "../../../Common/undoDelete";
import BatchReceiveFormCard from "./BatchReceiveFormCard";
import MonthRangePicker from "../../../Common/MonthRangePicker";
import { filterByMonthRange, EMPTY_MONTH_RANGE } from "../../../Common/monthRange";
import SearchField from "../../../Common/SearchField";
import Blank from "../../../Common/DataTable/Blank";

// Global More > Batch Receive hub - the form (BatchReceiveFormCard) beside a list of every
// batch receive across all clients. The Client page's own batch-receive modal reuses the
// same form card with fixedClient set, see BatchReceiveModal.jsx.
const BatchReceiveForm = () => {
    const [receives, setReceives] = useState([]);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();
    const [search, setSearch] = useState("");
    const [monthRange, setMonthRange] = useState(EMPTY_MONTH_RANGE);

    const getReceives = useCallback(() => {
        batchReceiveBackend.listReceives().then((res) => setReceives(res.data));
    }, []);

    useEffect(() => {
        getReceives();
    }, [getReceives]);

    const confirmDelete = () => {
        if (!deleteTarget) return;
        const removed = receives.find((r) => r._id === deleteTarget);
        const index = receives.findIndex((r) => r._id === deleteTarget);
        setReceives((prev) => prev.filter((r) => r._id !== deleteTarget));
        setDeleteTarget(null);
        scheduleDelete({
            label: "batch receive",
            commit: () => {
                const formData = new FormData();
                formData.set("batch_id", deleteTarget);
                return batchReceiveBackend.deleteReceive(formData);
            },
            // Back where it was, not appended - the list is chronological.
            undo: () => setReceives((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
        });
    };

    // Month range first, then text - the range is the coarser cut.
    const visibleReceives = filterByMonthRange(receives, monthRange).filter((r) => {
        if (!search) return true;
        const needle = search.toUpperCase();
        return (
            (r.client?.clientFirm || "").toUpperCase().includes(needle) ||
            (r.client?.clientName || "").toUpperCase().includes(needle) ||
            (r.client?.clientPhone || "").includes(search) ||
            (r.bank_id?.name || "").toUpperCase().includes(needle) ||
            (r.note || "").toUpperCase().includes(needle)
        );
    });

    return (
        <>
            <Row className="mt-2">
                <Col xl="6">
                    <BatchReceiveFormCard onCreated={(receive) => setReceives((prev) => [receive, ...prev])} />
                </Col>
                <Col xl="6">
                    <div className="shell-card">
                        <div className="shell-card-header" style={{ flexWrap: "wrap", gap: 12 }}>
                            <SearchField style={{ flex: 1, minWidth: 200 }} placeholder="Search client, bank, or reference..." value={search} onChange={(e) => setSearch(e.target.value)} />
                            <MonthRangePicker value={monthRange} onChange={setMonthRange} />
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Date</th>
                                    <th scope="col">Client</th>
                                    <th scope="col">Bank</th>
                                    <th scope="col">Amount</th>
                                    <th scope="col">Applied to</th>
                                    <th scope="col">Mode</th>
                                    <th scope="col">Action</th>
                                </tr>
                            </thead>
                            <tbody>
                                {visibleReceives.length === 0 ? (
                                    <tr>
                                        <td colSpan={7} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                            No batch receives yet.
                                        </td>
                                    </tr>
                                ) : (
                                    visibleReceives.map((r, index) => (
                                        <tr key={r._id}>
                                            <td className="cell-mono">{getDateForEntry(r.date)}</td>
                                            <td>{r.client?.clientFirm || r.client?.clientName || <Blank />}</td>
                                            <td>{r.bank_id?.name || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                            <td className="cell-mono">₹{RoundOff(r.amount)}</td>
                                            {/* The jobs and invoices this transfer settled - stored all along for
                                                /delete to reverse, never shown until now. */}
                                            <td>
                                                <ReceiptDestinations destinations={r.destinations} />
                                            </td>
                                            <td>{r.mode || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                            <td>
                                                <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                                    <RowActionMenuItem
                                                        icon={Trash2}
                                                        variant="danger"
                                                        onClick={() => {
                                                            setDeleteTarget(r._id);
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
                        <div className="shell-card-footer">
                            <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                {visibleReceives.length} batch receives
                            </span>
                        </div>
                    </div>
                </Col>
            </Row>
            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this batch receive? Its allocations will be reversed off the affected jobs."
                confirmLabel="Delete"
                position="bottom"
                danger
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default BatchReceiveForm;
