import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Container } from "reactstrap";
import { Key, Copy, Download, Trash2, RotateCcw, Power, MessageSquare, Check } from "react-feather";
import LiteHeader from "../../../Common/Header/LiteHeader";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import StatCard from "../../../Shell/StatCard";
import { devBackend } from "../dev_backend";
import { notifySuccess, notifyError } from "../../../global/toast";
import WipeDialog from "./WipeDialog";
import InboxPanel from "./InboxPanel";
import "../dev.css";
import DateField from "../../../Common/DateField";
import { copyText } from "../../../Common/clipboard";
import Blank from "../../../Common/DataTable/Blank";

const fmtDate = (d) => (d ? new Date(d).toLocaleDateString() : "—");

// Active / Expired / Inactive is derived the same way Helpers/Tenancy.js derives it on the
// server, so the badge never disagrees with whether the customer can actually log in.
const statusOf = (row) => {
    if (!row.is_active) return { label: "Inactive", status: "red" };
    if (row.activeUntil && new Date(row.activeUntil).getTime() <= Date.now()) return { label: "Expired", status: "amber" };
    return { label: "Active", status: "lime" };
};

const DevIndex = () => {
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [busyUid, setBusyUid] = useState("");
    // People locked out cannot help themselves - there is no mail sender here - so the queue
    // carries a count, or a request sits unseen for a week.
    const [pwRequests, setPwRequests] = useState([]);
    const [issued, setIssued] = useState(null);

    const loadPwRequests = () =>
        devBackend
            .listPasswordRequests()
            .then((res) => setPwRequests(res.data || []))
            .catch(() => {});

    // Landing-page demo enquiries. Same shape as the password requests above: a list that
    // has to be looked at, with a way to mark one dealt with so the count means something.
    // The page had grown into one long scroll; the inbox is a different job from customer
    // administration, so it gets its own tab rather than a tenth card.
    const [tab, setTab] = useState("users");

    const [enquiries, setEnquiries] = useState([]);

    const loadEnquiries = () =>
        devBackend
            .listEnquiries()
            .then((res) => setEnquiries(res.data || []))
            .catch(() => {});


    const [wipeTarget, setWipeTarget] = useState(null);
    const [footprint, setFootprint] = useState(null);
    const [wiping, setWiping] = useState(false);
    const [lastBatch, setLastBatch] = useState({});

    const load = useCallback(() => {
        setLoading(true);
        devBackend
            .listAdmins()
            .then((res) => setRows(res.data))
            .catch(() => {})
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        load();
    }, [load]);

    // Both queues load on mount. loadPwRequests used to be called only from the resolve and
    // dismiss handlers, so the list could never fill: it started empty, and nothing can be
    // resolved from an empty list.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    useEffect(() => {
        loadEnquiries();
        loadPwRequests();
    }, []);

    // The badge counts what still needs a reply, not the whole history.
    const pendingEnquiries = useMemo(() => enquiries.filter((e) => !e.handled).length, [enquiries]);

    const stats = useMemo(() => {
        const total = rows.length;
        let active = 0;
        let expired = 0;
        let inactive = 0;
        rows.forEach((r) => {
            const s = statusOf(r).label;
            if (s === "Active") active += 1;
            else if (s === "Expired") expired += 1;
            else inactive += 1;
        });
        return { total, active, expired, inactive };
    }, [rows]);

    const openWipe = async (row) => {
        setWipeTarget(row);
        setFootprint(null);
        try {
            const res = await devBackend.footprint(row._id);
            setFootprint(res.data);
        } catch (err) {
            setFootprint({ total: 0, collections: 0, breakdown: [] });
        }
    };

    const doWipe = async (confirmEmail) => {
        setWiping(true);
        try {
            const res = await devBackend.wipe(wipeTarget._id, confirmEmail);
            // Hold on to the batch so Restore is one click away instead of a database dig.
            setLastBatch((prev) => ({ ...prev, [wipeTarget._id]: res.data.batchId }));
            setWipeTarget(null);
            load();
        } catch (err) {
            notifyError(err.message || "Could not archive this customer.");
        } finally {
            setWiping(false);
        }
    };

    const doRestore = async (row) => {
        const batchId = lastBatch[row._id];
        if (!batchId) return notifyError("No archive from this session to restore.");
        setBusyUid(row._id);
        try {
            const res = await devBackend.restore(row._id, batchId);
            load();
        } catch (err) {
            notifyError(err.message || "Could not restore.");
        } finally {
            setBusyUid("");
        }
    };

    const toggleActive = async (row) => {
        setBusyUid(row._id);
        try {
            await devBackend.setStatus(row._id, { is_active: !row.is_active });
            notifySuccess(`${row.email} is now ${row.is_active ? "inactive" : "active"}.`);
            load();
        } catch (err) {
            notifyError(err.message || "Could not update.");
        } finally {
            setBusyUid("");
        }
    };

    const setLimit = async (row, value) => {
        const next = Number(value);
        if (!Number.isFinite(next) || next === (row.companyLimit ?? 1)) return;
        setBusyUid(row._id);
        try {
            await devBackend.setCompanyLimit(row._id, next);
            await load();
        } catch (error) {
            // The global interceptor raises the toast; reload so the box shows what is stored
            // rather than what was refused.
            await load();
        } finally {
            setBusyUid("");
        }
    };

    const setUntil = async (row, value) => {
        setBusyUid(row._id);
        try {
            await devBackend.setStatus(row._id, { activeUntil: value });
            load();
        } catch (err) {
            notifyError(err.message || "Could not update.");
        } finally {
            setBusyUid("");
        }
    };

    // Every API call authenticates with a SESSION-TOKEN header, and an <a download> cannot
    // set headers - so the export is fetched, then handed to the browser as a blob.
    const doExport = async (row) => {
        setBusyUid(row._id);
        try {
            const res = await devBackend.exportData(row._id);
            const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: "application/json" });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = `invoicemg-${row.email.replace(/[^\w.-]+/g, "_")}-${new Date().toISOString().slice(0, 10)}.json`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            URL.revokeObjectURL(url);
            notifySuccess("Export downloaded.");
        } catch (err) {
            notifyError(err.message || "Could not export.");
        } finally {
            setBusyUid("");
        }
    };


    return (
        <>
            <LiteHeader bg="primary" />
            <Container fluid className="dev-page">

                <div className="segmented" role="tablist" aria-label="Developer sections">
                    {[
                        { id: "users", label: "Users" },
                        { id: "enquiries", label: "Enquiries" },
                        { id: "resets", label: "Resets" },
                        { id: "inbox", label: "Inbox" },
                    ].map((t) => (
                        <button
                            key={t.id}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.id}
                            className={["segmented-option", tab === t.id ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setTab(t.id)}
                        >
                            <span>{t.label}</span>
                        </button>
                    ))}
                </div>

                {tab === "users" && (
                    <>
                    <div className="stat-card-row">
                        <StatCard label="Customers" value={stats.total} />
                        <StatCard label="Active" value={stats.active} />
                        <StatCard label="Expired" value={stats.expired} />
                        <StatCard label="Inactive" value={stats.inactive} />
                    </div>

                    <DataTable loading={loading} className="dev-table">
                        <thead>
                            <tr>
                                <th>Customer</th>
                                <th>Status</th>
                                <th>Active until</th>
                                <th>Expiry</th>
                                <th>Companies / allowed</th>
                                <th>Last login</th>
                                <th style={{ textAlign: "right" }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {loading ? (
                                <tr>
                                    <td colSpan={7} className="dev-muted">
                                        Loading…
                                    </td>
                                </tr>
                            ) : (
                                rows.map((row) => {
                                    const status = statusOf(row);
                                    const isSuper = row.role === "superadmin";
                                    const busy = busyUid === row._id;
                                    return (
                                        <tr key={row._id}>
                                            <td>
                                                <div className="dev-cell-name">{row.name || <Blank />}</div>
                                                <div className="dev-muted">{row.email}</div>
                                            </td>
                                            <td>
                                                <StatusBadge status={status.status}>{status.label}</StatusBadge>
                                                {isSuper ? <span className="dev-chip">superadmin</span> : null}
                                            </td>
                                            <td>
                                                <DateField value={row.activeUntil ? String(row.activeUntil).slice(0, 10) : ""} disabled={isSuper || busy} onChange={(e) => setUntil(row, e.target.value)} />
                                            </td>
                                            <td>
                                                <span className={["dev-expiry", row.activeUntil ? "" : "dev-muted"].filter(Boolean).join(" ")}>
                                                    {row.activeUntil ? fmtDate(row.activeUntil) : "no expiry"}
                                                </span>
                                            </td>
                                            <td>
                                                {/* Owned / allowed. Editable because the allowance
                                                    is the thing a superadmin grants; the count
                                                    beside it is why the number matters. */}
                                                <span className="dev-limit">
                                                    <span className="cell-mono">{row.companies}</span>
                                                    <span className="dev-muted"> / </span>
                                                    <input
                                                        type="number"
                                                        inputMode="numeric"
                                                        min={Math.max(1, row.companies)}
                                                        className="dev-input dev-input--limit"
                                                        defaultValue={row.companyLimit ?? 1}
                                                        disabled={isSuper || busy}
                                                        onBlur={(e) => setLimit(row, e.target.value)}
                                                        onKeyDown={(e) => e.key === "Enter" && e.target.blur()}
                                                        aria-label={`Companies allowed for ${row.name}`}
                                                    />
                                                </span>
                                            </td>
                                            <td className="dev-muted">{fmtDate(row.lastLoginAt)}</td>
                                            <td className="dev-actions">
                                                {isSuper ? (
                                                    <span className="dev-muted">protected</span>
                                                ) : (
                                                    <>
                                                        <button
                                                            type="button"
                                                            className={`dev-icon-btn dev-icon-btn--toggle ${
                                                                row.is_active ? "dev-icon-btn--on" : "dev-icon-btn--off"
                                                            }`}
                                                            title={row.is_active ? "Deactivate" : "Activate"}
                                                            disabled={busy}
                                                            onClick={() => toggleActive(row)}
                                                        >
                                                            <Power size={15} />
                                                        </button>
                                                        <button
                                                            type="button"
                                                            className="dev-icon-btn"
                                                            title="Export data"
                                                            disabled={busy}
                                                            onClick={() => doExport(row)}
                                                        >
                                                            <Download size={15} />
                                                        </button>
                                                        {lastBatch[row._id] ? (
                                                            <button
                                                                type="button"
                                                                className="dev-icon-btn"
                                                                title="Restore the archive from this session"
                                                                disabled={busy}
                                                                onClick={() => doRestore(row)}
                                                            >
                                                                <RotateCcw size={15} />
                                                            </button>
                                                        ) : null}
                                                        <button
                                                            type="button"
                                                            className="dev-icon-btn dev-icon-btn--danger"
                                                            title="Archive all data"
                                                            disabled={busy}
                                                            onClick={() => openWipe(row)}
                                                        >
                                                            <Trash2 size={15} />
                                                        </button>
                                                    </>
                                                )}
                                            </td>
                                        </tr>
                                    );
                                })
                            )}
                        </tbody>
                    </DataTable>
                    </>
                )}

                {tab === "enquiries" && (
                    <>
                    <div className="dev-token-card">
                        <div className="dev-token-head">
                            <MessageSquare size={16} />
                            <span>Demo enquiries</span>
                            {pendingEnquiries > 0 && <span className="dev-badge-count">{pendingEnquiries}</span>}
                        </div>
                        <p className="dev-muted">
                            Sent from the landing page&rsquo;s &ldquo;Request a demo&rdquo; form. Nobody is emailed
                            automatically &mdash; reply on WhatsApp or by mail, then mark it handled.
                        </p>
                        {enquiries.length === 0 ? (
                            <p className="dev-muted">Nothing waiting.</p>
                        ) : (
                            <ul className="dev-pw-list">
                                {enquiries.map((e) => (
                                    <li key={e._id} style={{ opacity: e.handled ? 0.55 : 1 }}>
                                        <span className="cell-mono">{e.phone}</span>
                                        <span>
                                            {e.name}
                                            {e.companyName ? ` · ${e.companyName}` : ""}
                                        </span>
                                        <span className="dev-muted">{new Date(e.createdAt).toLocaleString()}</span>
                                        <span className="dev-muted" style={{ flexBasis: "100%" }}>
                                            {e.email}
                                            {e.note ? ` — ${e.note}` : ""}
                                        </span>
                                        <button
                                            type="button"
                                            className="shell-btn shell-btn-secondary"
                                            onClick={async () => {
                                                await devBackend.setEnquiryHandled(e._id, !e.handled);
                                                notifySuccess(e.handled ? "Reopened." : "Marked handled.");
                                                loadEnquiries();
                                            }}
                                        >
                                            <Check size={14} aria-hidden="true" />
                                            {e.handled ? "Reopen" : "Handled"}
                                        </button>
                                    </li>
                                ))}
                            </ul>
                        )}
                    </div>
                    </>
                )}

                {tab === "resets" && (
                    <>
                    <div className="dev-token-card">
                        <div className="dev-token-head">
                            <Key size={16} />
                            <span>Password resets</span>
                            {pwRequests.length > 0 && <span className="dev-badge-count">{pwRequests.length}</span>}
                        </div>
                        <p className="dev-muted">
                            Raised from the sign-in screen by someone who cannot get in. Resetting issues a random password,
                            shown once — read it out, it is not stored anywhere.
                        </p>
                        {pwRequests.length === 0 ? (
                            <p className="dev-muted">Nothing waiting.</p>
                        ) : (
                            <ul className="dev-pw-list">
                                {pwRequests.map((r) => (
                                    <li key={r._id}>
                                        <span className="cell-mono">{r.email}</span>
                                        <span className="dev-muted">{new Date(r.createdAt).toLocaleString()}</span>
                                        <span className="dev-pw-actions">
                                            <button
                                                type="button"
                                                className="dev-btn dev-btn--primary"
                                                onClick={async () => {
                                                    try {
                                                        const res = await devBackend.resolvePasswordRequest(r._id);
                                                        setIssued(res.data);
                                                        notifySuccess(res.message);
                                                        loadPwRequests();
                                                    } catch (err) {
                                                        notifyError(err.message || "Could not reset.");
                                                    }
                                                }}
                                            >
                                                Reset
                                            </button>
                                            <button
                                                type="button"
                                                className="dev-btn dev-btn--ghost"
                                                onClick={async () => {
                                                    await devBackend.resolvePasswordRequest(r._id, "dismiss");
                                                    loadPwRequests();
                                                }}
                                            >
                                                Dismiss
                                            </button>
                                        </span>
                                    </li>
                                ))}
                            </ul>
                        )}
                        {issued && (
                            <div className="dev-token-value">
                                <code>{issued.temporaryPassword}</code>
                                <button
                                    type="button"
                                    className="dev-btn dev-btn--ghost"
                                    onClick={async () => {
                                        if (await copyText(issued.temporaryPassword)) notifySuccess("Copied.");
                                        else notifyError("Couldn't copy - select it and copy manually.");
                                    }}
                                >
                                    <Copy size={14} /> Copy
                                </button>
                                <span className="dev-muted">for {issued.email} — shown once</span>
                            </div>
                        )}
                    </div>
                    </>
                )}

                {tab === "inbox" && <InboxPanel />}

            </Container>

            <WipeDialog
                open={!!wipeTarget}
                target={wipeTarget}
                footprint={footprint}
                busy={wiping}
                onConfirm={doWipe}
                onCancel={() => setWipeTarget(null)}
            />
        </>
    );
};

export default DevIndex;
