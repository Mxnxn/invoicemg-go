import React, { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Calendar, ChevronLeft, ChevronRight } from "react-feather";
import {
    WEEKDAYS,
    MONTHS,
    MONTHS_SHORT,
    YEARS_PER_PAGE,
    addMonths,
    formatDisplay,
    isSameDay,
    monthGrid,
    parseISO,
    toISO,
    toMonthISO,
    parseMonthISO,
    formatMonthDisplay,
    todayParts,
    yearPage,
} from "./calendarMath";
import "./dateField.css";
import { portalHost } from "./portalHost";

// A date field with our own calendar behind the calendar button.
//
// The native <input type="date"> can be styled down to its box, but the panel it opens is
// browser chrome - a different shape, a different accent, and a different idea of where the
// week starts on every platform. This keeps the typing (it is still a real date input, so the
// keyboard, paste and the y/m/d segments behave exactly as people expect) and replaces only
// the panel, which is the part that looked foreign.
//
// The native indicator is hidden in dateField.css and this button put in its place: opening
// the browser's picker as well would mean two calendars for one field.
//
// `onChange` takes the native (event) shape every other field in this app uses, so callers do
// not have to learn a second convention.
// `mode="month"` makes it a month picker: it holds "YYYY-MM", opens on the months grid and
// commits there. Same panel and the same year step above it, rather than a second component
// with its own idea of what a calendar looks like.
const DateField = ({ value = "", onChange, name, className, style, disabled, mode = "date", ...rest }) => {
    const monthly = mode === "month";
    const [open, setOpen] = useState(false);
    const [position, setPosition] = useState(null);
    // Which month the grid shows - the selected date, or today when nothing is set yet.
    const [view, setView] = useState(() => parseISO(value) || parseMonthISO(value) || todayParts());
    // Which of the three grids is showing. Year is the outermost: picking one drops to that
    // year's months, picking a month drops to its days. Going the other way is what the two
    // halves of the title do - "August" opens the months, "2026" opens the years - so the
    // panel narrows and widens through the same three steps in both directions.
    const [level, setLevel] = useState(monthly ? "months" : "days");
    const wrapRef = useRef(null);
    const panelRef = useRef(null);

    const selected = monthly ? parseMonthISO(value) : parseISO(value);
    const today = todayParts();

    useEffect(() => {
        if (!open) return;
        // Reopening lands on the value's month, not wherever paging left it last time.
        setView(parseISO(value) || parseMonthISO(value) || todayParts());
        setLevel(monthly ? "months" : "days");
        const rect = wrapRef.current.getBoundingClientRect();
        // Flips above the field when there is no room below it.
        const below = window.innerHeight - rect.bottom;
        setPosition({
            top: below < 320 && rect.top > 320 ? rect.top - 8 - 316 : rect.bottom + 6,
            left: Math.min(rect.left, window.innerWidth - 300),
        });
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [open]);

    useEffect(() => {
        if (!open) return;
        const close = () => setOpen(false);
        const onOutside = (e) => {
            if (panelRef.current?.contains(e.target) || wrapRef.current?.contains(e.target)) return;
            close();
        };
        const onKeyDown = (e) => {
            if (e.key === "Escape") close();
        };
        document.addEventListener("mousedown", onOutside);
        document.addEventListener("keydown", onKeyDown);
        window.addEventListener("resize", close);
        // Capture phase, so scrolling any ancestor closes it rather than leaving the panel
        // floating away from the field it belongs to.
        window.addEventListener("scroll", close, true);
        return () => {
            document.removeEventListener("mousedown", onOutside);
            document.removeEventListener("keydown", onKeyDown);
            window.removeEventListener("resize", close);
            window.removeEventListener("scroll", close, true);
        };
    }, [open]);

    const emit = (next) => onChange?.({ target: { name, value: next } });

    const pick = (day) => {
        emit(toISO({ year: view.year, month: view.month, day }));
        setOpen(false);
    };

    const cells = monthGrid(view.year, view.month);
    const years = yearPage(view.year);

    // The chevrons step whatever is on screen: a month, a year, or a block of years.
    const step = (delta) => {
        if (level === "days") return setView((v) => ({ ...addMonths(v, delta), day: v.day }));
        if (level === "months") return setView((v) => ({ ...v, year: v.year + delta }));
        return setView((v) => ({ ...v, year: v.year + delta * YEARS_PER_PAGE }));
    };

    const heading =
        level === "days"
            ? `${MONTHS[view.month]} ${view.year}`
            : level === "months"
            ? String(view.year)
            : `${years[0]} - ${years[years.length - 1]}`;

    return (
        <div className={["date-field", className].filter(Boolean).join(" ")} style={style} ref={wrapRef}>
            <input
                type={monthly ? "month" : "date"}
                name={name}
                value={value}
                onChange={onChange}
                disabled={disabled}
                className="date-field-input"
                {...rest}
            />
            <button
                type="button"
                className="date-field-trigger"
                aria-label="Open calendar"
                aria-haspopup="dialog"
                aria-expanded={open}
                disabled={disabled}
                onClick={() => setOpen((v) => !v)}
            >
                <Calendar size={14} />
            </button>

            {open &&
                position &&
                createPortal(
                    <div
                        ref={panelRef}
                        className="date-panel"
                        role="dialog"
                        aria-label="Choose a date"
                        style={{ top: position.top, left: position.left }}
                    >
                        <div className="date-panel-head">
                            <button type="button" className="shell-icon-btn" aria-label="Previous" onClick={() => step(-1)}>
                                <ChevronLeft size={15} />
                            </button>
                            {/* The title is the way back up. On days it is two targets, so the
                                month and the year can each be opened directly. */}
                            {level === "days" ? (
                                <span className="date-panel-title">
                                    <button type="button" className="date-panel-crumb" onClick={() => setLevel("months")}>
                                        {MONTHS[view.month]}
                                    </button>
                                    <button type="button" className="date-panel-crumb" onClick={() => setLevel("years")}>
                                        {view.year}
                                    </button>
                                </span>
                            ) : (
                                <span className="date-panel-title">
                                    <button
                                        type="button"
                                        className="date-panel-crumb"
                                        // Already at the top - the years grid has nowhere further
                                        // out to go, so its heading is a label, not a button.
                                        disabled={level === "years"}
                                        onClick={() => setLevel("years")}
                                    >
                                        {heading}
                                    </button>
                                </span>
                            )}
                            <button type="button" className="shell-icon-btn" aria-label="Next" onClick={() => step(1)}>
                                <ChevronRight size={15} />
                            </button>
                        </div>

                        {level === "days" && (
                            <>
                                <div className="date-panel-grid date-panel-weekdays">
                                    {WEEKDAYS.map((d) => (
                                        <span key={d}>{d}</span>
                                    ))}
                                </div>

                                <div className="date-panel-grid">
                                    {cells.map((day, i) => {
                                        if (day === null) return <span key={i} />;
                                        const parts = { year: view.year, month: view.month, day };
                                        return (
                                            <button
                                                key={i}
                                                type="button"
                                                className={[
                                                    "date-panel-day",
                                                    isSameDay(parts, selected) ? "is-selected" : "",
                                                    isSameDay(parts, today) ? "is-today" : "",
                                                ]
                                                    .filter(Boolean)
                                                    .join(" ")}
                                                aria-current={isSameDay(parts, today) ? "date" : undefined}
                                                onClick={() => pick(day)}
                                            >
                                                {day}
                                            </button>
                                        );
                                    })}
                                </div>
                            </>
                        )}

                        {level === "months" && (
                            <div className="date-panel-grid date-panel-grid-3">
                                {MONTHS_SHORT.map((label, month) => (
                                    <button
                                        key={label}
                                        type="button"
                                        className={[
                                            "date-panel-cell",
                                            selected && selected.year === view.year && selected.month === month ? "is-selected" : "",
                                            today.year === view.year && today.month === month ? "is-today" : "",
                                        ]
                                            .filter(Boolean)
                                            .join(" ")}
                                        onClick={() => {
                                            // In month mode the month IS the value. Otherwise it
                                            // is a step down - a month on its own is not a date,
                                            // so nothing is emitted until a day is picked.
                                            if (monthly) {
                                                emit(toMonthISO({ year: view.year, month }));
                                                setOpen(false);
                                                return;
                                            }
                                            setView((v) => ({ ...v, month }));
                                            setLevel("days");
                                        }}
                                    >
                                        {label}
                                    </button>
                                ))}
                            </div>
                        )}

                        {level === "years" && (
                            <div className="date-panel-grid date-panel-grid-3">
                                {years.map((year) => (
                                    <button
                                        key={year}
                                        type="button"
                                        className={[
                                            "date-panel-cell",
                                            selected && selected.year === year ? "is-selected" : "",
                                            today.year === year ? "is-today" : "",
                                        ]
                                            .filter(Boolean)
                                            .join(" ")}
                                        onClick={() => {
                                            setView((v) => ({ ...v, year }));
                                            setLevel("months");
                                        }}
                                    >
                                        {year}
                                    </button>
                                ))}
                            </div>
                        )}

                        <div className="date-panel-foot">
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary"
                                onClick={() => {
                                    emit(monthly ? toMonthISO(today) : toISO(today));
                                    setOpen(false);
                                }}
                            >
                                {monthly ? "This month" : "Today"}
                            </button>
                            <span className="date-panel-value">
                                {(monthly ? formatMonthDisplay(value) : formatDisplay(value)) || (monthly ? "No month" : "No date")}
                            </span>
                        </div>
                    </div>,
                    portalHost()
                )}
        </div>
    );
};

export default DateField;
