import React, { useState, useEffect, useCallback, useMemo } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Phone, ChevronRight, Clock } from "react-feather";

import Loader from "../global/Loader/Loader";
import { clientsBackend } from "./Client/client_backend";
import { sheetsBackend } from "./Sheet/sheet_backend";
import { getDateForEntry } from "../Common/DateAndTime/getDate";
import Pagination from "../Shell/Pagination";
import SearchField from "../Common/SearchField";
import { relativeDayLabel, dayParts, initials, cardDelay } from "./dashboardCards";
import "../Shell/shell.css";
import "./dashboard.css";

// The dashboard, as two decks of cards.
//
// It was two tables side by side under a pair of stat cards. The stat cards counted the rows
// of the tables directly beneath them, which is a number anybody could see by looking, and the
// tables asked someone to read a date as "2026-09-13" and a customer as a row of four columns.
// Neither is hard for a person who works in software all day. Both are work for a person who
// came here to answer "what came in on Tuesday" or "what is Himani's number".
//
// So: one thing at a time, chosen by a tab, and each thing shaped like what it is. A day is a
// tile with the count on it. A customer is a name with a face-substitute and a number you can
// press. Everything else went.
//
// Two tabs rather than both decks at once, because a screen showing two scrolling grids makes
// you decide where to look before you can start looking.
const TABS = [
    { key: "daily", label: "Daily" },
    { key: "customers", label: "Customers" },
];

// A grid-friendly page. Not Appearance's rows-per-page: that is a reading preference about
// table ROWS, and these are tiles whose count is a property of the layout - 24 fills three,
// four or six to a row without leaving a ragged last line at any common width.
const PER_PAGE = 24;

// Enough job-ids named for someone to recognise which ones; the rest are a count. The route
// caps its answer at 200, so "all of them" was never on the table.
const OPEN_SHOWN = 6;

