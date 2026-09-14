import "./stageAgingBars.css";

// Sequential, oldest last. The order is the meaning, so it is fixed here rather than
// derived from whatever key order the API happened to serialise.
const AGE_BUCKETS = ["0-2d", "3-7d", "8-14d", "15+d"];
const BUCKET_VAR = {
    "0-2d": "var(--age-1)",
    "3-7d": "var(--age-2)",
    "8-14d": "var(--age-3)",
    "15+d": "var(--age-4)",
};

/**
 * Work waiting, as one bar per row: length is how many cards, shade is how long they have
 * waited. The longest bar and the darkest bar are usually different rows, and that gap is
 * the whole point of the panel - the biggest pile and the stalest pile are rarely the same
 * place.
 *
 * `footerRow` is rendered below a rule instead of among the others. It exists for
 * "Nobody assigned", which is not a person and must not be ranked as though it were.
 */
export default function StageAgingBars({ rows = [], footerRow = null }) {
    const all = footerRow ? [...rows, footerRow] : rows;
    const max = Math.max(1, ...all.map((r) => r.total || 0));

    const renderRow = (row) => (
        <div className="aging-row" key={row.name}>
            <div className="aging-label text-body-regular">{row.name}</div>
            <div className="aging-bar">
                {AGE_BUCKETS.filter((b) => (row.buckets?.[b] || 0) > 0).map((b) => (
                    <div
                        key={b}
                        className="aging-seg"
                        style={{ width: `${((row.buckets[b] || 0) / max) * 100}%`, background: BUCKET_VAR[b] }}
                        title={`${row.name} — ${row.buckets[b]} card(s), ${b}`}
                    />
                ))}
            </div>
            <div className="aging-total text-body-medium">{row.total}</div>
        </div>
    );

    return (
        <div>
            {rows.map(renderRow)}

            {footerRow && (
                <div className="aging-footer" data-testid="aging-footer">
                    {renderRow(footerRow)}
                </div>
            )}

            {/* Always present: three of these steps fall below 3:1 against the card
                surface, so identity can never rest on colour alone. */}
            <div className="aging-legend text-body-small">
                {AGE_BUCKETS.map((b) => (
                    <span key={b}>
                        <i className="aging-swatch" style={{ background: BUCKET_VAR[b] }} aria-hidden="true" />
                        {b}
                    </span>
                ))}
            </div>
        </div>
    );
}
