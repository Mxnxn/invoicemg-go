import React, { useEffect, useMemo, useState } from "react";
import { Input } from "reactstrap";
import { Search, RefreshCw, CheckCircle, AlertTriangle, Link2 } from "react-feather";
import { whatsappBackend } from "../../Common/whatsapp_backend";
import "./templatePicker.css";
// Brings .xan-shimmer-cell in - the same placeholder the tables use, so the app has one
// shimmer look rather than two.
import "../../Common/DataTable/dataTable.css";

// The templates approved on this company's WhatsApp Business Account.
//
// Worth showing rather than leaving people to Meta's dashboard: what a template needs in
// order to SEND - how many body variables, whether its button takes a link - is invisible
// from the name alone, and getting either wrong is a rejected send discovered only by
// trying. So each row states its own requirements.
//
// Read live on open. Approval state changes on Meta's side without telling us, and a list
// that says APPROVED about a template Meta has since paused is worse than no list.
// `onLoaded` hands the approved list back to the parent, so the template MAPPINGS beside it
// can offer the same names this list shows rather than fetching them a second time.
const TemplatePicker = ({ selected, onSelect, onLoaded }) => {
    const [templates, setTemplates] = useState([]);
    const [query, setQuery] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");

    const load = async () => {
        setLoading(true);
        setError("");
        try {
            const res = await whatsappBackend.listTemplates();
            setTemplates(res.data || []);
            onLoaded?.(res.data || []);
        } catch (err) {
            setError(err?.message || "Couldn't load templates.");
            setTemplates([]);
            onLoaded?.([]);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        load();
    }, []);

    // Name, body and category all searched: people look for "the pickup one" as often as
    // they look for its actual name.
    const filtered = useMemo(() => {
        const needle = query.trim().toLowerCase();
        if (!needle) return templates;
        return templates.filter((t) =>
            [t.name, t.bodyText, t.category, t.language].filter(Boolean).join(" ").toLowerCase().includes(needle)
        );
    }, [templates, query]);

    return (
        <div className="tpl-picker">
            <div className="tpl-picker-head">
                <div className="tpl-search">
                    <Search size={14} aria-hidden="true" />
                    <Input
                        type="search"
                        value={query}
                        onChange={(e) => setQuery(e.target.value)}
                        placeholder="Search templates"
                        aria-label="Search templates"
                    />
                </div>
                <button type="button" className="shell-btn shell-btn-secondary" onClick={load} disabled={loading}>
                    <RefreshCw size={14} aria-hidden="true" />
                    {loading ? "Loading…" : "Refresh"}
                </button>
            </div>

            {error && (
                <p className="tpl-empty" role="status">
                    {error}
                </p>
            )}

            {!error && !loading && filtered.length === 0 && (
                <p className="tpl-empty" role="status">
                    {templates.length === 0 ? "No templates on this account yet." : "No template matches that search."}
                </p>
            )}

            {/* Only when the list is empty. A refresh of an already-loaded list should not
                blank out what is on screen - that reads as the page having lost the data. */}
            {loading && templates.length === 0 && (
                <ul className="tpl-list" data-testid="tpl-skeleton" aria-hidden="true">
                    {Array.from({ length: 4 }).map((_, i) => (
                        <li key={i} className="tpl-skeleton-row">
                            <span className="xan-shimmer-cell tpl-skeleton-name" />
                            <span className="xan-shimmer-cell tpl-skeleton-body" />
                            <span className="xan-shimmer-cell tpl-skeleton-meta" />
                        </li>
                    ))}
                </ul>
            )}

            <ul className="tpl-list">
                {filtered.map((t) => {
                    const isSelected = selected === t.name;
                    const approved = t.status === "APPROVED";
                    return (
                        <li key={`${t.id}-${t.language}`}>
                            <button
                                type="button"
                                className={["tpl-row", isSelected ? "is-selected" : ""].filter(Boolean).join(" ")}
                                aria-pressed={isSelected}
                                onClick={() => onSelect?.(t)}
                            >
                                <span className="tpl-row-head">
                                    <span className="tpl-name">{t.name}</span>
                                    <span className={`tpl-status tpl-status-${approved ? "ok" : "warn"}`}>
                                        {approved ? <CheckCircle size={12} /> : <AlertTriangle size={12} />}
                                        {t.status}
                                    </span>
                                </span>
                                {t.headerText && <span className="tpl-header">{t.headerText}</span>}
                                {t.bodyText && <span className="tpl-body">{t.bodyText}</span>}
                                <span className="tpl-meta">
                                    <span>{t.language}</span>
                                    <span>{t.category}</span>
                                    {/* The two things that decide whether a send succeeds. */}
                                    <span>
                                        {t.bodyVariables} {t.bodyVariables === 1 ? "variable" : "variables"}
                                    </span>
                                    {t.hasUrlButton && (
                                        <span className="tpl-meta-link">
                                            <Link2 size={11} aria-hidden="true" />
                                            {t.urlButtonHasVariable ? "link button (dynamic)" : "link button"}
                                        </span>
                                    )}
                                </span>
                            </button>
                        </li>
                    );
                })}
            </ul>
        </div>
    );
};

export default TemplatePicker;
