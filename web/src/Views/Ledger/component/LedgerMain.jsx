import React from "react";
import { PDFViewer } from "@react-pdf/renderer";
import { getLedgerTemplate } from "../registry";
import "../../Invoice/App.css";

// Same shape as Invoice/Template/Main.js and Quotation/QuotationMain.jsx.
const LedgerMain = ({ ledger, user, from, to }) => {
    const Template = getLedgerTemplate(user?.ledgerTemplate);
    return (
        <PDFViewer className="app">
            <Template ledger={ledger} user={user} from={from} to={to} fontKey={user?.documentFont} scaleId={user?.documentScale} />
        </PDFViewer>
    );
};

export default LedgerMain;
