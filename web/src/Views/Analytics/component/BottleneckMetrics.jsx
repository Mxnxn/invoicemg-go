import React, { useCallback, useEffect, useState } from "react";
import { AlertCircle, Clock, Inbox, Activity } from "react-feather";
import StatCard from "../../../Shell/StatCard";
import AvgPaymentTimeCard from "./AvgPaymentTimeCard";
import AvgPendingTimeCard from "./AvgPendingTimeCard";
import { analyticsBackend } from "../analytics_backend";

const formatCurrency = (n) => `₹${Number(n || 0).toLocaleString()}`;

// Time-to-cash bottleneck indicators: where money is stuck (AR aging) and where
// billing itself is lagging behind completed work (unbilled entries). Both are
// "how long has this been sitting" metrics - the questions a financial-ops
// analyst asks first when chasing down cash flow problems.
const BottleneckMetrics = ({ uid, stoken }) => {
    const [aging, setAging] = useState(null);
    const [unbilled, setUnbilled] = useState(null);
    const [loading, setLoading] = useState(false);

    const load = useCallback(async () => {
        try {
            const formData = new FormData();
            formData.set("uid", uid);
            const [agingRes, unbilledRes] = await Promise.all([
                analyticsBackend.getAging(formData, stoken),
                analyticsBackend.getUnbilled(formData, stoken),
            ]);
            setAging(agingRes);
            setUnbilled(unbilledRes);
        } catch (error) {
            console.log(error);
        } finally {
            setLoading(true);
        }
    }, [uid, stoken]);

    useEffect(() => {
        load();
    }, [load]);

    if (!loading) return null;

    const overdueBucket = aging?.data?.find((b) => b.label === "90+ days");

    return (
        <div className="stat-card-row">
            <StatCard
                accent
                icon={AlertCircle}
                label="Outstanding (AR)"
                value={formatCurrency(aging?.totalOutstanding)}
                sub="Invoiced, not yet collected"
            />
            <StatCard
                accent
                icon={Clock}
                label="Oldest Overdue"
                value={aging?.oldestDays ? `${aging.oldestDays}d` : "0d"}
                sub={overdueBucket && overdueBucket.count > 0 ? `${overdueBucket.count} invoice(s) 90+ days` : "Within 90 days"}
            />
            <StatCard
                accent
                icon={Inbox}
                label="Unbilled Backlog"
                value={formatCurrency(unbilled?.totalValue)}
                sub={`${unbilled?.count || 0} entr${unbilled?.count === 1 ? "y" : "ies"} awaiting invoice`}
            />
            <StatCard
                accent
                icon={Activity}
                label="Avg Billing Lag"
                value={unbilled?.avgAgeDays ? `${unbilled.avgAgeDays}d` : "0d"}
                sub="Time from work done to invoiced"
            />
            {/* The two timing metrics belong with the receivables story, not off in the chart
                grid - all six answer "how long is money taking to arrive". standalone={false}
                so the row's own border wraps them. */}
            <AvgPaymentTimeCard uid={uid} stoken={stoken} standalone={false} />
            <AvgPendingTimeCard uid={uid} stoken={stoken} standalone={false} />
        </div>
    );
};

export default BottleneckMetrics;
