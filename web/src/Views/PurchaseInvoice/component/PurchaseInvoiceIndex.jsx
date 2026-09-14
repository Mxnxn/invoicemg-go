import React, { useCallback, useEffect, useState } from "react";
import { Container, Row, Col, Button } from "reactstrap";
import { Plus, Search, Trash2, Edit } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import NumberBadge from "../../../Common/DataTable/NumberBadge";
import PurchaseInvoiceDetailModal from "./PurchaseInvoiceDetailModal";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { getDateForEntry } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { purchaseInvoiceBackend } from "../purchase_invoice_backend";
import { purchaseInvoiceGrandTotal } from "../purchaseInvoiceMath";
import CreatePurchaseInvoiceModal from "./CreatePurchaseInvoiceModal";
import { notifyError } from "../../../global/toast";
import { useUndoDelete } from "../../../Common/undoDelete";
import MonthRangePicker from "../../../Common/MonthRangePicker";
import { filterByMonthRange, EMPTY_MONTH_RANGE } from "../../../Common/monthRange";
import SearchField from "../../../Common/SearchField";
import DocumentCard from "../../../Common/cards/DocumentCard";
import ViewToggle, { VIEW_BOARD, readView } from "../../../Common/cards/ViewToggle";

const PurchaseInvoiceIndex = () => {
    const [invoices, setInvoices] = useState([]);
    const [loading, setLoading] = useState(true);
    // The invoice being read, if any. Read-only - editing is still the row menu.
    const [detail, setDetail] = useState(null);
    const [view, setView] = useState(() => readView("purchase-invoices"));
    const [search, setSearch] = useState("");
    const [monthRange, setMonthRange] = useState(EMPTY_MONTH_RANGE);
    const [createModal, setCreateModal] = useState(false);
    const [editingInvoice, setEditingInvoice] = useState(null);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();

    const getInvoices = useCallback(() => {
        setLoading(true);
        purchaseInvoiceBackend
            .listInvoices()
            .then((res) => setInvoices(res.data))
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        getInvoices();
    }, [getInvoices]);

    const filtered = filterByMonthRange(invoices, monthRange).filter((inv) => {
        if (!search) return true;
        const needle = search.toUpperCase();
        const supplierName = inv.supplier_id?.firm || inv.supplier_id?.name || "";
        const supplierPhone = inv.supplier_id?.phone || "";
        return (
            inv.invoiceNumber.toUpperCase().includes(needle) ||
            supplierName.toUpperCase().includes(needle) ||
            supplierPhone.includes(search)
        );
    });

    // Honours Account settings > Appearance > Rows per page, like every other list.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

    const onCreate = (formData) =>
        purchaseInvoiceBackend.createInvoice(formData).then((res) => {
            setInvoices((prev) => [res.data, ...prev]);
        });

    const onUpdate = (formData) =>
        purchaseInvoiceBackend.updateInvoice(formData).then((res) => {
            setInvoices((prev) => prev.map((inv) => (inv._id === res.data._id ? res.data : inv)));
        });

    const onDelete = (id) => {
        const removed = invoices.find((inv) => inv._id === id);
        const index = invoices.findIndex((inv) => inv._id === id);
        setInvoices((prev) => prev.filter((inv) => inv._id !== id));
        setMoreMenu(-1);
        setDeleteTarget(null);
        scheduleDelete({
            label: "purchase invoice",
            commit: () => {
                const formData = new FormData();
                formData.set("purchase_invoice_id", id);
                return purchaseInvoiceBackend.deleteInvoice(formData);
            },
            undo: () => setInvoices((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
        });
    };

    return (
        <>
            <Container fluid>
                <Row className="mt-2">
                    <Col xl="12">
                        <div className={`shell-card${view === VIEW_BOARD ? " is-deck" : ""}`}>
                            <div className="shell-card-header">
                                <ViewToggle view={view} onChange={setView} storageKey="purchase-invoices" />
                                <div className="list-header-actions">
                                    <div className="list-header-group">
                                        <MonthRangePicker value={monthRange} onChange={setMonthRange} />
                                        <SearchField
                                            className="list-header-search"
                                            placeholder="Search invoice number, supplier, phone"
                                            value={search}
                                            onChange={(e) => setSearch(e.target.value)}
                                        />
                                        <Button
                                            className="shell-btn shell-btn-primary d-flex align-items-center invoice-create-btn"
                                            style={{ gap: 6 }}
                                            onClick={() => {
                                                setEditingInvoice(null);
                                                setCreateModal(true);
                                            }}
                                        >
                                            <Plus size={15} />
                                            Create Purchase Invoice
                                        </Button>
                                    </div>
                                </div>
                            </div>
                            {view === VIEW_BOARD ? (
                                loading ? (
                                    <p className="doc-deck-note text-body-small">Loading...</p>
                                ) : filtered.length === 0 ? (
                                    <p className="doc-deck-note text-body-small">
                                        {search ? "No purchase invoices match that search." : "No purchase invoices yet."}
                                    </p>
                                ) : (
                                    <div className="doc-card-deck doc-deck-padded">
                                        {pageRows.map((inv) => (
                                            <DocumentCard
                                                key={inv._id}
                                                number={inv.invoiceNumber}
                                                party={inv.supplier_id?.firm || inv.supplier_id?.name}
                                                date={getDateForEntry(inv.date)}
                                                rows={inv.rows}
                                                figures={[
                                                    {
                                                        label: "Total",
                                                        value: `\u20b9${RoundOff(inv.total ?? purchaseInvoiceGrandTotal(inv.rows))}`,
                                                    },
                                                    { label: "Rows", value: inv.rows?.length || 0 },
                                                ]}
                                                onOpen={() => setDetail(inv)}
                                            />
                                        ))}
                                    </div>
                                )
                            ) : (
                            <DataTable loading={loading}>
                                <thead>
                                    <tr>
                                        <th scope="col">Invoice #</th>
                                        <th scope="col">Date</th>
                                        <th scope="col">Supplier</th>
                                        <th scope="col">Rows</th>
                                        <th scope="col">Grand Total</th>
                                        <th scope="col">Action</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {loading ? (
                                        <tr>
                                            <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                                Loading…
                                            </td>
                                        </tr>
                                    ) : filtered.length === 0 ? (
                                        <tr>
                                            <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                                No purchase invoices yet.
                                            </td>
                                        </tr>
                                    ) : (
                                        pageRows.map((inv, idx) => (
                                            <tr key={inv._id}>
                                                <td>
                                                    {/* The number opens the invoice, the way a
                                                        job-id does. Reading a bill used to mean
                                                        opening the form that edits it. */}
                                                    <NumberBadge
                                                        accent
                                                        onClick={() => setDetail(inv)}
                                                        title="Open this purchase invoice"
                                                    >
                                                        {inv.invoiceNumber}
                                                    </NumberBadge>
                                                </td>
                                                <td className="cell-mono">{getDateForEntry(inv.date)}</td>
                                                <td>{inv.supplier_id?.firm || inv.supplier_id?.name}</td>
                                                <td className="cell-mono">{inv.rows?.length || 0}</td>
                                                <td className="cell-mono">₹{RoundOff(inv.total ?? purchaseInvoiceGrandTotal(inv.rows))}</td>
                                                <td>
                                                    <RowActionMenu open={moreMenu === idx} onOpenChange={(next) => setMoreMenu(next ? idx : -1)}>
                                                        <RowActionMenuItem
                                                            icon={Edit}
                                                            variant="warning"
                                                            onClick={() => {
                                                                setEditingInvoice(inv);
                                                                setCreateModal(true);
                                                                setMoreMenu(-1);
                                                            }}
                                                        >
                                                            Edit
                                                        </RowActionMenuItem>
                                                        <RowActionMenuItem
                                                            icon={Trash2}
                                                            variant="danger"
                                                            onClick={() => {
                                                                setDeleteTarget(inv._id);
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
                            )}
                            <div className="table-foot">
                                <span className="text-body-small table-foot-count">
                                    {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} invoices` : `${total} invoices`}
                                </span>
                                <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                            </div>
                        </div>
                    </Col>
                </Row>
            </Container>

            <PurchaseInvoiceDetailModal invoice={detail} isOpen={detail !== null} toggle={() => setDetail(null)} />

            <CreatePurchaseInvoiceModal
                isOpen={createModal}
                toggle={() => {
                    setCreateModal(false);
                    setEditingInvoice(null);
                }}
                onCreate={onCreate}
                onUpdate={onUpdate}
                editingInvoice={editingInvoice}
            />
            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this purchase invoice? This can't be undone."
                confirmLabel="Delete"
                position="bottom"
                danger
                onConfirm={() => onDelete(deleteTarget)}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default PurchaseInvoiceIndex;
