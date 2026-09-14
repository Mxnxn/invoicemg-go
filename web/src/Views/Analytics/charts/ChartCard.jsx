import { useState } from "react";
import { BarChart2, Table as TableIcon } from "react-feather";

import "./charts.css";

/**
 * The frame every chart sits in.
 *
 * It owns loading, empty and error so seven panels cannot disagree about what "no data"
 * looks like - which is what the old flat grid did, each card solving it differently.
 *
 * It also owns the table toggle, which is not a nicety: three hues in the light palette
 * fall below 3:1 against white, so the numbers have to be reachable without reading
 * colour.
 */
export default function ChartCard({ title, subtitle, loading, error, empty, columns = [], rows = [], children }) {
    const [asTable, setAsTable] = useState(false);

    return (
        <section className="chart-card">
            <header className="chart-card-head">
                <span>
                    <span className="text-heading-brand">{title}</span>
                    {subtitle && <span className="chart-card-sub">{subtitle}</span>}
                </span>
                <button
                    type="button"
                    className="shell-btn shell-btn-secondary chart-card-toggle"
                    aria-pressed={asTable}
                    onClick={() => setAsTable((v) => !v)}
                >
                    {asTable ? <BarChart2 size={14} aria-hidden="true" /> : <TableIcon size={14} aria-hidden="true" />}
                    {asTable ? "Chart" : "Table"}
                </button>
            </header>

            <div className="chart-card-body">
                {error ? (
                    <p className="chart-card-state">{error}</p>
                ) : loading ? (
                    <p className="chart-card-state">Loading…</p>
                ) : empty ? (
                    <p className="chart-card-state">Nothing to show for this period.</p>
                ) : asTable ? (
                    <table className="chart-card-table">
                        <thead>
                            <tr>
                                {columns.map((c) => (
                                    <th key={c.key} scope="col">
                                        {c.label}
                                    </th>
                                ))}
                            </tr>
                        </thead>
                        <tbody>
                            {rows.map((row, i) => (
                                <tr key={row.label ?? row.name ?? i}>
                                    {columns.map((c) => (
                                        <td key={c.key}>{row[c.key]}</td>
                                    ))}
                                </tr>
                            ))}
                        </tbody>
                    </table>
                ) : (
                    children
                )}
            </div>
        </section>
    );
}
