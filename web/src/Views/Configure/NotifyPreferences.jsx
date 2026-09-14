import React, { useEffect, useMemo, useState } from "react";
import { Send } from "react-feather";
import DataTable from "../../Common/DataTable/DataTable";
import Pagination from "../../Shell/Pagination";
import SearchField from "../../Common/SearchField";
import usePagedRows from "../../Common/useRowsPerPage";
import { clientsBackend } from "../Client/client_backend";
import { notifySuccess } from "../../global/toast";
import "./notifyPreferences.css";
import Blank from "../../Common/DataTable/Blank";

// The customers who have a remembered answer to "tell them when a job-id is raised?".
//
// The prompt that sets these (Views/Lifecycle/component/JobCreatedNotifyPrompt) is transient:
// it appears after a job is raised and then it is gone. Without this table a "Not now, and
// remember that" ticked by accident could only be undone by raising another job for that
// customer - a setting you can only reach by doing unrelated work is a setting nobody
// corrects.
//
// Reset writes "clear", not false. The three states are genuinely different: true sends after
// a countdown, false never asks, null asks each time. Resetting to false would silence the
// customer permanently, which is the opposite of what "reset" means.
//
// Paged through usePagedRows so it honours Account settings > Appearance > Rows per page,
// like every other table in the app - a list of customers grows without limit, and this one
// should not be the single table that ignores the setting.
// null is a real third state: never answered, so the prompt still asks. Rendering it as
// "Never told" would claim the customer opted out of something nobody put to them.
const Choice = ({ value }) => {
    if (value === true) return <span className="notify-prefs-chip text-body-small is-on">Told automatically</span>;
    if (value === false) return <span className="notify-prefs-chip text-body-small is-off">Never told</span>;
    return <span className="notify-prefs-chip text-body-small is-ask">Asks each time</span>;
};

// The company-wide defaults live here rather than beside the templates: these ARE the rule
// the table's rows are exceptions to, and a default you cannot see the exceptions to (or
// exceptions with no visible default) is half a setting.
const CompanyDefault = ({ label, desc, value, onChange }) => (
    <button
        type="button"
        className={["relation-toggle", value ? "is-on" : ""].filter(Boolean).join(" ")}
        aria-pressed={value}
        onClick={() => onChange((on) => !on)}
    >
        <span className="relation-toggle-icon">
            <Send size={16} />
        </span>
        <span className="relation-toggle-body">
            <span className="relation-toggle-title">
                {label}
                <span className="relation-toggle-state">{value ? "On" : "Off"}</span>
            </span>
            <span className="relation-toggle-desc">{desc}</span>
        </span>
    </button>
);

