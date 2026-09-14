import React, { useState, useEffect, useCallback } from "react";
import { Clock } from "react-feather";
import StatCard from "../../../Shell/StatCard";
import { analyticsBackend } from "../analytics_backend";

const AvgPaymentTimeCard = ({ uid, stoken , standalone = true }) => {
    const [avgDays, setAvgDays] = useState(0);
    const [count, setCount] = useState(0);
    const [loading, setLoading] = useState(false);

    const getAvgPaymentTime = useCallback(async () => {
        setLoading(false);
        try {
            const formData = new FormData();
            formData.set("uid", uid);
            const res = await analyticsBackend.getAvgPaymentTime(formData, stoken);
            setAvgDays(res.data.avgDays);
            setCount(res.data.count);
        } catch (error) {
            console.log(error);
            setAvgDays(0);
            setCount(0);
        } finally {
            setLoading(true);
        }
    }, [uid, stoken]);

    useEffect(() => {
        getAvgPaymentTime();
    }, [getAvgPaymentTime]);

    return (
        <StatCard
            // Inside BottleneckMetrics' shared row the border/radius come from the row
            // itself, so standalone is opt-out there.
            standalone={standalone}
            icon={Clock}
            label="Avg. Time to Get Paid"
            value={loading && count > 0 ? `${avgDays}d` : "-"}
            sub={!loading ? "Loading..." : count === 0 ? "No data yet" : `Based on ${count} paid invoice${count === 1 ? "" : "s"}`}
        />
    );
};

export default AvgPaymentTimeCard;
