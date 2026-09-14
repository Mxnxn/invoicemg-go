import React from "react";

// The Add Row trigger's icon: a plus that rotates into a cross when the menu is open, and
// back when it closes.
//
// Three strokes rather than two, and one geometry, so the rotation is the only thing that
// moves and the three cannot drift apart:
//
//   shaft   a vertical bar through the middle
//   left    a half bar reaching left from the centre
//   right   a half bar reaching right from the centre
//
// Flat, they read as a plus. Rotated 45 degrees, as a cross.
//
// There is no component state and no timer here on purpose - the whole transition is one CSS
// rule on the root svg, which means it cannot get out of step with itself, and reduced motion
// is handled by a media query rather than by branching in JavaScript.
const AddRowIcon = ({ open, size = 13 }) => (
    <svg
        className={`addrow-icon ${open ? "is-x" : "is-plus"}`}
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        aria-hidden="true"
        focusable="false"
    >
        <line className="addrow-icon-shaft" x1="12" y1="5" x2="12" y2="19" />
        <line className="addrow-icon-left" x1="5" y1="12" x2="12" y2="12" />
        <line className="addrow-icon-right" x1="12" y1="12" x2="19" y2="12" />
    </svg>
);

export default AddRowIcon;
