import React, { useEffect, useMemo, useState } from "react";
import { RotateCcw } from "react-feather";
import DataTable from "../../Common/DataTable/DataTable";
import Pagination from "../../Shell/Pagination";
import SearchField from "../../Common/SearchField";
import usePagedRows from "../../Common/useRowsPerPage";
import Blank from "../../Common/DataTable/Blank";
import { notifySuccess } from "../../global/toast";
import "./reminderPreferences.css";

// Who gets which WhatsApp message.
//
// One table for suppliers and for customers. They differ in where the rows come from, what
// the events are called, and what an unanswered cell means - so those three are props, and
// `load`/`save` keep this component from knowing about either backend. Two near-identical
// files would drift the moment one of them gained a column.
//
// `load()` resolves to rows already flattened to { _id, name, firm, phone, ...eventFields },
// so the adapter absorbs the fact that a Client calls its name `clientName` and a Person
// calls it `name`. The table stays dumb about both.
//
// Three states per cell either way, but the THIRD one differs and the difference matters:
//
//   nullMeans="default"  (suppliers) - no answer, so a constant decides. The cell says which.
//   nullMeans="ask"      (customers) - no answer, so the prompt after a job-id is raised asks
//                        again. That is a real third behaviour, not a hidden no, which is why
//                        a customer cell cycles through all three rather than toggling two.
//
// Getting this wrong in either direction is a real mistake: showing a customer as "Off" when
// they will in fact be asked, or offering to put a supplier back to "ask" when nothing asks.

const ASK = { cls: "is-default", text: "Asks each time" };

const stateOf = (value, fallback, nullMeans) => {
    const ask = nullMeans === "ask";
    if (value === true) return { cls: "is-on", text: "On", next: "false" };
    if (value === false) return { cls: "is-off", text: "Off", next: ask ? "clear" : "true" };
    // Unanswered. For customers that cycles on to an explicit yes; for suppliers, pressing it
    // writes the OPPOSITE of what the default currently does - writing the same value would
    // look like nothing happened.
    if (ask) return { ...ASK, next: "true" };
    return {
        cls: "is-default",
        text: `Default: ${fallback ? "On" : "Off"}`,
        next: fallback ? "false" : "true",
    };
};

const ReminderPreferences = ({
    title,
    noun,
    events,
    intro,
    emptyHint,
    note,
    load,
    save,
    nullMeans = "default",
}) => {
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState("");
    // Per person, so one slow write does not freeze every other row in the table.
    const [busyId, setBusyId] = useState(null);

    useEffect(() => {
        let live = true;
        setLoading(true);
        load()
            // The global interceptor toasts any failure; this only stops the shimmer.
            .then((rows) => live && setRows(rows || []))
            .catch(() => {})
            .finally(() => live && setLoading(false));
        return () => {
            live = false;
        };
    }, [load]);

    // Name, firm and phone - the same three every other search in this app covers. A supplier
    // is filed under whichever of the first two the person entering them reached for, and
    // searching one while silently missing the other is how a row looks deleted when it is not.
    const filtered = useMemo(() => {
        const term = search.trim().toLowerCase();
        if (!term) return rows;
        return rows.filter((p) => `${p.name || ""} ${p.firm || ""} ${p.phone || ""}`.toLowerCase().includes(term));
    }, [rows, search]);

    // Honours Account settings > Appearance > Rows per page, like every other table.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

    const write = async (person, field, value) => {
        setBusyId(person._id);
        try {
            await save(person._id, field, value);
            const next = value === "clear" ? null : value === "true";
            setRows((prev) => prev.map((p) => (p._id === person._id ? { ...p, [field]: next } : p)));
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setBusyId(null);
        }
    };

    const reset = async (person) => {
        setBusyId(person._id);
        try {
            // Every event, because a Reset that cleared one would leave the row still carrying
            // the other answers - which reads as the button not having worked.
            for (const event of events) {
                await save(person._id, event.field, "clear");
            }
            const cleared = {};
            events.forEach((e) => (cleared[e.field] = null));
            setRows((prev) => prev.map((p) => (p._id === person._id ? { ...p, ...cleared } : p)));
            notifySuccess(
                nullMeans === "ask"
                    ? `${person.name} will be asked each time again.`
                    : `${person.name} follows the default again.`
            );
        } catch (error) {
            // Interceptor has said it.
        } finally {
            setBusyId(null);
        }
    };

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">{title}</span>
            </div>
            <div className="reminder-prefs-body">
                <div className="reminder-prefs-head">
                    <p className="text-body-small reminder-prefs-intro">{intro}</p>
                    <SearchField
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder={`Search ${noun}, firm or phone`}
                    />
                </div>

                {note && <p className="text-body-small reminder-prefs-note">{note}</p>}

                {!loading && total === 0 ? (
                    <p className="reminder-prefs-empty text-body-small">
                        {search ? `No ${noun} matches that search.` : emptyHint}
                    </p>
                ) : (
                    <>
                        {/* Columns told rather than counted: the header is built from `events`,
                            which DataTable's counter cannot see through. */}
                        <DataTable loading={loading} columns={4 + events.length}>
                            <thead>
                                <tr>
                                    <th scope="col">Name</th>
                                    <th scope="col">Firm</th>
                                    <th scope="col">Phone</th>
                                    {events.map((e) => (
                                        <th scope="col" key={e.field} title={e.hint}>
                                            {e.label}
                                        </th>
                                    ))}
                                    <th aria-label="Actions" />
                                </tr>
                            </thead>
                            <tbody>
                                {pageRows.map((person) => (
                                    <tr key={person._id}>
                                        <td>{person.name || <Blank />}</td>
                                        <td>{person.firm || <Blank />}</td>
                                        <td className="cell-mono">{person.phone || <Blank label="No phone" />}</td>
                                        {events.map((event) => {
                                            const cell = stateOf(person[event.field], event.fallback, nullMeans);
                                            return (
                                                <td key={event.field}>
                                                    <button
                                                        type="button"
                                                        className={`reminder-prefs-cell text-body-small ${cell.cls}`}
                                                        disabled={busyId === person._id}
                                                        // The row's identity is in the label,
                                                        // because "On" repeated down three
                                                        // columns tells a screen reader nothing
                                                        // about whose setting it is.
                                                        aria-label={`${person.name} — ${event.label}: ${cell.text}`}
                                                        onClick={() => write(person, event.field, cell.next)}
                                                    >
                                                        {cell.text}
                                                    </button>
                                                </td>
                                            );
                                        })}
                                        <td className="reminder-prefs-actions">
                                            <button
                                                type="button"
                                                className="reminder-prefs-reset text-body-small"
                                                disabled={busyId === person._id}
                                                aria-label={`Reset ${person.name}`}
                                                title={
                                                    nullMeans === "ask"
                                                        ? "Go back to being asked each time"
                                                        : "Go back to following the default"
                                                }
                                                onClick={() => reset(person)}
                                            >
                                                <RotateCcw size={13} aria-hidden="true" />
                                                Reset
                                            </button>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </DataTable>
                        <div className="table-foot">
                            <span className="text-body-small reminder-prefs-intro">
                                {perPage > 0 && total > perPage
                                    ? `${pageRows.length} of ${total} ${noun}s`
                                    : `${total} ${total === 1 ? noun : `${noun}s`}`}
                            </span>
                            <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                        </div>
                    </>
                )}
            </div>
        </div>
    );
};

export default ReminderPreferences;
