import React, { useCallback, useEffect, useState } from "react";
import { Container, Row, Col, Button } from "reactstrap";
import { Plus, Search, Trash2, Eye } from "react-feather";
import LiteHeader from "../../Common/Header/LiteHeader";
import DataTable from "../../Common/DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "../../Common/DataTable/RowActionMenu";
import { getDateForEntry } from "../../Common/DateAndTime/getDate";
import { RoundOff } from "../../Common/DateAndTime/RoundOff";
import { quotationBackend } from "./quotation_backend";
import { quotationGrandTotal } from "./quotationMath";
import CreateQuotationModal from "./CreateQuotationModal";
import QuotationDetailModal from "./QuotationDetailModal";
import QuotationPreviewModal from "./QuotationPreviewModal";
import { notifyError } from "../../global/toast";
import ConfirmDialog from "../../Common/ConfirmDialog";
import { useUndoDelete } from "../../Common/undoDelete";
import SearchField from "../../Common/SearchField";
import DocumentCard from "../../Common/cards/DocumentCard";
import ViewToggle, { VIEW_BOARD, readView } from "../../Common/cards/ViewToggle";

const QuotationsIndex = ({ user }) => {
    const [quotations, setQuotations] = useState([]);
    const [loading, setLoading] = useState(true);
    const [view, setView] = useState(() => readView("quotations"));
    const [search, setSearch] = useState("");
    const [createModal, setCreateModal] = useState(false);
    const [activeQuotation, setActiveQuotation] = useState(null);
    const [previewQuotation, setPreviewQuotation] = useState(null);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();

    const getQuotations = useCallback(() => {
        setLoading(true);
        quotationBackend
            .listQuotations()
            .then((res) => setQuotations(res.data))
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        getQuotations();
    }, [getQuotations]);

    const filtered = quotations.filter((q) => {
        if (!search) return true;
        const needle = search.toUpperCase();
        const firm = q.client_id?.clientFirm || "";
        return firm.toUpperCase().includes(needle) || q.quotationNumber.toUpperCase().includes(needle);
    });

    const onCreateQuotation = (formData) => {
        return quotationBackend.createQuotation(formData).then((res) => {
            setQuotations((prev) => [res.data, ...prev]);
            setPreviewQuotation(res.data);
        });
    };

    const onDeleteQuotation = (id) => {
        const removed = quotations.find((q) => q._id === id);
        const index = quotations.findIndex((q) => q._id === id);
        setQuotations((prev) => prev.filter((q) => q._id !== id));
        setMoreMenu(-1);
        setDeleteTarget(null);
        scheduleDelete({
            label: "quotation",
            commit: () => {
                const formData = new FormData();
                formData.set("quotation_id", id);
                return quotationBackend.deleteQuotation(formData);
            },
            undo: () => setQuotations((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
        });
    };

    const onQuotationChanged = (updated) => {
        setQuotations((prev) => prev.map((q) => (q._id === updated._id ? updated : q)));
        setActiveQuotation(updated);
    };

    return (
        <>
            <LiteHeader bg="primary" />
            <Container fluid>
                <Row className="mt-2">
                    <Col xl="12">
                        <div className={`shell-card${view === VIEW_BOARD ? " is-deck" : ""}`}>
                            <div className="shell-card-header">
                                <ViewToggle view={view} onChange={setView} storageKey="quotations" />
                                {/* Search and Create together on the right, as on every other
                                    list. The search was flex:1, so it took the whole middle and
                                    the two ended up at opposite edges of the card. */}
                                <div className="list-header-actions">
                                    <div className="list-header-group">
                                        <SearchField
                                            className="list-header-search"
                                            placeholder="Company or Quotation Number"
                                            value={search}
                                            onChange={(e) => setSearch(e.target.value)}
                                        />
                                        <Button
                                            className="shell-btn shell-btn-primary d-flex align-items-center invoice-create-btn"
                                            style={{ gap: 6 }}
                                            onClick={() => setCreateModal(true)}
                                        >
                                            <Plus size={15} />
                                            Create Quotation
                                        </Button>
                                    </div>
                                </div>
                            </div>
                            {view === VIEW_BOARD ? (
                                loading ? (
                                    <p className="doc-deck-note text-body-small">Loading...</p>
                                ) : filtered.length === 0 ? (
                                    <p className="doc-deck-note text-body-small">
                                        {search ? "No quotations match that search." : "No quotations yet."}
                                    </p>
                                ) : (
                                    <div className="doc-card-deck doc-deck-padded">
                                        {filtered.map((q) => (
                                            <DocumentCard
                                                key={q._id}
                                                number={q.quotationNumber}
                                                party={q.client_id?.clientFirm || q.client_id?.clientName}
                                                date={getDateForEntry(q.date)}
                                                rows={q.rows}
                                                figures={[
                                                    { label: "Total", value: `\u20b9${RoundOff(quotationGrandTotal(q.rows))}` },
                                                    { label: "Rows", value: q.rows?.length || 0 },
                                                ]}
                                                onOpen={() => setActiveQuotation(q)}
                                            />
                                        ))}
                                    </div>
                                )
                            ) : (
                            <DataTable loading={loading}>
                                <thead>
                                    <tr>
                                        <th scope="col">Quotation #</th>
                                        <th scope="col">Date</th>
                                        <th scope="col">Client</th>
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
                                                No quotations yet.
                                            </td>
                                        </tr>
                                    ) : (
                                        filtered.map((q, idx) => (
                                            <tr key={q._id} style={{ cursor: "pointer" }} onClick={() => setActiveQuotation(q)}>
                                                <td className="cell-mono">
                                                    <button type="button" className="lifecycle-link">
                                                        {q.quotationNumber}
                                                    </button>
                                                </td>
                                                <td className="cell-mono">{getDateForEntry(q.date)}</td>
                                                <td>{q.client_id?.clientFirm || q.client_id?.clientName}</td>
                                                <td className="cell-mono">{q.rows?.length || 0}</td>
                                                <td className="cell-mono">₹{RoundOff(quotationGrandTotal(q.rows))}</td>
                                                <td onClick={(e) => e.stopPropagation()}>
                                                    <RowActionMenu open={moreMenu === idx} onOpenChange={(next) => setMoreMenu(next ? idx : -1)}>
                                                        <RowActionMenuItem
                                                            icon={Eye}
                                                            onClick={() => {
                                                                setPreviewQuotation(q);
                                                                setMoreMenu(-1);
                                                            }}
                                                        >
                                                            Preview
                                                        </RowActionMenuItem>
                                                        <RowActionMenuItem
                                                            icon={Trash2}
                                                            variant="danger"
                                                            onClick={() => {
                                                                setDeleteTarget(q._id);
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
                            <div className="shell-card-footer">
                                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    {quotations.length} quotations
                                </span>
                            </div>
                        </div>
                    </Col>
                </Row>
            </Container>

            <CreateQuotationModal isOpen={createModal} toggle={() => setCreateModal(false)} onCreate={onCreateQuotation} />
            <QuotationDetailModal quotation={activeQuotation} onClose={() => setActiveQuotation(null)} onChange={onQuotationChanged} />
            <QuotationPreviewModal isOpen={!!previewQuotation} toggle={() => setPreviewQuotation(null)} quotation={previewQuotation} user={user} />
            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this quotation?"
                confirmLabel="Yes"
                cancelLabel="No"
                position="bottom"
                danger
                onConfirm={() => onDeleteQuotation(deleteTarget)}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default QuotationsIndex;
