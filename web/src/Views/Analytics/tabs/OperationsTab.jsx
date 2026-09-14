import { useEffect, useState } from "react";

import StatCard from "../../../Shell/StatCard";
import ChartCard from "../charts/ChartCard";
import CategoryChart from "../charts/CategoryChart";
import BottleneckMetrics from "../component/BottleneckMetrics";
import ReviewsPanel from "../component/ReviewsPanel";
import { foldSeries } from "../charts/chartTheme";
import { analyticsBackend } from "../analytics_backend";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

const displayName = (row) => row.clientFirm || row.clientName || "Unnamed";

export default function OperationsTab({ uid, stoken }) {
    const [unbilled, setUnbilled] = useState({
        loading: true,
        error: "",
        count: 0,
        totalValue: 0,
        avgAgeDays: 0,
        topClients: [],
    });
    const [pendingDays, setPendingDays] = useState(null);

    useEffect(() => {
        analyticsBackend
            .getUnbilled(new FormData(), stoken)
            .then((res) =>
                setUnbilled({
                    loading: false,
                    error: "",
                    count: res.count || 0,
                    totalValue: res.totalValue || 0,
                    avgAgeDays: res.avgAgeDays || 0,
                    topClients: res.topClients || [],
                })
            )
            .catch(() => setUnbilled((p) => ({ ...p, loading: false, error: "Could not load unbilled work." })));

        analyticsBackend
            .getAvgPendingTime(new FormData(), stoken)
            // The route answers { data: { avgDays, count } }, not a bare `days`.
            .then((res) => setPendingDays(res.data?.avgDays ?? null))
            .catch(() => setPendingDays(null));
    }, [stoken]);

    const folded = foldSeries(
        unbilled.topClients.map((row) => ({ name: displayName(row), value: Number(row.value) || 0 })),
        8
    );
    const chart = [...folded.shown, ...(folded.other ? [folded.other] : [])];

    return (
        <>
            <div className="analytics-kpis">
                <StatCard label="Unbilled entries" value={String(unbilled.count)} />
                <StatCard label="Unbilled value" value={money(unbilled.totalValue)} />
                <StatCard label="Avg age (days)" value={String(unbilled.avgAgeDays)} />
                <StatCard label="Avg pending (days)" value={pendingDays === null ? "—" : String(pendingDays)} />
            </div>

            <div className="analytics-grid">
                <ChartCard
                    title="Unbilled work by customer"
                    subtitle="Delivered but not yet invoiced"
                    loading={unbilled.loading}
                    error={unbilled.error}
                    empty={!unbilled.loading && !unbilled.error && chart.length === 0}
                    columns={[
                        { key: "name", label: "Customer" },
                        { key: "value", label: "Value" },
                    ]}
                    rows={chart}
                >
                    <CategoryChart data={chart} xKey="name" layout="vertical" series={[{ key: "value", label: "Value" }]} />
                </ChartCard>

                <BottleneckMetrics uid={uid} stoken={stoken} />

                {/* Operations, not Customers: this is how the work was received - quality,
                    speed, communication - which is the same question the bottleneck and
                    unbilled panels beside it ask from the inside. */}
                <ReviewsPanel stoken={stoken} />
            </div>
        </>
    );
}
