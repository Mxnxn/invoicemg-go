import React from "react";
import { PDFViewer } from "@react-pdf/renderer";

// @react-pdf/renderer's dependency chain includes a top-level require() that isn't valid
// in a browser ESM bundle - always import this file with React.lazy (see
// TemplatePreviewPanel.jsx) so it only executes once a preview is actually shown, same
// precaution as InvoicePreviewModal.js/QuotationPreviewModal.jsx.
const PDFPreview = ({ Component, componentProps }) => (
    <PDFViewer style={{ width: "100%", height: "100%", border: "none" }}>
        <Component {...componentProps} />
    </PDFViewer>
);

export default PDFPreview;
