import React, { useEffect, useMemo, useState } from "react";
import { AlertTriangle, ChevronDown, ChevronRight } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import DateField from "../../../Common/DateField";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { inventoryBackend } from "../inventory_backend";
import { notifyError, notifySuccess } from "../../../global/toast";
import "./inventory.css";

// Stock per product: bought, minus consumed by jobs, minus wastage.
//
// Two things this screen is careful about, both inherited from the arithmetic in
// Helpers/InventoryMath.js:
//
//   Negative stock is shown, not hidden. It means the floor used stock bought before the
//   system started, or a purchase was never entered - which is the case actually worth
//   chasing, so clamping it to zero would hide the only rows that need attention.
//
//   Rows that could not be counted are shown too, in their own panel. A report that quietly
//   drops rows looks complete and is not. Every product still missing a unit lands there
//   with the reason, rather than being defaulted into a guess.

const number = (n) => (Number(n) || 0).toLocaleString("en-IN", { maximumFractionDigits: 2 });

const InventoryIndex = () => {
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    const [search, setSearch] = useState("");
    const [report, setReport] = useState(null);
    const [loading, setLoading] = useState(false);
    const [showNotCounted, setShowNotCounted] = useState(false);
    // The units this company offers, for the inline picker below. Loaded once - the report's
    // own call already seeds the defaults server-side, so this is never empty on a fresh
    // company.
    const [units, setUnits] = useState([]);
    const [settingUnit, setSettingUnit] = useState(null);
    // Bumped after a unit is assigned, to re-run the report: a product that just gained a
    // unit becomes a stock line, so both tables change.
    const [reloads, setReloads] = useState(0);

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    useEffect(() => {
        let live = true;
        setLoading(true);
        const formData = new FormData();
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        inventoryBackend
            .report(formData)
            .then((res) => live && setReport(res.data))
            .catch((err) => live && notifyError(err.message || "Couldn't load the inventory report."))
            .finally(() => live && setLoading(false));
        return () => {
            live = false;
        };
    }, [from, to, reloads]);

    useEffect(() => {
        inventoryBackend
            .units()
            .then((res) => setUnits(res.data || []))
            // The picker simply stays empty; the interceptor has said why.
            .catch(() => {});
    }, []);

    // Assigning a unit from the warning itself, rather than sending someone to
    // Configure > Products to fix what this panel is already telling them is wrong.
    const assignUnit = async (row, unit) => {
        if (!unit) return;
        setSettingUnit(row.ref);
        try {
            const formData = new FormData();
            formData.set("material_id", row.ref);
            formData.set("unit", unit);
            await inventoryBackend.setUnit(formData);
            notifySuccess(`${row.name} is now counted in ${unit}.`);
            setReloads((n) => n + 1);
        } catch (error) {
            // Interceptor toasts the reason - including "belongs to another company", which is
            // the one case the panel cannot fix itself.
        } finally {
            setSettingUnit(null);
        }
    };

    const rows = useMemo(() => {
        const all = report?.rows || [];
        const term = search.trim().toLowerCase();
        if (!term) return all;
        return all.filter((r) => r.material.toLowerCase().includes(term));
    }, [report, search]);

    // The spreadsheet and the printed page take the same arrays the table renders from, so
    // the three can never disagree about what the report says.
    const columns = [
        { key: "material", label: "Product", width: 2 },
        { key: "unit", label: "Unit" },
        { key: "purchased", label: "Purchased", align: "right" },
        { key: "consumed", label: "Consumed", align: "right" },
        { key: "wasted", label: "Wasted", align: "right" },
        { key: "inStock", label: "In stock", align: "right" },
        { key: "purchaseValue", label: "Value", align: "right" },
    ];
    const exportRows = rows.map((r) => ({
        material: r.material + (r.companies.length > 1 ? ` (${r.companies.join(", ")})` : ""),
        unit: r.unit,
        purchased: number(r.purchased),
        consumed: number(r.consumed),
        wasted: number(r.wasted),
        inStock: number(r.inStock),
        purchaseValue: `₹${RoundOff(r.purchaseValue)}`,
    }));

    // Paged against the Rows-per-page setting, like every other list. A catalogue of a few
    // hundred products is an ordinary size, and the export still carries every row - it is
    // built from `rows`, not from the visible page.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(rows);

    const notCounted = report?.notCounted || [];

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Inventory</span>
                <ReportDownloads
                    title="Inventory"
                    subtitle={report?.shared ? `Across ${report.companyCount} companies` : undefined}
                    columns={columns}
                    rows={exportRows}
                    disabled={loading}
                />
            </div>
            <div style={{ padding: 20 }}>
                <div className="inventory-filters">
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            From
                        </label>
                        <DateField value={from} onChange={(e) => setFrom(e.target.value)} />
                    </div>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            To
                        </label>
                        <DateField value={to} onChange={(e) => setTo(e.target.value)} />
                    </div>
                    <div className="inventory-search-wrap">
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            Search
                        </label>
                        <input
                            className="form-control inventory-search"
                            placeholder="Product name"
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                        />
                    </div>
                </div>

                {/* The sharing switch itself lives in Configure > Across Company, because it
                    governs customers and products too - one control in one place, rather than
                    the same setting reachable from three screens that could each describe it
                    differently. This only reports which state is in force. */}
                {report?.shared && (
                    <p className="inventory-caveat text-body-small">
                        Counting all {report.companyCount} companies you own. Each stock line names the businesses it
                        spans. Change this in Configure → Across Company.
                    </p>
                )}

                {report?.wastageIgnoresRange && (
                    <p className="inventory-caveat text-body-small">
                        Wastage has no date on record, so it is counted in full whatever range you pick.
                    </p>
                )}

                {loading ? (
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Loading…
                    </p>
                ) : rows.length === 0 ? (
                    <p className="inventory-empty text-body-small">
                        {search
                            ? "No product matches that search."
                            : "Nothing to count yet. A product is only counted once it has a unit — set one in Configure → Products."}
                    </p>
                ) : (
                    <DataTable>
                        <thead>
                            <tr>
                                <th>Product</th>
                                <th>Unit</th>
                                <th className="text-right">Purchased</th>
                                <th className="text-right">Consumed</th>
                                <th className="text-right">Wasted</th>
                                <th className="text-right">In stock</th>
                                <th className="text-right">Value</th>
                            </tr>
                        </thead>
                        <tbody>
                            {pageRows.map((r) => (
                                <tr key={r.material}>
                                    <td>
                                        {r.material}
                                        {r.companies.length > 1 && (
                                            <span className="inventory-companies text-body-small">
                                                {r.companies.join(" · ")}
                                            </span>
                                        )}
                                    </td>
                                    <td>{r.unit}</td>
                                    <td className="cell-mono text-right">{number(r.purchased)}</td>
                                    <td className="cell-mono text-right">{number(r.consumed)}</td>
                                    <td className="cell-mono text-right">{number(r.wasted)}</td>
                                    <td className={`cell-mono text-right${r.negative ? " inventory-negative" : ""}`}>
                                        {number(r.inStock)}
                                    </td>
                                    <td className="cell-mono text-right">₹{RoundOff(r.purchaseValue)}</td>
                                </tr>
                            ))}
                        </tbody>
                    </DataTable>
                )}

                {rows.length > 0 && (
                    <div className="table-foot">
                        <span className="text-body-small inventory-count">
                            {perPage > 0 && total > perPage
                                ? `${pageRows.length} of ${total} products`
                                : `${total} ${total === 1 ? "product" : "products"}`}
                        </span>
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                )}

                {/* Collapsed, with its count on the summary line: visible without being noisy.
                    Hiding it entirely would be the failure this panel exists to prevent. */}
                {notCounted.length > 0 && (
                    <div className="inventory-notcounted">
                        <button
                            type="button"
                            className="inventory-notcounted-head"
                            aria-expanded={showNotCounted}
                            onClick={() => setShowNotCounted((on) => !on)}
                        >
                            {showNotCounted ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                            <AlertTriangle size={15} aria-hidden="true" />
                            <span>
                                {notCounted.length} {notCounted.length === 1 ? "row" : "rows"} could not be counted
                            </span>
                        </button>
                        {showNotCounted && (
                            <DataTable>
                                <thead>
                                    <tr>
                                        <th>Where</th>
                                        <th>Name</th>
                                        <th>Why</th>
                                        <th>Fix</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {notCounted.map((n, i) => (
                                        <tr key={`${n.source}-${n.ref}-${i}`}>
                                            <td className="inventory-source">{n.source}</td>
                                            <td>{n.name || <em>(blank)</em>}</td>
                                            <td>{n.reason}</td>
                                            <td className="inventory-fix">
                                                {/* Only a missing unit is fixable from here: it
                                                    is one field on a product this company owns.
                                                    "No matching product" needs a product created
                                                    or a name corrected, and an unreadable
                                                    dimension is on the job row - neither belongs
                                                    behind a dropdown in a warnings panel. */}
                                                {n.source === "product" && n.ref ? (
                                                    <select
                                                        className="form-control inventory-unit-pick"
                                                        value=""
                                                        disabled={settingUnit === n.ref}
                                                        onChange={(e) => assignUnit(n, e.target.value)}
                                                        aria-label={`Set the unit for ${n.name}`}
                                                    >
                                                        <option value="">
                                                            {settingUnit === n.ref ? "Saving…" : "Set unit…"}
                                                        </option>
                                                        {units.map((u) => (
                                                            <option key={u._id} value={u.name}>
                                                                {u.name}
                                                            </option>
                                                        ))}
                                                    </select>
                                                ) : (
                                                    <span className="inventory-nofix">—</span>
                                                )}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </DataTable>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
};

export default InventoryIndex;