const Index = () => {
    const [ready, setReady] = useState(false);
    const [tab, setTab] = useState("daily");
    const [customers, setCustomers] = useState([]);
    const [dates, setDates] = useState([]);
    const [openJobs, setOpenJobs] = useState([]);
    const [query, setQuery] = useState("");
    const [page, setPage] = useState(1);
    const navigate = useNavigate();

    const load = useCallback(async () => {
        try {
            const formData = new FormData();
            formData.set("uid", window.localStorage.getItem("uid"));
            // getAllSheets takes the token as its SECOND argument and sets the header from
            // it - dropping it sends SESSION-TOKEN: undefined, which comes back 401 and logs
            // the session out on the way past the interceptor.
            const stoken = window.localStorage.getItem("session_token");
            const [clientRes, sheetRes] = await Promise.all([
                clientsBackend.getAllClientWithData(formData, stoken),
                sheetsBackend.getAllSheets(formData, stoken),
            ]);
            setCustomers(clientRes.data || []);
            // Newest first: the day you are asked about is nearly always a recent one.
            setDates([...(sheetRes.data || [])].sort((a, b) => new Date(b.date) - new Date(a.date)));
            // Failing here must not take the dashboard down with it.
            sheetsBackend
                .getOpenJobs()
                .then((res) => setOpenJobs(res.data || []))
                .catch(() => setOpenJobs([]));
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setReady(true);
        }
    }, []);

    useEffect(() => {
        load();
    }, [load]);

    // One search box, searching whatever is on screen. Two boxes for two decks meant the one
    // you wanted was whichever you were not looking at.
    const term = query.trim().toLowerCase();

    const shownDates = useMemo(() => {
        if (!term) return dates;
        return dates.filter((sheet) => {
            const parts = dayParts(sheet.date);
            // Typed as "13 sep", as "sep", as "2026-09-13", or as the word Today - all of them
            // are how somebody says the day they mean.
            const haystack = `${getDateForEntry(sheet.date)} ${parts.weekday} ${parts.day} ${parts.month} ${parts.year} ${relativeDayLabel(sheet.date)}`;
            return haystack.toLowerCase().includes(term);
        });
    }, [dates, term]);

    const shownCustomers = useMemo(() => {
        if (!term) return customers;
        return customers.filter((c) =>
            `${c.clientFirm || ""} ${c.clientName || ""} ${c.clientPhone || ""}`.toLowerCase().includes(term)
        );
    }, [customers, term]);

    // Cards, not job-ids: the sentence says cards, so the number has to be the rows.
    const openCards = useMemo(() => openJobs.reduce((sum, job) => sum + (job.openCards || 0), 0), [openJobs]);

    const list = tab === "daily" ? shownDates : shownCustomers;
    const pageCount = Math.max(1, Math.ceil(list.length / PER_PAGE));
    const safePage = Math.min(page, pageCount);
    const pageRows = list.slice((safePage - 1) * PER_PAGE, safePage * PER_PAGE);

    const switchTab = (key) => {
        if (key === tab) return;
        setTab(key);
        // Both the search and the page belong to the deck that is leaving. Carrying either
        // across lands you on page 3 of a filter you cannot see.
        setQuery("");
        setPage(1);
    };

    if (!ready) return <Loader />;

    return (
        <div className="dash">
            <div className="dash-head">
                <div className="shell-segmented" role="tablist">
                    {TABS.map((t) => (
                        <button
                            key={t.key}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.key}
                            className={`shell-segmented-btn${tab === t.key ? " active" : ""}`}
                            onClick={() => switchTab(t.key)}
                        >
                            {t.label}
                            <span className="dash-tab-count">{t.key === "daily" ? dates.length : customers.length}</span>
                        </button>
                    ))}
                </div>

                <SearchField
                    className="dash-search"
                    placeholder={tab === "daily" ? "Search a day — 13 Sep, September, today…" : "Search a firm, name or number…"}
                    value={query}
                    onChange={(e) => {
                        setQuery(e.target.value);
                        setPage(1);
                    }}
                />
            </div>

            {/* Only on Daily, and only when it applies: job-ids that still have work on them.
                Not a fault - the ordinary state of anything in progress - but the thing the
                day list cannot tell you, because a day shows what came in, not what is left. */}
            {tab === "daily" && openJobs.length > 0 && (
                <section className="dash-pending" aria-label="Job-ids with work still open">
                    <span className="dash-pending-icon" aria-hidden="true">
                        <Clock size={17} />
                    </span>
                    <div className="dash-pending-body">
                        <p className="dash-pending-title">
                            {openJobs.length} job-id{openJobs.length === 1 ? "" : "s"} still open
                        </p>
                        <p className="dash-pending-detail text-body-small">
                            {openCards} job card{openCards === 1 ? "" : "s"} {openCards === 1 ? "has" : "have"} not
                            reached Done. Oldest first — the one at the front has been waiting the longest.
                        </p>
                        <ul className="dash-pending-chips">
                            {openJobs.slice(0, OPEN_SHOWN).map((job) => (
                                <li key={job._id} className="dash-pending-chip">
                                    {job.challanNumber}
                                    <span className="dash-pending-chip-count">
                                        {job.openCards}/{job.cards}
                                    </span>
                                </li>
                            ))}
                            {openJobs.length > OPEN_SHOWN && (
                                <li className="dash-pending-chip is-more">+{openJobs.length - OPEN_SHOWN} more</li>
                            )}
                        </ul>
                    </div>
                    {/* ?open=1, not the bare list: landing on all 47 job-ids with nothing to
                        say which 35 the sentence meant makes the alert a dead end. */}
                    <Link to="/admin/lifecycle?open=1" className="dash-pending-action">
                        Open in Lifecycle
                        <ChevronRight size={15} />
                    </Link>
                </section>
            )}

            {list.length === 0 ? (
                <p className="dash-empty text-body-regular">
                    {term
                        ? `Nothing matches “${query.trim()}”.`
                        : tab === "daily"
                        ? "No days with work on them yet."
                        : "No customers yet."}
                </p>
            ) : (
                <>
                    {/* Keyed by tab AND page so React rebuilds the deck on every change, which
                        is what replays the entrance. A grid that re-sorts in place under you
                        is harder to follow than one that arrives. */}
                    <div className={`dash-grid is-${tab}`} key={`${tab}-${safePage}`}>
                        {tab === "daily"
                            ? pageRows.map((sheet, i) => {
                                  const parts = dayParts(sheet.date);
                                  const relative = relativeDayLabel(sheet.date);
                                  // `cards`, not `jobs`: the label says job cards, so the
                                  // number has to be the rows, not the job-ids holding them.
                                  // The route sends both (routes/Sheet.js) precisely so a card
                                  // cannot print one under the other's name.
                                  const count = sheet.cards ?? 0;
                                  return (
                                      <button
                                          type="button"
                                          key={sheet.date}
                                          className={`dash-card dash-day${relative ? " is-recent" : ""}`}
                                          style={{ animationDelay: `${cardDelay(i)}ms` }}
                                          onClick={() => navigate(`/sheet/${sheet.date}`)}
                                      >
                                          <span className="dash-day-top">
                                              <span className="dash-day-weekday">{parts.weekday}</span>
                                              {relative && <span className="dash-day-chip">{relative}</span>}
                                          </span>
                                          <span className="dash-day-date">
                                              <span className="dash-day-num">{parts.day}</span>
                                              <span className="dash-day-mon">
                                                  {parts.month}
                                                  <span className="dash-day-year">{parts.year}</span>
                                              </span>
                                          </span>
                                          {/* The count is the answer the card exists to give,
                                              so it is the biggest thing on it after the day. */}
                                          <span className="dash-day-count">
                                              <strong>{count}</strong> job card{count === 1 ? "" : "s"}
                                          </span>
                                      </button>
                                  );
                              })
                            : pageRows.map((client, i) => {
                                  return (
                                      <button
                                          type="button"
                                          key={client._id}
                                          className="dash-card dash-customer"
                                          style={{ animationDelay: `${cardDelay(i)}ms` }}
                                          onClick={() => navigate(`/customer/${client._id}`)}
                                      >
                                          <span className="dash-monogram" aria-hidden="true">
                                              {initials(client.clientFirm || client.clientName)}
                                          </span>
                                          <span className="dash-customer-body">
                                              <span className="dash-customer-firm">{client.clientFirm || "—"}</span>
                                              {client.clientName && client.clientName !== client.clientFirm && (
                                                  <span className="dash-customer-name">{client.clientName}</span>
                                              )}
                                              {/* Read, not pressed. It was a tel: link, which
                                                  put a second target inside a card that is
                                                  already one button - two things to hit, one
                                                  of which dials. The number is what someone
                                                  came to the card for; the card opens the
                                                  customer. */}
                                              {client.clientPhone ? (
                                                  <span className="dash-customer-phone">
                                                      <Phone size={12} aria-hidden="true" />
                                                      {client.clientPhone}
                                                  </span>
                                              ) : (
                                                  <span className="dash-customer-phone is-missing">No number</span>
                                              )}
                                          </span>
                                      </button>
                                  );
                              })}
                    </div>

                    {/* Only once there is more than one page. Below that the row held a
                        count the tab badge already carries and a pager with nothing to page,
                        so it read as a band of empty space under a half-filled grid. */}
                    {pageCount > 1 && (
                        <div className="dash-foot">
                            <Pagination totalItems={list.length} perPage={PER_PAGE} currentPage={safePage} setCurrentPage={setPage} />
                        </div>
                    )}
                </>
            )}
        </div>
    );
};

export default Index;
