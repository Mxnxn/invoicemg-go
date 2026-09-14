import React from "react";
import "./dataTable.css";

// actions: [{ label, icon: Component, onClick }]
const BulkActionsBar = ({ count, itemLabel = "Items", actions = [] }) => {
    if (!count) return null;

    return (
        <div className="xan-bulk-bar">
            <div className="xan-bulk-bar-count">
                <span className="badge">{count}</span>
                <span className="label">
                    {itemLabel} Selected
                </span>
            </div>
            <div className="xan-bulk-bar-actions">
                {actions.map(({ label, icon: Icon, onClick }) => (
                    <button key={label} type="button" onClick={onClick}>
                        {Icon && <Icon size={13} />}
                        {label}
                    </button>
                ))}
            </div>
        </div>
    );
};

export default BulkActionsBar;
