import React, { useState, useEffect, useCallback } from "react";
import { AlertCircle } from "react-feather";
import StatCard from "../../../Shell/StatCard";
import { analyticsBackend } from "../analytics_backend";

const AvgPendingTimeCard = ({ uid, stoken , standalone = true }) => {
    const [avgDays, setAvgDays] = useState(0);
    const [count, setCount] = useState(0);
    const [loading, setLoading] = useState(false);

    const getAvgPendingTime = useCallback(async () => {
        setLoading(false);
        try {
            const formData = new FormData();
            formData.set("uid", uid);
            const res = await analyticsBackend.getAvgPendingTime(formData, stoken);
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
        getAvgPendingTime();
    }, [getAvgPendingTime]);

    return (
        <StatCard
            // Inside BottleneckMetrics' shared row the border/radius come from the row
            // itself, so standalone is opt-out there.
            standalone={standalone}
            icon={AlertCircle}
            label="Avg. Time Pending"
            value={loading && count > 0 ? `${avgDays}d` : "-"}
            sub={!loading ? "Loading..." : count === 0 ? "No data yet" : `Across ${count} outstanding invoice${count === 1 ? "" : "s"}`}
        />
    );
};

export default AvgPendingTimeCard;
