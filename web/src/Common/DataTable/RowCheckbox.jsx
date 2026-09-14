import React from "react";
import { Check } from "react-feather";
import "./dataTable.css";

const RowCheckbox = ({ checked, onChange, ariaLabel, indeterminate = false }) => (
    <span className="xan-checkbox">
        <input
            type="checkbox"
            checked={checked}
            onChange={onChange}
            aria-label={ariaLabel}
            ref={(el) => el && (el.indeterminate = indeterminate)}
        />
        <span className="xan-checkbox-box">{(checked || indeterminate) && <Check size={12} />}</span>
    </span>
);

export default RowCheckbox;
