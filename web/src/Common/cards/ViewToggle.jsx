import React from "react";
import { List, Grid } from "react-feather";

/**
 * Table or Board, for a list that can be read either way.
 *
 * Table is always first and always the default: these lists have been tables for as long as
 * the app has existed, and a view someone relies on should not be replaced by a new one
 * because the new one is nicer. The board is a second way to look, offered beside it.
 *
 * The choice is remembered per list, in localStorage - somebody who prefers one has that
 * preference about invoices and quotations separately, and being asked again on every visit is
 * the fastest way to make them stop using it.
 */
export const VIEW_TABLE = "table";
export const VIEW_BOARD = "board";

export const readView = (key) => {
    try {
        return window.localStorage.getItem(`view:${key}`) === VIEW_BOARD ? VIEW_BOARD : VIEW_TABLE;
    } catch (error) {
        // Private windows and blocked site data throw on read. A default view is not worth
        // taking the page down for.
        return VIEW_TABLE;
    }
};

export const writeView = (key, view) => {
    try {
        window.localStorage.setItem(`view:${key}`, view);
    } catch (error) {
        // Same again: the preference is a convenience, not state the page depends on.
    }
};

export default function ViewToggle({ view, onChange, storageKey }) {
    const pick = (next) => {
        if (next === view) return;
        if (storageKey) writeView(storageKey, next);
        onChange(next);
    };

    return (
        <div className="shell-segmented" role="tablist">
            {/* Icon AND label, never icon alone. The shape is read faster than the word - a
                stack of rows and a grid of squares are what these two views literally look
                like - but an icon on its own is a guess, and this app is used by people who
                are not going to enjoy guessing. The label is the meaning; the icon is the
                shortcut to it. */}
            {[
                { key: VIEW_TABLE, label: "Table", Icon: List },
                { key: VIEW_BOARD, label: "Board", Icon: Grid },
            ].map((t) => (
                <button
                    key={t.key}
                    type="button"
                    role="tab"
                    aria-selected={view === t.key}
                    className={`shell-segmented-btn${view === t.key ? " active" : ""}`}
                    onClick={() => pick(t.key)}
                >
                    <t.Icon size={13} aria-hidden="true" />
                    {t.label}
                </button>
            ))}
        </div>
    );
}
