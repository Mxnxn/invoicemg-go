import { useEffect, useState } from "react";

import StatCard from "../../../Shell/StatCard";
import ChartCard from "../charts/ChartCard";
import CategoryChart from "../charts/CategoryChart";
import StageAgingBars from "../component/StageAgingBars";
import { analyticsBackend } from "../analytics_backend";

const days = (n) => (n === null || n === undefined ? "—" : `${n}d`);

const AGE_COLUMNS = [
    { key: "name", label: "Row" },
    { key: "total", label: "Cards" },
    { key: "0-2d", label: "0-2d" },
    { key: "3-7d", label: "3-7d" },
    { key: "8-14d", label: "8-14d" },
    { key: "15+d", label: "15+d" },
];

// ChartCard's table view reads flat keys off each row, so the age buckets are flattened
// up beside the name. Without this the table - which is the required relief for the pale
// ramp steps - would show blanks where the numbers should be.
const flatten = (rows = []) => rows.map((r) => ({ ...r, ...r.buckets }));

export default function ProductionTab({ stoken, source }) {
    const [wip, setWip] = useState({ loading: true, error: "", data: null });
    const [flow, setFlow] = useState({ loading: true, error: "", data: null });

    useEffect(() => {
        analyticsBackend
            .getProductionWip(new FormData(), stoken)
            .then((res) => setWip({ loading: false, error: "", data: res.data }))
            .catch(() => setWip({ loading: false, error: "Could not load work in progress.", data: null }));

        const form = new FormData();
        form.set("period", "weekly");
        analyticsBackend
            .getProductionThroughput(form, stoken)
            .then((res) => setFlow({ loading: false, error: "", data: res.data }))
            .catch(() => setFlow({ loading: false, error: "Could not load throughput.", data: null }));
    }, [stoken]);

    const w = wip.data;
    const f = flow.data;
    const trend = f?.trend || [];
    const thisWeek = trend.length ? trend[trend.length - 1].completed : null;
    const lastWeek = trend.length > 1 ? trend[trend.length - 2].completed : null;

    return (
        <>
            {/* The navbar's Invoiced/All switch reaches every tab, but production work has
                no billing state - a card in Printing is neither invoiced nor not. Per the
                existing Analytics rule, a tab that cannot honour the switch says so on
                screen rather than appearing to respond to it. */}
            <p className="analytics-note">
                The Invoiced/All switch does not apply here — these figures cover all production work.
            </p>

            <div className="analytics-kpis">
                <StatCard label="Open cards" value={w ? String(w.totalOpen) : "—"} sub="Not yet Done" />
                <StatCard
                    label="Oldest waiting"
                    value={w ? days(w.oldestDays) : "—"}
                    sub={w && !w.agesExact ? "Some ages are estimated" : "In its current stage"}
                />
                <StatCard
                    label="Cycle time (median)"
                    value={f ? days(f.cycleTimeDays.median) : "—"}
                    sub={f ? `p90 ${days(f.cycleTimeDays.p90)} · ${f.cycleTimeDays.count} jobs` : ""}
                />
                <StatCard
                    label="Done this week"
                    value={thisWeek === null ? "—" : String(thisWeek)}
                    sub={lastWeek === null ? "" : `${lastWeek} last week`}
                />
            </div>

            <div className="analytics-grid">
                <ChartCard
                    title="Work in progress by stage"
                    subtitle={
                        w && !w.agesExact
                            ? "Bar length is how many cards; shade is how long. Some ages are estimated from when the card was created."
                            : "Bar length is how many cards; shade is how long they have waited."
                    }
                    loading={wip.loading}
                    error={wip.error}
                    empty={!wip.loading && !wip.error && !(w?.stages || []).length}
                    columns={AGE_COLUMNS}
                    rows={flatten(w?.stages)}
                >
                    <StageAgingBars rows={w?.stages || []} />
                </ChartCard>

                <ChartCard
                    title="Work in progress by who's holding it"
                    subtitle="Unassigned is listed separately — it is not a person's backlog."
                    loading={wip.loading}
                    error={wip.error}
                    empty={!wip.loading && !wip.error && !(w?.holders || []).length && !w?.unassigned?.total}
                    columns={AGE_COLUMNS}
                    rows={flatten([...(w?.holders || []), ...(w?.unassigned ? [w.unassigned] : [])])}
                >
                    <StageAgingBars rows={w?.holders || []} footerRow={w?.unassigned || null} />
                </ChartCard>

                <ChartCard
                    title="Cards completed per person"
                    subtitle="Who finished the work, from the job history — not who is holding it now."
                    loading={flow.loading}
                    error={flow.error}
                    empty={!flow.loading && !flow.error && !(f?.perPerson || []).length}
                    columns={[
                        { key: "name", label: "Person" },
                        { key: "completed", label: "Cards done" },
                    ]}
                    rows={f?.perPerson || []}
                >
                    <CategoryChart
                        data={f?.perPerson || []}
                        xKey="name"
                        layout="vertical"
                        series={[{ key: "completed", label: "Cards done" }]}
                    />
                </ChartCard>

                <ChartCard
                    title="Throughput"
                    subtitle="Cards reaching Done each week"
                    loading={flow.loading}
                    error={flow.error}
                    empty={!flow.loading && !flow.error && !trend.length}
                    columns={[
                        { key: "label", label: "Week" },
                        { key: "completed", label: "Cards done" },
                    ]}
                    rows={trend}
                >
                    <CategoryChart data={trend} xKey="label" series={[{ key: "completed", label: "Cards done" }]} />
                </ChartCard>
            </div>
        </>
    );
}
