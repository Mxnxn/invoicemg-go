import React from "react";
import { PDFViewer } from "@react-pdf/renderer";
import { getQuotationTemplate } from "./registry";
import "../Invoice/App.css";

// Same shape as Invoice/Template/Main.js - PDFViewer and the Document it renders must be
// lazy-loaded together as one unit (see QuotationPreviewModal.jsx), not imported at the
// top of a file that's part of the eager bundle. `user` already carries
// `quotationTemplate` (see routes/UserInfo.js's buildProfile) since every caller sources
// it from the same /userinfo/get fetch.
const QuotationMain = ({ quotation, user }) => {
    const Template = getQuotationTemplate(user?.quotationTemplate);
    return (
        <PDFViewer className="app">
            <Template quotation={quotation} user={user} fontKey={user?.documentFont} scaleId={user?.documentScale} />
        </PDFViewer>
    );
};

export default QuotationMain;
