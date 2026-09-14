import React from "react";
import "./loader.css";

// .jsx, not .js: vitest transforms JSX by EXTENSION and ignores the esbuild loader
// the app build uses, so a .js file holding JSX fails to parse under test while
// working perfectly in the browser. Same trap AnalyticsIndex documents.

// The full-page loading indicator.
//
// Replaces react-spinners/MoonLoader, which brought @emotion/core with it and
// painted a hardcoded #4F4F4F - a mid grey that sat wrong on both themes and
// followed neither. This is CSS only and takes its colour from a token.
//
// See loader.css for why the shape does not visibly repeat.
//
// Still a default export taking no props, so every existing `<Loader />` keeps
// working untouched.
const Loader = ({ label = "Loading" }) => (
    <div className="blob-loader blob-loader--page" role="status" aria-live="polite">
        <span className="blob-loader-shape" />
        {/* Named for a screen reader, which has nothing else to go on: the shape
            carries no text and aria-busy alone announces nothing. */}
        <span className="blob-loader-label">{label}…</span>
    </div>
);

export default Loader;
