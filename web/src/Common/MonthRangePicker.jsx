import React from "react";
import { Calendar, X } from "react-feather";
import DateField from "./DateField";
import { isMonthRangeActive, EMPTY_MONTH_RANGE } from "./monthRange";

// Start-end month picker shared by the Client Entries feed, Purchase Invoices and both Bank
// Transfers tabs. Built on the same DateField as every other date control, in its month mode -
// the native <input type="month"> panel was the one remaining piece of browser chrome in
// these headers.
const MonthRangePicker = ({ value = EMPTY_MONTH_RANGE, onChange, label = "Month range" }) => (
    <div className="d-flex align-items-center" style={{ gap: 6, flexWrap: "wrap" }} aria-label={label}>
        <Calendar size={14} style={{ color: "var(--text-tertiary)" }} />
        <DateField
            mode="month"
            style={{ width: 150 }}
            value={value.from || ""}
            onChange={(e) => onChange({ ...value, from: e.target.value })}
        />
        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
            to
        </span>
        <DateField
            mode="month"
            style={{ width: 150 }}
            value={value.to || ""}
            onChange={(e) => onChange({ ...value, to: e.target.value })}
        />
        {isMonthRangeActive(value) && (
            <button type="button" className="shell-icon-btn" aria-label="Clear month range" onClick={() => onChange(EMPTY_MONTH_RANGE)}>
                <X size={14} />
            </button>
        )}
    </div>
);

export default MonthRangePicker;
