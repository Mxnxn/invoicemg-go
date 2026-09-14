import React, { useEffect, useState } from "react";
import { Input } from "reactstrap";
import {
    FONT_GROUPS,
    SCALE_ROLES,
    clampTracking,
    scaleToPx,
    pxToScale,
    pxRange,
    TRACKING_MIN,
    TRACKING_MAX,
    readAppearance,
    writeAppearance,
    applyAppearance,
    DEFAULT_APPEARANCE,
    ACCENT_OPTIONS,
    GRADIENT_OPTIONS,
    ROWS_PER_PAGE_OPTIONS,
} from "../../../Common/appearance";
import { pushAppearance } from "../../../Common/appearanceSync";

// Font and per-role text sizes, applied live - a preview that only updates on save makes
// you guess, and this is a preference judged by looking at it.
//
// The preview uses the real utility classes and reports the size the browser actually
// resolved, so what is shown is literally what the rest of the app will render rather than
// an approximation of it.
const AppearanceSettings = () => {
    const [appearance, setAppearance] = useState(() => readAppearance());
    const [resolved, setResolved] = useState({});

    // What the boxes currently show, which is not what is applied until it is committed - a
    // half-typed "1" on the way to "120" would otherwise shrink the whole app to 1%.
    const asDraft = (a) => ({
        ...Object.fromEntries(SCALE_ROLES.map(({ id }) => [id, String(scaleToPx(id, a.scales[id]))])),
        tracking: String(a.tracking),
    });
    const [draft, setDraft] = useState(() => asDraft(readAppearance()));

    const update = (patch) => {
        const next = { ...appearance, ...patch };
        setAppearance(next);
        applyAppearance(next);
        writeAppearance(next);
        // Device first, server after: the change is already visible and saved locally, so
        // the sync is debounced and allowed to fail quietly. It is what carries the
        // preference to the next browser or machine this person logs in from.
        pushAppearance(next);
    };

    // Out-of-range or non-numeric input snaps back to what is actually applied rather than
    // being silently accepted or left showing a value that is not in effect.
    const commitScale = (role) => {
        const typed = Number(draft[role]);
        // A blank or nonsensical entry keeps what is applied rather than resetting to a
        // default the user never asked for.
        const next = Number.isFinite(typed) && typed > 0 ? pxToScale(role, typed) : appearance.scales[role];
        update({ scales: { ...appearance.scales, [role]: next } });
        setDraft((d) => ({ ...d, [role]: String(scaleToPx(role, next)) }));
    };

    const commitTracking = () => {
        const value = Number(draft.tracking);
        const next = Number.isFinite(value) ? value : appearance.tracking;
        update({ tracking: next });
        setDraft((d) => ({ ...d, tracking: String(clampTracking(next)) }));
    };

    const reset = () => {
        const fresh = { ...DEFAULT_APPEARANCE, scales: { ...DEFAULT_APPEARANCE.scales } };
        update(fresh);
        setDraft(asDraft(fresh));
    };

    // Read back the computed px rather than recomputing it here - if the two ever disagree,
    // the number on screen would be the lie.
    useEffect(() => {
        const next = {};
        SCALE_ROLES.forEach(({ id }) => {
            const el = document.querySelector(`[data-preview="${id}"]`);
            if (el) next[id] = Math.round(parseFloat(getComputedStyle(el).fontSize) * 10) / 10;
        });
        setResolved(next);
    }, [appearance]);

    return (
        <div className="shell-card" style={{ height: "100%" }}>
            <div className="shell-card-header">
                <span className="text-heading-brand">Appearance</span>
                {/* Reset sits beside the title rather than in the footer, matching the company
                    stepper opposite it - both act on the whole card, not on one field. */}
                <button type="button" className="shell-btn shell-btn-secondary" onClick={reset}>
                    Reset
                </button>
            </div>

            {/* A grid rather than a single column. The controls stay narrow - a 280px select
                reads better than one stretched across a panel - but on a wide screen they sit
                beside each other instead of leaving two thirds of it empty. */}
            <div className="appearance-grid">
                <div style={{ maxWidth: 280 }}>
                    <label className="form-control-label pp fs-12" htmlFor="appearance-font">
                        Font
                    </label>
                    <Input
                        id="appearance-font"
                        type="select"
                        value={appearance.font}
                        onChange={(e) => update({ font: e.target.value })}
                    >
                        {/* Grouped, and each name set in its own face. Seventeen names in one
                            flat list is a wall, and a font is a thing you choose by looking at
                            it - the same argument the accent swatches below make. Where a
                            browser refuses to style <option>, the names still read plainly. */}
                        {FONT_GROUPS.map((group) => (
                            <optgroup key={group.label} label={group.label}>
                                {group.fonts.map((f) => (
                                    <option key={f.id} value={f.id} style={{ fontFamily: f.stack }}>
                                        {f.label}
                                    </option>
                                ))}
                            </optgroup>
                        ))}
                    </Input>
                </div>

                <div style={{ maxWidth: 280 }}>
                    <label className="form-control-label pp fs-12" htmlFor="appearance-rows">
                        Rows per page
                    </label>
                    <Input
                        id="appearance-rows"
                        type="select"
                        value={String(appearance.rowsPerPage)}
                        onChange={(e) => update({ rowsPerPage: Number(e.target.value) })}
                    >
                        {ROWS_PER_PAGE_OPTIONS.map((o) => (
                            <option key={o.value} value={String(o.value)}>
                                {o.label}
                            </option>
                        ))}
                    </Input>
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        How many rows a table shows before it pages. "All" shows everything.
                    </span>
                </div>

                {/* Accent, as swatches rather than a dropdown - a colour is chosen by looking
                    at it. Picking a gradient clears the flat choice and vice versa: they set
                    the same accent, so offering both at once would leave one of them lit
                    while doing nothing. */}
                <div>
                    <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                        Accent colour
                    </label>
                    <div className="accent-swatches">
                        {ACCENT_OPTIONS.map((a) => (
                            <button
                                key={a.id}
                                type="button"
                                className={[
                                    "accent-swatch",
                                    appearance.gradient === "none" && appearance.accent === a.id ? "is-on" : "",
                                ]
                                    .filter(Boolean)
                                    .join(" ")}
                                style={{ background: a.hex }}
                                aria-label={a.label}
                                aria-pressed={appearance.gradient === "none" && appearance.accent === a.id}
                                title={a.label}
                                onClick={() => update({ accent: a.id, gradient: "none" })}
                            />
                        ))}
                    </div>
                </div>

                <div>
                    <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                        Gradient
                    </label>
                    <div className="accent-swatches">
                        {GRADIENT_OPTIONS.filter((g) => g.id !== "none").map((g) => (
                            <button
                                key={g.id}
                                type="button"
                                className={["accent-swatch", "accent-swatch-wide", appearance.gradient === g.id ? "is-on" : ""]
                                    .filter(Boolean)
                                    .join(" ")}
                                style={{ background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)` }}
                                aria-label={g.label}
                                aria-pressed={appearance.gradient === g.id}
                                title={g.label}
                                onClick={() => update({ gradient: g.id })}
                            />
                        ))}
                    </div>
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Buttons carry the gradient; text and borders use its base colour.
                    </span>
                </div>

                <div>
                    <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                        <label className="form-control-label pp fs-12" htmlFor="appearance-tracking" style={{ margin: 0 }}>
                            Letter spacing
                        </label>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            Table rows
                        </span>
                    </div>
                    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                        <Input
                            id="appearance-tracking"
                            type="number"
                            min={TRACKING_MIN}
                            max={TRACKING_MAX}
                            step="0.05"
                            style={{ maxWidth: 110 }}
                            value={draft.tracking}
                            onChange={(e) => setDraft((d) => ({ ...d, tracking: e.target.value }))}
                            onBlur={commitTracking}
                            onKeyDown={(e) => e.key === "Enter" && commitTracking()}
                        />
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            px
                        </span>
                    </div>
                </div>

                {SCALE_ROLES.map(({ id, label, hint }) => (
                    <div key={id}>
                        <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                            <label className="form-control-label pp fs-12" htmlFor={`appearance-scale-${id}`} style={{ margin: 0 }}>
                                {label}
                            </label>
                            <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                {hint}
                            </span>
                            {/* What the browser actually resolved, which should equal the box.
                                Shown only when they disagree - a silent mismatch would mean
                                the number typed is not the number rendered. */}
                            {resolved[id] && String(resolved[id]) !== draft[id] && (
                                <span className="cell-mono" style={{ marginLeft: "auto", color: "var(--text-tertiary)" }}>
                                    renders {resolved[id]}px
                                </span>
                            )}
                        </div>
                        {/* A typed percentage rather than a slider: you can put back exactly
                            what you had, which dragging cannot. Committed on blur so a
                            half-typed "1" does not briefly shrink the whole app to 1%. */}
                        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                            <Input
                                id={`appearance-scale-${id}`}
                                type="number"
                                min={pxRange(id).min}
                                max={pxRange(id).max}
                                step="0.5"
                                style={{ maxWidth: 110 }}
                                value={draft[id]}
                                onChange={(e) => setDraft((d) => ({ ...d, [id]: e.target.value }))}
                                onBlur={() => commitScale(id)}
                                onKeyDown={(e) => e.key === "Enter" && commitScale(id)}
                            />
                            <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                px ({pxRange(id).min}–{pxRange(id).max})
                            </span>
                        </div>
                    </div>
                ))}
            </div>

            <div style={{ padding: "0 20px 16px" }}>
                <div className="text-label-caps" style={{ color: "var(--text-tertiary)", marginBottom: 6 }}>
                    Preview
                </div>
                <div
                    style={{
                        border: "1px solid var(--border-default)",
                        borderRadius: "var(--radius-control)",
                        padding: 14,
                        background: "var(--bg-field-on-canvas)",
                        display: "grid",
                        gap: 10,
                    }}
                >
                    <div>
                        <div className="text-heading-page" data-preview="heading">
                            Invoice MG/26-27/INV-00001
                        </div>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            Headings — page titles and stat card figures
                        </span>
                    </div>
                    <div>
                        <div className="text-body-regular" data-preview="body">
                            The quick brown fox jumps over the lazy dog — 0123456789
                        </div>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            Body &amp; labels — form labels, sidebar, buttons
                        </span>
                    </div>
                    <div>
                        {/* A real table, so the table scale is shown by the thing it governs. */}
                        <table className="xan-table" style={{ width: "100%" }}>
                            <thead>
                                <tr>
                                    <th scope="col" data-preview="tableHead">Job</th>
                                    <th scope="col">Amount</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr>
                                    <td data-preview="tableRow">MG/26-27/00002</td>
                                    <td className="cell-mono">₹8,745.50</td>
                                </tr>
                            </tbody>
                        </table>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            Table headers and rows size separately
                        </span>
                    </div>
                </div>
            </div>

            <div className="shell-card-footer">
                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    Saved on this device, like the theme.
                </span>
            </div>
        </div>
    );
};

export default AppearanceSettings;
