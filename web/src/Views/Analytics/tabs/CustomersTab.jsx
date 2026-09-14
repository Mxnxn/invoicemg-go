import { useEffect, useState } from "react";

import StatCard from "../../../Shell/StatCard";
import ChartCard from "../charts/ChartCard";
import CategoryChart from "../charts/CategoryChart";
import { concentration } from "../analyticsMath";
import { foldSeries } from "../charts/chartTheme";
import { analyticsBackend } from "../analytics_backend";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// The endpoints return clientFirm and clientName separately, and a firm can be blank.
// Falling back keeps a bar from being labelled with nothing at all.
const displayName = (row) => row.clientFirm || row.clientName || "Unnamed";

export default function CustomersTab({ stoken }) {
    const [sales, setSales] = useState({ loading: true, error: "", data: [] });
    const [paid, setPaid] = useState({ loading: true, error: "", data: [] });

    useEffect(() => {
        analyticsBackend
            .getTopSales(new FormData(), stoken)
            .then((res) => setSales({ loading: false, error: "", data: res.data || [] }))
            .catch(() => setSales({ loading: false, error: "Could not load customers.", data: [] }));

        analyticsBackend
            .getTopPaid(new FormData(), stoken)
            .then((res) => setPaid({ loading: false, error: "", data: res.data || [] }))
            .catch(() => setPaid({ loading: false, error: "Could not load payments.", data: [] }));
    }, [stoken]);

    // Eight bars is the palette's limit; the rest becomes one honest "Other" rather than a
    // ninth invented colour.
    const salesFolded = foldSeries(
        sales.data.map((row) => ({ name: displayName(row), value: Number(row.amount) || 0 })),
        8
    );
    const salesChart = [...salesFolded.shown, ...(salesFolded.other ? [salesFolded.other] : [])];

    const paidFolded = foldSeries(
        paid.data.map((row) => ({ name: displayName(row), value: Number(row.paid) || 0 })),
        8
    );
    const paidChart = [...paidFolded.shown, ...(paidFolded.other ? [paidFolded.other] : [])];

    // concentration() reads `total`; the endpoint calls it `amount`.
    const topFive = concentration(
        sales.data.map((row) => ({ total: Number(row.amount) || 0 })),
        5
    );
    const revenue = sales.data.reduce((sum, row) => sum + (Number(row.amount) || 0), 0);

    return (
        <>
            <div className="analytics-kpis">
                <StatCard label="Active customers" value={String(sales.data.length)} />
                <StatCard label="Top-5 concentration" value={`${topFive}%`} />
                <StatCard label="Revenue from customers" value={money(revenue)} />
            </div>

            <div className="analytics-grid">
                <ChartCard
                    title="Top customers by sales"
                    subtitle="Concentration is the risk this chart shows"
                    loading={sales.loading}
                    error={sales.error}
                    empty={!sales.loading && !sales.error && salesChart.length === 0}
                    columns={[
                        { key: "name", label: "Customer" },
                        { key: "value", label: "Sales" },
                    ]}
                    rows={salesChart}
                >
                    <CategoryChart
                        data={salesChart}
                        xKey="name"
                        layout="vertical"
                        series={[{ key: "value", label: "Sales" }]}
                    />
                </ChartCard>

                <ChartCard
                    title="Top customers by payment"
                    loading={paid.loading}
                    error={paid.error}
                    empty={!paid.loading && !paid.error && paidChart.length === 0}
                    columns={[
                        { key: "name", label: "Customer" },
                        { key: "value", label: "Paid" },
                    ]}
                    rows={paidChart}
                >
                    <CategoryChart
                        data={paidChart}
                        xKey="name"
                        layout="vertical"
                        series={[{ key: "value", label: "Paid" }]}
                    />
                </ChartCard>
            </div>
        </>
    );
}
