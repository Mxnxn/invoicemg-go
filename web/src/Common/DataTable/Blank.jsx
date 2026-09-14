import React from "react";
import "./dataTable.css";

/**
 * A value that isn't there.
 *
 * One component rather than a "-" typed into each cell: the app had a mix of hyphens and em
 * dashes in full-strength ink, which made an absent value read with the same weight as a real
 * one - a column of dashes looks like data until you read it.
 *
 * An em dash in receding ink, and hidden from screen readers with the meaning given as words
 * instead: "dash" announced down a column tells nobody anything, where "not set" does.
 */
export default function Blank({ label = "Not set" }) {
    return (
        <span className="xan-blank">
            <span aria-hidden="true">—</span>
            {/* Argon ships .sr-only; no reason for a second visually-hidden helper. */}
            <span className="sr-only">{label}</span>
        </span>
    );
}
