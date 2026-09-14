import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { AlertTriangle, Edit, Trash2 } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import SearchField from "../../../Common/SearchField";
import usePagedRows from "../../../Common/useRowsPerPage";
import Pagination from "../../../Shell/Pagination";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { sharedBackend } from "../shared_backend";
import { materialsBackend } from "../../Material/material_backend";
import { notifySuccess, notifyError } from "../../../global/toast";
import "./sharedRecords.css";
import Blank from "../../../Common/DataTable/Blank";

// The same product entered separately in more than one company.
//
// This is the report the sharing design asks for FIRST, before anything is merged: the unique
// index on Client and the per-company Material records mean duplicates already exist, each
// with its own rate and its own history. Merging them is irreversible, so the table shows
// them and lets a person decide, one at a time.
//
// Grouped on the normalised name server-side, so "Art Card 300gsm" and "art card 300 gsm"
// land in one group - which is exactly the case worth seeing.
//
// PRICES ARE NOT RECONCILED. Two companies selling one product at two rates is a legitimate
// business fact as often as it is a mistake, so the mismatch is flagged and left alone.
const money = (n) => `₹${RoundOff(n)}`;

const DuplicateProducts = ({ onChanged }) => {
    const navigate = useNavigate();
    const [groups, setGroups] = useState([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState("");
    const [menuFor, setMenuFor] = useState(null);
    const [confirm, setConfirm] = useState(null);
    const [reloads, setReloads] = useState(0);

    useEffect(() => {
        let live = true;
        setLoading(true);
        sharedBackend
            .duplicateMaterials()
            .then((res) => {
                if (live) setGroups(res.data?.rows || []);
            })
            .catch(() => {})
            .finally(() => {
                if (live) setLoading(false);
            });
        return () => {
            live = false;
        };
    }, [reloads]);

    const filtered = useMemo(() => {
        const term = search.trim().toLowerCase();
        if (!term) return groups;
        return groups.filter(
            (g) =>
                g.name.toLowerCase().includes(term) ||
                g.companies.some((c) => c.toLowerCase().includes(term))
        );
    }, [groups, search]);

    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

    // Deleting one copy is the safe half of "merge": the other copies stay, and every job-id
    // and invoice that named this product keeps its own string snapshot, so nothing already
    // billed changes. That is why this is offered here at all - see Model/PurchaseInvoice.js.
    const removeCopy = async (copy) => {
        try {
            const formData = new FormData();
            formData.set("material_id", copy._id);
            await materialsBackend.deleteMaterial(formData, window.localStorage.getItem("session_token"));
            notifySuccess(`Removed the ${copy.companyName} copy.`);
            setReloads((n) => n + 1);
            onChanged?.();
        } catch (error) {
            notifyError(error?.message || "Could not remove that copy.");
        } finally {
            setConfirm(null);
        }
    };

    return (
        <div className="shared-dupes-panel">
            <div className="shared-section-head">
                <div>
                    <h3 className="text-heading-brand shared-section-title">Duplicate products</h3>
                    <p className="text-body-small shared-section-sub">
                        The same product entered in more than one company. Each copy keeps its own rate and its own
                        history — nothing here is merged automatically.
                    </p>
                </div>
                <SearchField
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search product or company"
                />
            </div>

            {loading ? (
                <p className="shared-note text-body-small">Loading…</p>
            ) : total === 0 ? (
                <p className="shared-note text-body-small">
                    {search ? "Nothing matches that search." : "No product is entered twice — nothing to reconcile."}
                </p>
            ) : (
                <>
                    <DataTable loading={loading}>
                        <thead>
                            <tr>
                                <th>Product</th>
                                <th>Company</th>
                                <th>Unit</th>
                                <th>HSN</th>
                                <th className="text-right">Sell rate</th>
                                <th className="text-right">Buy rate</th>
                                <th aria-label="Actions" />
                            </tr>
                        </thead>
                        <tbody>
                            {pageRows.map((group) =>
                                group.copies.map((copy, index) => (
                                    <tr key={copy._id} className={index === 0 ? "shared-group-start" : undefined}>
                                        {/* The name spans its copies rather than repeating on
                                            every line - the whole point is to read them as one
                                            product seen from several companies. */}
                                        {index === 0 ? (
                                            <td rowSpan={group.copies.length} className="shared-group-name">
                                                {group.name}
                                                <span className="shared-group-count text-body-small">
                                                    {group.copies.length} copies
                                                </span>
                                                {group.priceMismatch && (
                                                    <span className="shared-mismatch text-body-small">
                                                        <AlertTriangle size={12} aria-hidden="true" />
                                                        {money(group.lowestRate)} – {money(group.highestRate)}
                                                    </span>
                                                )}
                                            </td>
                                        ) : null}
                                        <td className="shared-company">{copy.companyName || <Blank />}</td>
                                        <td>{copy.unit || <span className="shared-missing">not set</span>}</td>
                                        <td className="cell-mono">{copy.hsn || <Blank />}</td>
                                        <td
                                            className={`cell-mono text-right${
                                                group.priceMismatch ? " shared-rate-flagged" : ""
                                            }`}
                                        >
                                            {money(copy.material_rate)}
                                        </td>
                                        <td className="cell-mono text-right">{money(copy.purchase_rate)}</td>
                                        <td className="shared-actions">
                                            <RowActionMenu
                                                open={menuFor === copy._id}
                                                onOpenChange={(next) => setMenuFor(next ? copy._id : null)}
                                            >
                                                <RowActionMenuItem
                                                    icon={Edit}
                                                    variant="warning"
                                                    onClick={() => {
                                                        setMenuFor(null);
                                                        navigate("/admin/configure/products");
                                                    }}
                                                >
                                                    Edit in Products
                                                </RowActionMenuItem>
                                                <RowActionMenuItem
                                                    icon={Trash2}
                                                    variant="danger"
                                                    onClick={() => {
                                                        setMenuFor(null);
                                                        setConfirm(copy);
                                                    }}
                                                >
                                                    Remove this copy
                                                </RowActionMenuItem>
                                            </RowActionMenu>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </DataTable>
                    <div className="table-foot">
                        <span className="text-body-small shared-count">
                            {perPage > 0 && total > perPage
                                ? `${pageRows.length} of ${total} duplicated products`
                                : `${total} duplicated ${total === 1 ? "product" : "products"}`}
                        </span>
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                </>
            )}

            {/* Named, not vague: it says which company's copy goes and what survives. Deleting
                a product cannot alter a document that already exists, and saying so is what
                stops this reading as though invoices are at risk. */}
            {confirm && (
                <div className="shared-confirm-backdrop" role="dialog" aria-modal="true">
                    <div className="shared-confirm">
                        <h4 className="shared-confirm-title">Remove {confirm.companyName}’s copy?</h4>
                        <p className="shared-confirm-body text-body-small">
                            This deletes <strong>{confirm.material_name}</strong> from <strong>{confirm.companyName}</strong> only.
                            The other {confirm.companyName ? "companies keep" : "copies keep"} theirs.
                        </p>
                        <p className="shared-confirm-body text-body-small">
                            Existing job-ids, invoices and challans are <strong>not</strong> affected — they store the
                            product name and rate as they were when raised, not a link to this record.
                        </p>
                        <div className="shared-confirm-actions">
                            <button type="button" className="shell-btn" onClick={() => setConfirm(null)}>
                                Cancel
                            </button>
                            <button type="button" className="shell-btn shared-btn-danger" onClick={() => removeCopy(confirm)}>
                                Remove copy
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default DuplicateProducts;
