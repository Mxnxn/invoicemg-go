import { useEffect, useState } from "react";

import StatCard from "../../../Shell/StatCard";
import ChartCard from "../charts/ChartCard";
import TimeSeriesChart from "../charts/TimeSeriesChart";
import CategoryChart from "../charts/CategoryChart";
import { netPosition } from "../analyticsMath";
import { analyticsBackend } from "../analytics_backend";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// Anything past the first bucket is late. The buckets themselves come from /aging.
const CURRENT_BUCKET = "0-30 days";

export default function CashflowTab({ stoken, source = "invoiced" }) {
    const [flow, setFlow] = useState({ loading: true, error: "", series: [] });
    const [aging, setAging] = useState({ loading: true, error: "", data: [], total: 0 });
    const [payables, setPayables] = useState({ total: 0 });
    const [avgDays, setAvgDays] = useState(null);
    const [payout, setPayout] = useState({ loading: true, error: "", data: [], periodLabel: "" });

    useEffect(() => {
        analyticsBackend
            .cashflow(new FormData(), stoken)
            .then((res) => setFlow({ loading: false, error: "", series: res.data?.series || [] }))
            .catch(() => setFlow({ loading: false, error: "Could not load cashflow.", series: [] }));

        analyticsBackend
            .getAging(new FormData(), stoken)
            .then((res) => setAging({ loading: false, error: "", data: res.data || [], total: res.totalOutstanding || 0 }))
            .catch(() => setAging({ loading: false, error: "Could not load aging.", data: [], total: 0 }));

        analyticsBackend
            .getPayables(new FormData(), stoken)
            .then((res) => setPayables({ total: res.totalPayable || 0 }))
            .catch(() => setPayables({ total: 0 }));

        analyticsBackend
            .getAvgPaymentTime(new FormData(), stoken)
            // The route answers { data: { avgDays, count } }, not a bare `days`.
            .then((res) => setAvgDays(res.data?.avgDays ?? null))
            .catch(() => setAvgDays(null));

        // This route is the one that insists on a period: it answers 422 without a year
        // and month, so the current month is passed explicitly rather than defaulted.
        const now = new Date();
        const payoutForm = new FormData();
        payoutForm.set("year", String(now.getFullYear()));
        payoutForm.set("month", String(now.getMonth()));
        analyticsBackend
            .getPayoutWeekday(payoutForm, stoken)
            .then((res) =>
                setPayout({ loading: false, error: "", data: res.data || [], periodLabel: res.periodLabel || "" })
            )
            .catch(() => setPayout({ loading: false, error: "Could not load payout days.", data: [], periodLabel: "" }));
    }, [stoken]);

    const overdue = aging.data
        .filter((b) => b.label !== CURRENT_BUCKET)
        .reduce((sum, b) => sum + (Number(b.amount) || 0), 0);

    return (
        <>
            <div className="analytics-kpis">
                <StatCard label="Outstanding" value={money(aging.total)} />
                <StatCard label="Overdue" value={money(overdue)} />
                <StatCard label="Payables" value={money(payables.total)} />
                <StatCard label="Net position" value={money(netPosition(aging.total, payables.total))} />
                <StatCard label="Avg days to payment" value={avgDays === null ? "—" : String(avgDays)} />
            </div>

            <div className="analytics-grid">
                <ChartCard
                    title="Cash in and out"
                    subtitle="Money actually received and paid"
                    loading={flow.loading}
                    error={flow.error}
                    empty={!flow.loading && !flow.error && flow.series.length === 0}
                    columns={[
                        { key: "month", label: "Month" },
                        { key: "cashIn", label: "In" },
                        { key: "cashOut", label: "Out" },
                    ]}
                    rows={flow.series}
                >
                    <TimeSeriesChart
                        data={flow.series}
                        xKey="month"
                        series={[
                            { key: "cashIn", label: "In" },
                            { key: "cashOut", label: "Out" },
                        ]}
                    />
                </ChartCard>

                <ChartCard
                    title="Receivables aging"
                    subtitle="What is owed, by how long it has been owed"
                    loading={aging.loading}
                    error={aging.error}
                    empty={!aging.loading && !aging.error && aging.data.length === 0}
                    columns={[
                        { key: "label", label: "Bucket" },
                        { key: "amount", label: "Amount" },
                        { key: "count", label: "Invoices" },
                    ]}
                    rows={aging.data}
                >
                    <CategoryChart data={aging.data} xKey="label" series={[{ key: "amount", label: "Amount" }]} />
                </ChartCard>
                <ChartCard
                    title="Payments by weekday"
                    subtitle={payout.periodLabel || "This month"}
                    loading={payout.loading}
                    error={payout.error}
                    empty={!payout.loading && !payout.error && payout.data.every((d) => !d.amount)}
                    columns={[
                        { key: "label", label: "Day" },
                        { key: "amount", label: "Received" },
                    ]}
                    rows={payout.data}
                >
                    <CategoryChart data={payout.data} xKey="label" series={[{ key: "amount", label: "Received" }]} />
                </ChartCard>
            </div>

            {source === "all" && (
                // Uninvoiced work has not been billed, let alone paid. Letting the switch
                // move a cash figure would be the dashboard telling a lie.
                <p className="analytics-note">
                    Cashflow figures are always invoiced — uninvoiced work has not been billed, so it cannot appear
                    as money received or owed.
                </p>
            )}
        </>
    );
}
