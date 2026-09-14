import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useParams } from "react-router-dom";
import { Container } from "reactstrap";

import LiteHeader from "../../../Common/Header/LiteHeader";
import ThemeToggleButton from "../../../Common/ThemeToggleButton";
import SearchField from "../../../Common/SearchField";
import Loader from "../../../global/Loader/Loader";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { sheetsBackend } from "../sheet_backend";
import JobDetailModal from "../../Lifecycle/component/JobDetailModal";
import { dayParts, relativeDayLabel } from "../../dashboardCards";
import { groupJobsByClient, sheetTotals, filterGroups } from "../sheetGroups";
import SheetClientSection from "./SheetClientSection";
import "../sheet.css";

const money = (n) => `₹${RoundOff(Number(n) || 0)}`;

/**
 * One day's sheet.
 *
 * It was three stat cards over one thirteen-column table per customer, with each job-id's
 * rows nested underneath using box-drawing characters in the Product cell. On a day with
 * several customers that is several horizontally-scrolling tables, and the page that tells
 * the shop what to make today was the hardest page in the app to read.
 *
 * Now: the date said once, in words, with the day's money beside it - then a section per
 * customer, holding their job-ids as cards. The cards are the board's cards, because a job-id
 * is a job-id wherever you meet it.
 *
 * ONE request for the jobs, not one per customer. The page used to mount a component per
 * customer and each fetched its own client's jobs, so a ten-customer day made ten calls that
 * between them returned the same list ten times.
 */
// A day in the URL, as the admin typed it on the job-id.
const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

const PreSheetComponents = () => {
    // /sheet/2026-09-13, or /sheet/<sheet id> for a link made before days were addressed by
    // date. A day is a date now - the Sheet document was only ever consulted to read one back.
    const { id: param } = useParams();
    const paramIsDate = ISO_DATE.test(param || "");
    const [ready, setReady] = useState(false);
    const [date, setDate] = useState("");
    const [jobs, setJobs] = useState([]);
    const [query, setQuery] = useState("");
    const [activeJob, setActiveJob] = useState(null);

    const load = useCallback(async () => {
        try {
            // A date in the URL needs no lookup at all: this page has never read the Sheet's
            // entries, only its date, and then grouped every job by it.
            let day = param;
            if (!paramIsDate) {
                const formData = new FormData();
                formData.set("uid", window.localStorage.getItem("uid"));
                formData.set("sid", param);
                const res = await sheetsBackend.getSheetDetail(formData, window.localStorage.getItem("session_token"));
                day = res.date;
            }
            setDate(day);

            const all = await lifecycleBackend.listJobs({});
            setJobs(all.data || []);
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setReady(true);
        }
    }, [param, paramIsDate]);

    useEffect(() => {
        load();
    }, [load]);

    const groups = useMemo(() => groupJobsByClient(jobs, date), [jobs, date]);
    const shown = useMemo(() => filterGroups(groups, query), [groups, query]);
    // The header counts the WHOLE day, not the search - a total that moved as you typed would
    // be answering a different question from the one the page is for.
    const totals = useMemo(() => sheetTotals(groups), [groups]);

    if (!ready) return <Loader />;

    const parts = dayParts(date);
    const relative = relativeDayLabel(date);

    return (
        // data-shell, like AdminLayout. The Appearance preferences - font family, text scales,
        // accent, scoped resets - are applied under [data-shell] in tokens.css, so a page
        // routed outside ProtectiveRoute falls back to Argon's own body font without it.
        <div data-shell className="sheet-page">
            <LiteHeader bg="primary" />
            <ThemeToggleButton />

            <Container fluid className="sheet-container">
                <header className="sheet-head">
                    <span className="sheet-date">
                        <span className="sheet-date-day">{parts.day}</span>
                        <span className="sheet-date-rest">
                            <span className="sheet-date-mon">
                                {parts.month} {parts.year}
                            </span>
                            <span className="sheet-date-weekday">
                                {parts.weekday}
                                {relative && <span className="sheet-date-chip">{relative}</span>}
                            </span>
                        </span>
                    </span>

                    <span className="sheet-totals">
                        <span className="sheet-total">
                            <span className="sheet-total-label">Customers</span>
                            <span className="sheet-total-value">{totals.customers}</span>
                        </span>
                        <span className="sheet-total">
                            <span className="sheet-total-label">Job-ids</span>
                            <span className="sheet-total-value">{totals.jobs}</span>
                        </span>
                        <span className="sheet-total">
                            <span className="sheet-total-label">Total</span>
                            <span className="sheet-total-value">{money(totals.total)}</span>
                        </span>
                        <span className="sheet-total">
                            <span className="sheet-total-label">Received</span>
                            <span className="sheet-total-value">{money(totals.advance)}</span>
                        </span>
                        <span className={`sheet-total${totals.due > 0 ? " is-due" : ""}`}>
                            <span className="sheet-total-label">Due</span>
                            <span className="sheet-total-value">{money(totals.due)}</span>
                        </span>
                    </span>
                </header>

                {groups.length > 0 && (
                    <SearchField
                        className="sheet-search"
                        placeholder="Search a customer, job-id or product…"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                    />
                )}

                {shown.length === 0 ? (
                    <p className="sheet-empty text-body-regular">
                        {query.trim()
                            ? `Nothing on this day matches “${query.trim()}”.`
                            : "No job-ids were raised on this day."}
                    </p>
                ) : (
                    shown.map((group) => (
                        <SheetClientSection key={group.key} group={group} onOpenJob={setActiveJob} />
                    ))
                )}
            </Container>

            {activeJob && (
                <JobDetailModal
                    job={activeJob}
                    onClose={() => setActiveJob(null)}
                    onChange={(updated) => {
                        setJobs((prev) => prev.map((j) => (j._id === updated._id ? updated : j)));
                        setActiveJob(updated);
                    }}
                />
            )}
        </div>
    );
};

export default PreSheetComponents;
