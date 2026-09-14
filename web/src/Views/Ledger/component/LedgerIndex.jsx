import React, { useEffect, useState } from "react";
import DataTable from "../../../Common/DataTable/DataTable";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { getDate } from "../../../Common/DateAndTime/getDate";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { ledgerBackend } from "../ledger_backend";
import { userBackend } from "../../UserProfile/user_backend";
import { notifyError } from "../../../global/toast";
import LedgerPreviewModal from "./LedgerPreviewModal";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import DateField from "../../../Common/DateField";

// A client's ledger is entirely derived (Invoice + InvoiceReceived/BatchReceive, merged
// chronologically server-side - see routes/Ledger.js) - there's nothing new being stored
// here, this view just renders what /ledger/client computes.
// One shared empty array: a fresh [] each render would change identity and re-run the
// memo inside usePagedRows on every paint.
const EMPTY_ROWS = [];

const LedgerIndex = () => {
    const [clients, setClients] = useState([]);
    const [clientId, setClientId] = useState("");
    const [clientName, setClientName] = useState("");
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    const [ledger, setLedger] = useState(null);
    const [loading, setLoading] = useState(false);
    const [user, setUser] = useState(null);
    const [previewOpen, setPreviewOpen] = useState(false);

    useEffect(() => {
        const formData = new FormData();
        formData.set("uid", window.localStorage.getItem("uid"));
        userBackend.getUserInfo(formData, window.localStorage.getItem("session_token")).then((res) => setUser(res.data));
    }, []);

    useEffect(() => {
        lifecycleBackend.lookupClients().then((res) => setClients(res.data));
    }, []);

    const clientOptions = clients.map((c) => ({
        id: c._id,
        name: c.clientFirm || c.clientName,
        search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
    }));

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    // Switching customer matters as much as the dates here: the old customer's ledger
    // landing after the new one is picked shows one name above another's figures.
    useEffect(() => {
        if (!clientId) {
            setLedger(null);
            return undefined;
        }
        let live = true;
        setLoading(true);
        const formData = new FormData();
        formData.set("client_id", clientId);
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        ledgerBackend
            .clientLedger(formData)
            .then((res) => live && setLedger(res.data))
            .catch((err) => live && notifyError(err.message || "Couldn't load this client's ledger."))
            .finally(() => live && setLoading(false));
        return () => {
            live = false;
        };
    }, [clientId, from, to]);

    // `ledger` is null until the fetch lands, so the hook takes a stable [] - it must be
    // called on every render regardless, and a conditional hook is a React error.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(ledger?.rows || EMPTY_ROWS);

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Client Ledger</span>
            </div>
            <div style={{ padding: 20 }}>
                <div className="d-flex align-items-end" style={{ gap: 16, flexWrap: "wrap", marginBottom: 20 }}>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            Client
                        </label>
                        <AssigneeDropdown
                            value={clientName}
                            placeholder="Select client"
                            options={clientOptions}
                            fullWidth
                            menuWidth={320}
                            onSelect={(id, name) => {
                                setClientId(id || "");
                                setClientName(name || "");
                            }}
                        />
                    </div>
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
                </div>

                {!clientId && (
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Pick a client to see their ledger.
                    </p>
                )}

                {clientId && loading && (
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Loading...
                    </p>
                )}

                {clientId && !loading && ledger && (
                    <>
                        <div className="d-flex align-items-center" style={{ gap: 24, marginBottom: 16, flexWrap: "wrap" }}>
                            <div>
                                <div className="text-label-caps" style={{ color: "var(--text-tertiary)" }}>
                                    Opening Balance
                                </div>
                                <div className="cell-mono" style={{ fontSize: 16, fontWeight: 700 }}>
                                    ₹{RoundOff(ledger.openingBalance)}
                                </div>
                            </div>
                            <div>
                                <div className="text-label-caps" style={{ color: "var(--text-tertiary)" }}>
                                    Balance (as of today)
                                </div>
                                <div
                                    className="cell-mono"
                                    style={{ fontSize: 16, fontWeight: 700, color: ledger.currentBalance > 0 ? "var(--status-red-text)" : "inherit" }}
                                >
                                    ₹{RoundOff(ledger.currentBalance)}
                                </div>
                            </div>
                            <button type="button" className="shell-btn shell-btn-sm shell-btn-secondary" onClick={() => setPreviewOpen(true)}>
                                Preview PDF
                            </button>
                            {/* Preview above renders the ledger's own template; these export
                                the same rows as a file. */}
                            <ReportDownloads
                                title="Client Ledger"
                                subtitle={ledger.clientFirm || ledger.clientName}
                                summary={[
                                    { label: "Opening Balance", value: `₹${RoundOff(ledger.openingBalance)}` },
                                    { label: "Closing Balance", value: `₹${RoundOff(ledger.closingBalance)}` },
                                    { label: "Balance (today)", value: `₹${RoundOff(ledger.currentBalance)}` },
                                ]}
                                columns={[
                                    { label: "Sr", width: "6%" },
                                    { label: "Date", width: "13%" },
                                    { label: "Type", width: "19%" },
                                    { label: "Invoice No", width: "18%" },
                                    { label: "Bill", width: "14%", align: "right" },
                                    { label: "Receipt", width: "15%", align: "right" },
                                    { label: "Balance", width: "15%", align: "right" },
                                ]}
                                rows={(ledger.rows || []).map((r) => [
                                    r.sr,
                                    getDate(r.date),
                                    r.type,
                                    r.invoiceNo || "",
                                    r.bill != null ? RoundOff(r.bill) : "",
                                    r.receipt != null ? RoundOff(r.receipt) : "",
                                    RoundOff(r.balance),
                                ])}
                                foot={["", "", "Balance After", "", "", "", RoundOff(ledger.currentBalance)]}
                            />
                        </div>

                        <DataTable loading={loading}>
                            <thead>
                                <tr>
                                    <th scope="col">Sr</th>
                                    <th scope="col">Date</th>
                                    <th scope="col">Type</th>
                                    <th scope="col">Invoice No</th>
                                    <th scope="col">Bill Amount</th>
                                    <th scope="col">Receipt / Payment Amount</th>
                                    <th scope="col">Balance</th>
                                </tr>
                            </thead>
                            <tbody>
                                {pageRows.map((row) => (
                                    <tr key={row.sr}>
                                        <td className="cell-mono">{row.sr}</td>
                                        <td className="cell-mono">{getDate(row.date)}</td>
                                        <td>{row.type}</td>
                                        <td className="cell-mono">{row.invoiceNo || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                        <td className="cell-mono">{row.bill != null ? `₹${RoundOff(row.bill)}` : ""}</td>
                                        <td className="cell-mono">{row.receipt != null ? `₹${RoundOff(row.receipt)}` : ""}</td>
                                        <td className="cell-mono">₹{RoundOff(row.balance)}</td>
                                    </tr>
                                ))}
                                <tr>
                                    <td className="cell-mono">{ledger.rows.length + 1}</td>
                                    <td className="cell-mono">{getDate(to || new Date())}</td>
                                    <td style={{ fontWeight: 700 }}>Balance After</td>
                                    <td />
                                    <td />
                                    <td />
                                    <td className="cell-mono" style={{ fontWeight: 700 }}>
                                        ₹{RoundOff(ledger.currentBalance)}
                                    </td>
                                </tr>
                            </tbody>
                        </DataTable>
                        <div className="table-foot">
                            <span className="text-body-small table-foot-count">
                                {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} entries` : `${total} entries`}
                            </span>
                            <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                        </div>
                    </>
                )}
            </div>
            <LedgerPreviewModal isOpen={previewOpen} toggle={() => setPreviewOpen(false)} ledger={ledger} user={user} from={from} to={to} />
        </div>
    );
};

export default LedgerIndex;
