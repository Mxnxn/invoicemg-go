import React, { useEffect, useMemo, useState } from "react";
import { Share2, Copy } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import SearchField from "../../../Common/SearchField";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { sharedBackend } from "../shared_backend";
import { inventoryBackend } from "../../Inventory/inventory_backend";
import DuplicateProducts from "./DuplicateProducts";
import { notifyError, notifySuccess } from "../../../global/toast";
import "./sharedRecords.css";
import Blank from "../../../Common/DataTable/Blank";

// Configure > Across Company. The single place the cross-company switch lives, plus the two
// lists it governs and the duplicates it surfaces.
//
// It sits in Configure rather than More because it is a SETTING: it changes what every other
// company can see and select, which is not a report. The inventory report and the pickers
// only honour what is decided here.
//
// Sharing is ON by default (see Helpers/SharedRecords.js) - a product or customer entered once
// reaches every company its owner has, which is the point. Turning it OFF is the guarded
// direction, because that is the one that takes access away.
const TABS = [
    { id: "customers", label: "Customers" },
    { id: "materials", label: "Products" },
    { id: "duplicates", label: "Duplicates" },
];

const money = (n) => `₹${RoundOff(n)}`;

const SharedRecordsIndex = () => {
    const isAdmin = (window.localStorage.getItem("role") || "admin") !== "employee";
    const [tab, setTab] = useState("customers");
    const [data, setData] = useState(null);
    const [loading, setLoading] = useState(false);
    const [search, setSearch] = useState("");
    const [sharing, setSharing] = useState(false);
    const [confirmOff, setConfirmOff] = useState(false);
    const [typed, setTyped] = useState("");
    const [toggling, setToggling] = useState(false);
    const [reloads, setReloads] = useState(0);

    const isList = tab !== "duplicates";

    useEffect(() => {
        if (!isList) return undefined;
        let live = true;
        setLoading(true);
        setData(null);
        setSearch("");
        const call = tab === "customers" ? sharedBackend.customers() : sharedBackend.materials();
        call.then((res) => {
            if (!live) return;
            setData(res.data);
            setSharing(Boolean(res.data.shared));
        })
            .catch((err) => notifyError(err.message || "Couldn't load the list."))
            .finally(() => {
                if (live) setLoading(false);
            });
        return () => {
            live = false;
        };
    }, [tab, reloads, isList]);

    const applySharing = async (next) => {
        setToggling(true);
        try {
            const formData = new FormData();
            formData.set("reportsAcrossCompanies", String(next));
            await inventoryBackend.setSharing(formData);
            setSharing(next);
            setReloads((n) => n + 1);
            notifySuccess(next ? "Showing every company you own." : "Showing this company only.");
        } catch (error) {
            // Interceptor already toasted; the switch stays where it was.
        } finally {
            setToggling(false);
            setConfirmOff(false);
            setTyped("");
        }
    };

    // Turning it ON adds access and needs no ceremony. Turning it OFF takes access away, so it
    // is confirmed by name - the same shape a repository delete uses, and for the same reason:
    // a yes/no is too easy to click through for something that changes what another business
    // can see.
    const onToggle = () => (sharing ? setConfirmOff(true) : applySharing(true));

    const rows = useMemo(() => {
        const all = data?.rows || [];
        const term = search.trim().toLowerCase();
        if (!term) return all;
        return all.filter((r) =>
            tab === "customers"
                ? `${r.clientFirm || ""} ${r.clientName || ""} ${r.clientPhone || ""}`.toLowerCase().includes(term)
                : `${r.material_name || ""} ${r.hsn || ""}`.toLowerCase().includes(term)
        );
    }, [data, search, tab]);

    const { pageRows, page, setPage, perPage, total } = usePagedRows(rows);

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Across Company</span>
            </div>
            <div style={{ padding: 20 }}>
                <div className="shell-segmented" role="tablist" style={{ marginBottom: 16 }}>
                    {TABS.map((t) => (
                        <button
                            key={t.id}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.id}
                            className="shell-segmented-btn"
                            style={tab === t.id ? { background: "var(--xan-violet-bg)", color: "var(--xan-violet)" } : undefined}
                            onClick={() => setTab(t.id)}
                        >
                            {t.label}
                        </button>
                    ))}
                </div>

                {isAdmin && (
                    <button
                        type="button"
                        className={`shared-switch${sharing ? " is-on" : ""}`}
                        aria-pressed={sharing}
                        disabled={toggling}
                        onClick={onToggle}
                    >
                        <span className="shared-switch-icon">
                            <Share2 size={16} aria-hidden="true" />
                        </span>
                        <span className="shared-switch-body">
                            <span className="shared-switch-title">
                                Share customers, products and inventory across my companies
                                <span className="shared-switch-state">{sharing ? "On" : "Off"}</span>
                            </span>
                            <span className="shared-switch-desc">
                                On, a customer or product entered in one company can be selected in all of them, and
                                the inventory report counts stock across every company you own. Each record is still
                                owned — and edited — by the company that created it.
                            </span>
                        </span>
                    </button>
                )}

                {isList && data?.shared && data.duplicates > 0 && (
                    <p className="shared-dupes text-body-small">
                        <Copy size={14} aria-hidden="true" />
                        {data.duplicates} {data.duplicates === 1 ? "record appears" : "records appear"} in more than one
                        company, each with its own history. See the Duplicates tab.
                    </p>
                )}

                {tab === "duplicates" ? (
                    <DuplicateProducts onChanged={() => setReloads((n) => n + 1)} />
                ) : (
                    <>
                        <div className="shared-section-head">
                            <SearchField
                                value={search}
                                onChange={(e) => setSearch(e.target.value)}
                                placeholder={tab === "customers" ? "Search name, firm or phone" : "Search product or HSN"}
                            />
                        </div>

                        {loading ? (
                            <p className="shared-note text-body-small">Loading…</p>
                        ) : total === 0 ? (
                            <p className="shared-note text-body-small">
                                {search ? "Nothing matches that search." : "Nothing to show."}
                            </p>
                        ) : (
                            <>
                                <DataTable loading={loading}>
                                    <thead>
                                        {tab === "customers" ? (
                                            <tr>
                                                <th>Firm</th>
                                                <th>Name</th>
                                                <th>Phone</th>
                                                <th>GST</th>
                                                <th>Company</th>
                                            </tr>
                                        ) : (
                                            <tr>
                                                <th>Product</th>
                                                <th>Unit</th>
                                                <th>HSN</th>
                                                <th className="text-right">Rate</th>
                                                <th>Company</th>
                                            </tr>
                                        )}
                                    </thead>
                                    <tbody>
                                        {pageRows.map((r) =>
                                            tab === "customers" ? (
                                                <tr key={r._id}>
                                                    <td>{r.clientFirm || <Blank />}</td>
                                                    <td>{r.clientName || <Blank />}</td>
                                                    <td className="cell-mono">{r.clientPhone || <Blank />}</td>
                                                    <td className="cell-mono">{r.clientGST || <Blank />}</td>
                                                    <td className="shared-company">{r.companyName || <Blank />}</td>
                                                </tr>
                                            ) : (
                                                <tr key={r._id}>
                                                    <td>{r.material_name}</td>
                                                    <td>{r.unit || <span className="shared-missing">not set</span>}</td>
                                                    <td className="cell-mono">{r.hsn || <Blank />}</td>
                                                    <td className="cell-mono text-right">{money(r.material_rate)}</td>
                                                    <td className="shared-company">{r.companyName || <Blank />}</td>
                                                </tr>
                                            )
                                        )}
                                    </tbody>
                                </DataTable>
                                <div className="table-foot">
                                    <span className="text-body-small shared-count">
                                        {perPage > 0 && total > perPage
                                            ? `${pageRows.length} of ${total} records`
                                            : `${total} ${total === 1 ? "record" : "records"}`}
                                    </span>
                                    <Pagination
                                        totalItems={total}
                                        perPage={perPage}
                                        currentPage={page}
                                        setCurrentPage={setPage}
                                    />
                                </div>
                            </>
                        )}
                    </>
                )}
            </div>

            {/* The named confirmation. It states the two things that actually change and the one
                that does not, because "are you sure" would leave someone assuming invoices are
                at risk when they are not. */}
            {confirmOff && (
                <div className="shared-confirm-backdrop" role="dialog" aria-modal="true">
                    <div className="shared-confirm">
                        <h4 className="shared-confirm-title">Turn off sharing?</h4>
                        <p className="shared-confirm-body text-body-small">
                            Each company will go back to seeing only its own customers and products. Two things change:
                        </p>
                        <ul className="shared-confirm-list text-body-small">
                            <li>Records owned by another company can no longer be selected on new jobs or invoices.</li>
                            <li>The inventory report counts this company only; rows naming another company’s product
                                move to “Not counted”.</li>
                        </ul>
                        <p className="shared-confirm-body text-body-small">
                            <strong>Existing job-ids, invoices, challans and quotations are unaffected</strong> — they
                            keep the product name and rate recorded when they were raised. Turning sharing back on
                            restores everything.
                        </p>
                        <label className="shared-confirm-label text-body-small" htmlFor="shared-confirm-input">
                            Type <strong>OFF</strong> to confirm
                        </label>
                        <input
                            id="shared-confirm-input"
                            className="form-control shared-confirm-input"
                            value={typed}
                            onChange={(e) => setTyped(e.target.value)}
                            autoComplete="off"
                        />
                        <div className="shared-confirm-actions">
                            <button
                                type="button"
                                className="shell-btn"
                                onClick={() => {
                                    setConfirmOff(false);
                                    setTyped("");
                                }}
                            >
                                Cancel
                            </button>
                            <button
                                type="button"
                                className="shell-btn shared-btn-danger"
                                disabled={typed.trim().toUpperCase() !== "OFF" || toggling}
                                onClick={() => applySharing(false)}
                            >
                                {toggling ? "Turning off…" : "Turn off sharing"}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default SharedRecordsIndex;
