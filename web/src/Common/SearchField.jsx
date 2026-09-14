import React from "react";
import { Search, X } from "react-feather";

// The one search box.
//
// The same eight lines of markup were copied into eleven views, which is eleven chances for
// the size, the icon or the clear button to drift - and they had: some carried an X, most
// did not, and none of them sized their text to anything in particular.
//
// `onChange` keeps the native (event) signature every caller already passes, so clearing
// hands back a synthetic { target: { value: "" } } rather than making every call site learn
// a second convention.
const SearchField = ({ value = "", onChange, placeholder, style, className, ...rest }) => {
    const active = String(value).length > 0;
    return (
        <div
            className={["shell-search-field", active ? "is-active" : "", className].filter(Boolean).join(" ")}
            style={style}
        >
            <Search size={15} />
            <input value={value} onChange={onChange} placeholder={placeholder} {...rest} />
            {active && (
                <button
                    type="button"
                    className="shell-search-clear"
                    aria-label="Clear search"
                    // A search you cannot get out of is a trap on a touch screen, where there
                    // is no Escape key to fall back on.
                    onClick={() => onChange?.({ target: { value: "" } })}
                >
                    <X size={14} />
                </button>
            )}
        </div>
    );
};

export default SearchField;
