import React from "react";
import "./dataTable.css";

const handleMouseMove = (e) => {
    const rect = e.currentTarget.getBoundingClientRect();
    e.currentTarget.style.setProperty("--mouse-x", `${e.clientX - rect.left}px`);
    e.currentTarget.style.setProperty("--mouse-y", `${e.clientY - rect.top}px`);
};

const SpotlightCard = ({ children, className = "", style, onClick }) => (
    <div
        className={["xan-spotlight", className].filter(Boolean).join(" ")}
        style={onClick ? { cursor: "pointer", ...style } : style}
        onMouseMove={handleMouseMove}
        onClick={onClick}
    >
        {children}
    </div>
);

export default SpotlightCard;
