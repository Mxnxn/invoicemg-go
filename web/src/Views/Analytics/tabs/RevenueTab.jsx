import { useEffect, useState } from "react";
import { ChevronLeft } from "react-feather";

import StatCard from "../../../Shell/StatCard";
import ChartCard from "../charts/ChartCard";
import TimeSeriesChart from "../charts/TimeSeriesChart";
import CategoryChart from "../charts/CategoryChart";
import { collectionRate } from "../analyticsMath";
import { analyticsBackend } from "../analytics_backend";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// The three shapes routes/Analytics.js understands. Anything else is refused with a 422.
const PERIODS = [
    { id: "weekly", label: "Weekly" },
    { id: "monthly", label: "Monthly" },
    { id: "yearly", label: "Yearly" },
];

const MONTH_ABBR = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export default function RevenueTab({ stoken, source = "invoiced" }) {
    const [revenue, setRevenue] = useState({ loading: true, error: "", data: [] });
    const [margins, setMargins] = useState({ loading: true, error: "", best: [] });
    // Monthly by default - the route's own fallback shape, and the span most questions about
    // revenue are asked over.
    const [period, setPeriod] = useState("monthly");
    // The drill-down. Clicking a year bar asks for that year's months; clicking a month bar
    // asks for that month's weeks. null means the unfocused, trailing-range view.
    const [focus, setFocus] = useState({ year: null, month: null });

    useEffect(() => {
        const form = new FormData();
        // /revenue is the one endpoint that honours the navbar's Invoiced/All switch.
        form.set("source", source);
        // REQUIRED by routes/Analytics.js - without it the route answers
        // { code: 422, message: "Invalid request." } and the chart never loads.
        form.set("period", period);
        // Only when drilled in. Sending focusYear: null would have the route parseInt("null")
        // into NaN and select nothing.
        if (focus.year !== null) form.set("focusYear", String(focus.year));
        if (focus.month !== null) form.set("focusMonth", String(focus.month));

        analyticsBackend
            .getRevenue(form, stoken)
            .then((res) => setRevenue({ loading: false, error: "", data: res.data || [] }))
            .catch(() => setRevenue({ loading: false, error: "Could not load revenue.", data: [] }));

        analyticsBackend
            .cashflow(new FormData(), stoken)
            .then((res) => setMargins({ loading: false, error: "", best: res.data?.bestMargins || [] }))
            .catch(() => setMargins({ loading: false, error: "Could not load margins.", best: [] }));
    }, [stoken, source, period, focus]);

    // Changing the period always clears the drill-down. Carrying a focusYear from a monthly
    // drill into a yearly request asks the route for "the months of 2025" while the tab is
    // claiming to show years - the chart and its own label would disagree.
    const changePeriod = (next) => {
        setFocus({ year: null, month: null });
        setPeriod(next);
    };

    // One step back out of a drill-down, to where the click came from.
    const goBack = () => {
        if (period === "weekly" && focus.month !== null) {
            setFocus({ year: focus.year, month: null });
            setPeriod("monthly");
        } else if (period === "monthly" && focus.year !== null) {
            setFocus({ year: null, month: null });
            setPeriod("yearly");
        }
    };

    // Clicking a bar drills one level in. Yearly -> that year's months -> that month's weeks.
    // Weekly is the floor; there is nothing below it to ask for.
    const onBarClick = (label) => {
        if (!label) return;
        if (period === "yearly") {
            setFocus({ year: parseInt(label, 10), month: null });
            setPeriod("monthly");
        } else if (period === "monthly") {
            // Labels come back as "Oct 2023" - see getMonthlyBuckets in Helpers/DateBuckets.
            const [abbr, year] = String(label).split(" ");
            const month = MONTH_ABBR.indexOf(abbr);
            if (month < 0 || !year) return;
            setFocus({ year: parseInt(year, 10), month });
            setPeriod("weekly");
        }
    };

    const drilled = focus.year !== null;

    const all = source === "all";
    // Under "all" these are production and receipts against entries, not billed revenue -
    // the same relabelling the old RevenueCard did, for the same reason: calling
    // uninvoiced work "billed" would put a number on screen that the books disagree with.
    const billedLabel = all ? "Production value" : "Billed";
    const collectedLabel = all ? "Paid on entries" : "Collected";

    const billed = revenue.data.reduce((sum, r) => sum + (Number(r.billed) || 0), 0);
    const collected = revenue.data.reduce((sum, r) => sum + (Number(r.collected) || 0), 0);
    const periods = revenue.data.length;

    // What the drill-down is currently showing, said in words. A chart whose bars changed
    // meaning with no label is the thing that makes drill-downs confusing.
    const focusLabel = !drilled
        ? ""
        : focus.month !== null
        ? `${MONTH_ABBR[focus.month]} ${focus.year}`
        : String(focus.year);

    return (
        <>
            <div className="revenue-periods">
                <div className="shell-segmented" role="tablist" aria-label="Revenue period">
                    {PERIODS.map((p) => (
                        <button
                            key={p.id}
                            type="button"
                            role="tab"
                            aria-selected={period === p.id}
                            className="shell-segmented-btn"
                            style={period === p.id ? { background: "var(--xan-blue-bg)", color: "var(--xan-blue)" } : undefined}
                            onClick={() => changePeriod(p.id)}
                        >
                            {p.label}
                        </button>
                    ))}
                </div>
                {drilled && (
                    <button type="button" className="shell-btn shell-btn-sm revenue-back" onClick={goBack}>
                        <ChevronLeft size={14} aria-hidden="true" />
                        Back from {focusLabel}
                    </button>
                )}
            </div>

            <div className="analytics-kpis">
                <StatCard label={billedLabel} value={money(billed)} />
                <StatCard label={collectedLabel} value={money(collected)} />
                <StatCard label="Collection rate" value={`${collectionRate(revenue.data)}%`} />
                <StatCard label="Avg per period" value={money(periods ? billed / periods : 0)} />
            </div>

            <div className="analytics-grid">
                {/* Both series are money, so they share one axis - two series, not a
                    dual-axis chart. */}
                <ChartCard
                    title="Revenue over time"
                    subtitle={
                        drilled
                            ? `${billedLabel} against ${collectedLabel.toLowerCase()} · ${focusLabel}`
                            : `${billedLabel} against ${collectedLabel.toLowerCase()}`
                    }
                    loading={revenue.loading}
                    error={revenue.error}
                    empty={!revenue.loading && !revenue.error && revenue.data.length === 0}
                    columns={[
                        { key: "label", label: "Period" },
                        { key: "billed", label: billedLabel },
                        { key: "collected", label: collectedLabel },
                    ]}
                    rows={revenue.data}
                >
                    <TimeSeriesChart
                        data={revenue.data}
                        xKey="label"
                        series={[
                            { key: "billed", label: billedLabel },
                            { key: "collected", label: collectedLabel },
                        ]}
                        // Weekly is the floor - there is nothing below it to drill into, so
                        // the handler is withheld and the chart stops offering a pointer.
                        onPointClick={period === "weekly" ? undefined : onBarClick}
                    />
                </ChartCard>

                <ChartCard
                    title="Best margins by product"
                    subtitle="Always invoiced figures"
                    loading={margins.loading}
                    error={margins.error}
                    empty={!margins.loading && !margins.error && margins.best.length === 0}
                    columns={[
                        { key: "name", label: "Product" },
                        { key: "margin", label: "Margin" },
                    ]}
                    rows={margins.best}
                >
                    <CategoryChart
                        data={margins.best}
                        xKey="name"
                        layout="vertical"
                        series={[{ key: "margin", label: "Margin" }]}
                    />
                </ChartCard>
            </div>

            {all && (
                // The switch reaches revenue only. Saying so beats a page where one panel
                // quietly obeys a control the others ignore.
                <p className="analytics-note">
                    Showing production value. Margins below remain invoiced-only — that endpoint does not yet
                    distinguish uninvoiced work.
                </p>
            )}
        </>
    );
}
