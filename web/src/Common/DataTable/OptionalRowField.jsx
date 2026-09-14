import React, { useState } from "react";
import { Plus, X } from "react-feather";

const fieldStyle = {
    width: 58,
    padding: "4px 6px",
    fontSize: "0.675rem",
    height: "auto",
    border: "1px solid var(--border-default, #d9d9df)",
    borderRadius: "var(--radius-chip, 4px)",
    background: "var(--bg-field-on-canvas, transparent)",
    color: "var(--text-primary, inherit)",
    fontFamily: "inherit",
};

// Compact per-row optional field (IGST% / Discount / Charges / Unit) - stays a narrow
// "+ Label" chip until added, so rows that don't need it don't burn table width; once added
// it's a small inline control with its own clear button that resets it and collapses back to
// the chip. `type="select"` (with `options`, an array of strings) swaps the numeric input for
// a dropdown - used by the Purchase Invoice row's Unit field.
const OptionalRowField = ({ label, value, onChange, disabled, type = "number", options }) => {
    const emptyValue = type === "select" ? "" : 0;
    const hasValue = type === "select" ? !!value : Number(value) > 0;
    const [active, setActive] = useState(hasValue);

    if (!active) {
        return (
            <button
                type="button"
                onClick={() => setActive(true)}
                disabled={disabled}
                style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 3,
                    border: "1px dashed var(--border-default)",
                    background: "transparent",
                    borderRadius: "var(--radius-chip)",
                    padding: "3px 7px",
                    fontSize: 11.5,
                    fontFamily: "inherit",
                    color: "var(--text-tertiary)",
                    cursor: disabled ? "default" : "pointer",
                    whiteSpace: "nowrap",
                }}
            >
                <Plus size={10} />
                {label}
            </button>
        );
    }

    return (
        <div style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
            {type === "select" ? (
                <select
                    autoFocus={!hasValue}
                    value={value || ""}
                    disabled={disabled}
                    onChange={(e) => onChange(e.target.value)}
                    title={label}
                    style={{ ...fieldStyle, width: 76 }}
                >
                    <option value="">{label}</option>
                    {(options || []).map((opt) => (
                        <option key={opt} value={opt}>
                            {opt}
                        </option>
                    ))}
                </select>
            ) : (
                <input
                    type="number"
                    step="1"
                    autoFocus={!hasValue}
                    value={value}
                    disabled={disabled}
                    onChange={(e) => onChange(e.target.value)}
                    placeholder={label}
                    title={label}
                    style={fieldStyle}
                />
            )}
            {!disabled && (
                <button
                    type="button"
                    aria-label={`Remove ${label}`}
                    onClick={() => {
                        onChange(emptyValue);
                        setActive(false);
                    }}
                    className="xan-clear-btn"
                >
                    <X size={13} strokeWidth={2.5} />
                </button>
            )}
        </div>
    );
};

export default OptionalRowField;
