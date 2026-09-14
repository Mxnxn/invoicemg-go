import React from "react";
import "./loader.css";

// .jsx, not .js: vitest transforms JSX by EXTENSION and ignores the esbuild loader
// the app build uses, so a .js file holding JSX fails to parse under test while
// working perfectly in the browser. Same trap AnalyticsIndex documents.

// The inline sibling of Loader - same shape, sized for a spot inside a form or a
// row rather than a whole page. Replaces react-spinners/ScaleLoader.
//
// Default export, no props, so existing `<LoaderComponent />` call sites are
// unchanged.
const LoaderComponent = ({ label = "Loading" }) => (
    <span className="blob-loader blob-loader--inline" role="status" aria-live="polite">
        <span className="blob-loader-shape" />
        <span className="blob-loader-label">{label}…</span>
    </span>
);

export default LoaderComponent;