const NotifyPreferences = ({ notifyOnCreate, setNotifyOnCreate, notifyOnUpdate, setNotifyOnUpdate }) => {
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState("");
    // Per-row, so one slow reset does not disable every other button in the table.
    const [busyId, setBusyId] = useState(null);

    useEffect(() => {
        let live = true;
        clientsBackend
            .listNotifyPreferences()
            .then((res) => {
                if (live) setRows(res.data || []);
            })
            // The global interceptor toasts the failure; this only stops the spinner.
            .catch(() => {})
            .finally(() => {
                if (live) setLoading(false);
            });
        return () => {
            live = false;
        };
    }, []);

    // Name AND firm, because a customer is filed under either depending on who entered them -
    // searching one and silently missing the other is how a row looks deleted when it is not.
    const filtered = useMemo(() => {
        const term = search.trim().toLowerCase();
        if (!term) return rows;
        return rows.filter((r) =>
            `${r.clientFirm || ""} ${r.clientName || ""}`.toLowerCase().includes(term)
        );
    }, [rows, search]);

    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

    const reset = async (client) => {
        setBusyId(client._id);
        try {
            // Both channels, because one Reset button that cleared only half would leave the
            // row still listed with the other answer intact - which reads as the button not
            // having worked.
            for (const kind of ["created", "updated"]) {
                const formData = new FormData();
                formData.set("client_id", client._id);
                formData.set("kind", kind);
                formData.set("notifyOnCreate", "clear");
                await clientsBackend.setNotifyPreference(formData);
            }
            // Dropped from the list rather than re-fetched: the row no longer has a remembered
            // answer, so it no longer belongs here, and the route filters on exactly that.
            setRows((prev) => prev.filter((r) => r._id !== client._id));
            notifySuccess("This customer will be asked again next time.");
        } catch (error) {
            // Interceptor has already said what went wrong.
        } finally {
            setBusyId(null);
        }
    };

    return (
        // Its own card, matching every other section in Configure, rather than a block nested
        // inside the Templates card - it is a list of customers, not part of template setup.
        <div className="shell-card notify-prefs">
            <div className="shell-card-header">
                <span className="text-heading-brand">Remembered customer choices</span>
            </div>
            <div className="notify-prefs-body">
                {setNotifyOnCreate && (
                    <div className="notify-prefs-defaults">
                        <CompanyDefault
                            label="Notify the customer when a job-id is created"
                            desc="After a job-id is raised you are offered a WhatsApp message for it. With this on the offer sends itself after a few seconds unless you stop it; with it off it waits for you to press send. Either way nothing goes out without you seeing it first."
                            value={notifyOnCreate}
                            onChange={setNotifyOnCreate}
                        />
                        <CompanyDefault
                            label="Notify the customer when a job-id is updated"
                            desc="The same offer when an already-announced job-id changes — a row added or removed, or a quantity, size, rate or tax edited. A card moving between queue stages is not a change to the job-id and never triggers this."
                            value={notifyOnUpdate}
                            onChange={setNotifyOnUpdate}
                        />
                        <p className="text-body-small notify-prefs-savehint">
                            Saved with the connection form — press Save there after changing these.
                        </p>
                    </div>
                )}

            <div className="notify-prefs-head">
                <div>
                    <p className="text-body-small notify-prefs-intro">
                        Set from the card that appears after a job-id is raised, when “Remember choice for this
                        customer” is ticked. Each one overrides the company-wide default. Resetting puts that customer
                        back to being asked every time.
                    </p>
                </div>
                <SearchField
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search client or firm"
                />
            </div>

            {loading ? (
                <p className="notify-prefs-empty text-body-small">Loading…</p>
            ) : total === 0 ? (
                <p className="notify-prefs-empty text-body-small">
                    {search
                        ? "No customer matches that search."
                        : "No customer has a remembered answer yet — every one of them is asked each time."}
                </p>
            ) : (
                <>
                    <DataTable loading={loading}>
                        <thead>
                            <tr>
                                <th>Firm</th>
                                <th>Client</th>
                                <th>Phone</th>
                                {/* One column per message, because the two answers are
                                    remembered separately - a customer can want to hear about a
                                    new job-id and not about every edit to it. */}
                                <th>Created</th>
                                <th>Updated</th>
                                <th aria-label="Actions" />
                            </tr>
                        </thead>
                        <tbody>
                            {pageRows.map((client) => (
                                <tr key={client._id}>
                                    <td>{client.clientFirm || <Blank />}</td>
                                    <td>{client.clientName || <Blank />}</td>
                                    <td className="cell-mono">{client.clientPhone || <Blank />}</td>
                                    <td>
                                        <Choice value={client.notifyOnCreate} />
                                    </td>
                                    <td>
                                        <Choice value={client.notifyOnUpdate} />
                                    </td>
                                    <td className="notify-prefs-actions">
                                        <button
                                            type="button"
                                            className="notify-prefs-reset text-body-small"
                                            disabled={busyId === client._id}
                                            onClick={() => reset(client)}
                                        >
                                            {busyId === client._id ? "Resetting…" : "Reset"}
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </DataTable>
                    <div className="table-foot">
                        <span className="text-body-small notify-prefs-count">
                            {/* The page's share, then the total - "showing 25 of 340" answers
                                "where am I" in a way a bare total does not. Matches the wording
                                LifecycleIndex uses on its own footer. */}
                            {perPage > 0 && total > perPage
                                ? `${pageRows.length} of ${total} customers`
                                : `${total} ${total === 1 ? "customer" : "customers"}`}
                        </span>
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                </>
            )}
            </div>
        </div>
    );
};

export default NotifyPreferences;
