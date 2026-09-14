import React from "react";
import "./shell.css";
import SpotlightCard from "../Common/DataTable/SpotlightCard";

const StatCard = ({ label, value, icon: Icon, sub, accent = false, standalone = false, onClick }) => {
    return (
        <SpotlightCard
            className={[
                "stat-card",
                accent ? "stat-card-accent" : "",
                standalone ? "stat-card-standalone" : "",
            ]
                .filter(Boolean)
                .join(" ")}
            onClick={onClick}
        >
            <div className="stat-card-label text-body-regular" style={{ display: "flex", alignItems: "center", gap: 8 }}>
                {Icon && <Icon size={16} />}
                {label}
            </div>
            <div className="text-kpi-value">{value}</div>
            {sub && (
                <div className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    {sub}
                </div>
            )}
        </SpotlightCard>
    );
};

export default StatCard;
