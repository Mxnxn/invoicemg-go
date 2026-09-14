import React from "react";
import "./dataTable.css";

// The variant decides the colour pair, and every value here has a rule in dataTable.css -
// anything else renders as an uncoloured pill, which is how the Invoiced column sat unstyled.
//
// Semantic: "paid" | "pending" | "overdue" | "neutral" | "open"
// Colour-named: "green" | "lime" | "amber" | "red" - for columns whose meaning is the colour
// itself (invoiced / expired / inactive) rather than one of the payment states above.
const StatusBadge = ({ status = "neutral", children, ...rest }) => (
    <span className={`xan-status-badge status-${status}`} {...rest}>
        <span className="xan-pulse-dot" />
        {children}
    </span>
);

export default StatusBadge;
