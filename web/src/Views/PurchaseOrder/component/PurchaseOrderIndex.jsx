import { useCallback, useEffect, useState } from "react";
import { Container, Row, Col, Button } from "reactstrap";
import { useSearchParams } from "react-router-dom";
import { Plus } from "react-feather";

import LiteHeader from "../../../Common/Header/LiteHeader";
import SearchField from "../../../Common/SearchField";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { poState } from "../poState";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import PoDetailSidebar from "./PoDetailSidebar";
import DocumentCard from "../../../Common/cards/DocumentCard";
import PurchaseOrderModal from "./PurchaseOrderModal";
import { purchaseOrderBackend } from "../purchaseOrder_backend";
import { usePendingApprovals } from "../../../Shell/pendingApprovals";
import "./poDetail.css";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;
const supplierName = (po) => po.supplier_id?.firm || po.supplier_id?.name || "Unnamed supplier";

// Shared with the detail panel - see Views/PurchaseOrder/poState.js.
const stateOf = poState;

export default function PurchaseOrderIndex({ uid }) {
    const stoken = window.localStorage.getItem("session_token");
    const [state, setState] = useState({ loading: true, error: "", rows: [] });
    const [search, setSearch] = useState("");
    // { id, rect } - the rect is the card that was pressed, so the panel can expand out of
    // it rather than arrive from the edge of the screen.
    const [openId, setOpenId] = useState(null);

    // Measured at press time from the DOM rather than tracked in state: the deck re-renders
    // on search, on paging and on every change the panel makes, and a stored rectangle would
    // be stale by the time it was used.
    const cardRect = (id) =>
        document.getElementById(`po-card-${id}`)?.getBoundingClientRect() || undefined;
    // null = closed. { order: null } = creating, { order } = editing that one.
    const [editing, setEditing] = useState(null);
    const { refresh: refreshPending } = usePendingApprovals();
    const [searchParams, setSearchParams] = useSearchParams();

    const load = useCallback(() => {
        purchaseOrderBackend
            .list(new FormData(), stoken)
            .then((res) => setState({ loading: false, error: "", rows: res.data || [] }))
            .catch(() => setState({ loading: false, error: "Could not load purchase orders.", rows: [] }));
        // Approving, editing and converting all reload this list and all change the count,
        // so refreshing here covers every one of them without a call per action.
        refreshPending();
    }, [stoken, refreshPending]);

    useEffect(() => {
        load();
    }, [load]);

    // The bell links to /admin/purchase-orders?open=<id>. Consumed once and cleared, so a
    // later close does not leave a stale id in the URL that reopens on refresh.
    useEffect(() => {
        const wanted = searchParams.get("open");
        if (!wanted) return;
        setOpenId(wanted);
        searchParams.delete("open");
        setSearchParams(searchParams, { replace: true });
    }, [searchParams, setSearchParams]);

    // The same three fields the purchase-invoice list searches, so the two screens sitting
    // beside each other behave the same way. Client-side over the loaded list, as there too.
    const filtered = state.rows.filter((po) => {
        if (!search) return true;
        const needle = search.toUpperCase();
        return (
            String(po.poNumber || "").toUpperCase().includes(needle) ||
            supplierName(po).toUpperCase().includes(needle) ||
            String(po.supplier_id?.phone || "").includes(search)
        );
    });

    // Honours Account settings > Appearance > Rows per page, like every other list.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

    return (
        <>
            <LiteHeader bg="primary" />
            <Container fluid>
                <Row className="mt-2">
                    <Col xl="12">
                        {/* is-deck: a page of cards gives up the frame around them. Bordered
                            cards inside a bordered card is a box in a box - the outer edge adds
                            nothing the inner ones are not already drawing, and at twenty orders
                            it reads as a frame round a frame. Same as every other board view. */}
                        <div className="shell-card is-deck">
                            <div className="shell-card-header">
                                <div className="list-header-actions">
                                    <div className="list-header-group">
                                        <SearchField
                                            className="list-header-search is-wide"
                                            placeholder="Search order number, supplier, phone"
                                            value={search}
                                            onChange={(e) => setSearch(e.target.value)}
                                        />
                                        <Button
                                            className="shell-btn shell-btn-primary d-flex align-items-center invoice-create-btn"
                                            style={{ gap: 6 }}
                                            onClick={() => setEditing({ order: null })}
                                        >
                                            <Plus size={15} /> New Purchase Order
                                        </Button>
                                    </div>
                                </div>
                            </div>

                            {state.error ? (
                                <p className="po-state doc-deck-note text-body-regular">{state.error}</p>
                            ) : !state.loading && filtered.length === 0 ? (
                                <p className="po-state doc-deck-note text-body-regular">
                                    {search ? "No purchase orders match that search." : "No purchase orders yet."}
                                </p>
                            ) : (
                                <>
                                    <div className="doc-card-deck doc-deck-padded">
                                        {pageRows.map((po) => {
                                            const badge = stateOf(po);
                                            return (
                                                <DocumentCard
                                                    key={po._id}
                                                    id={`po-card-${po._id}`}
                                                    number={po.poNumber}
                                                    party={supplierName(po)}
                                                    date={po.date}
                                                    rows={po.rows}
                                                    figures={[{ label: "Total", value: money(po.total) }]}
                                                    badges={
                                                        <>
                                                            <StatusBadge status={badge.status}>{badge.label}</StatusBadge>
                                                            {/* "Created" means never sent, which the approval
                                                                badge already implies - showing it too would
                                                                be noise on every card. */}
                                                            {po.send && po.send.status !== "Created" && (
                                                                <StatusBadge status={po.send.status === "Modified" ? "amber" : "neutral"}>
                                                                    {po.send.status}
                                                                </StatusBadge>
                                                            )}
                                                        </>
                                                    }
                                                    onOpen={() => setOpenId({ id: po._id, rect: cardRect(po._id) })}
                                                />
                                            );
                                        })}
                                    </div>
                                    <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                                </>
                            )}
                        </div>
                    </Col>
                </Row>

                {openId && (
                    <PoDetailSidebar
                        poId={openId.id}
                        rect={openId.rect}
                        uid={uid}
                        onClose={() => setOpenId(null)}
                        onChanged={load}
                        onEdit={(order) => setEditing({ order })}
                    />
                )}

                <PurchaseOrderModal
                    isOpen={editing !== null}
                    toggle={() => setEditing(null)}
                    editingOrder={editing?.order || null}
                    onCreate={(fd) => purchaseOrderBackend.create(fd, stoken).then(load)}
                    // Close the detail panel on a successful edit. It holds its own copy of
                    // the order, loaded when it opened, and an edit can change the order out
                    // from under it - an edit that touches price revokes approval server-side,
                    // so a panel left open would still offer "Withdraw approval" for an
                    // approval that no longer exists, and the click would be refused.
                    onUpdate={(fd) =>
                        purchaseOrderBackend.update(fd, stoken).then((res) => {
                            setOpenId(null);
                            load();
                            return res;
                        })
                    }
                />
            </Container>
        </>
    );
}
