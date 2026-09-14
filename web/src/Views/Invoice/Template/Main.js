import React, { useEffect, useState } from "react";
import { PDFViewer } from "@react-pdf/renderer";
import "../App.css";
import { getInvoiceTemplate } from "./registry";
import { userBackend } from "../../UserProfile/user_backend";
import InvoiceActions from "../Component/InvoiceActions";

const Main = ({ invoice }) => {
    // Resolved (not just defaulted) before PDFViewer ever mounts - swapping which Document
    // tree a live PDFViewer renders mid-lifecycle crashes react-pdf's internal layout engine
    // on unmount (cleanup runs against a node the new tree never set up), so the fetch has
    // to gate the first render instead of following it up with a second one.
    const [templateKey, setTemplateKey] = useState(null);
    // Company-wide, so it rides along with the template on the same profile fetch.
    const [fontKey, setFontKey] = useState(null);
    // Company-wide too, and fetched on the same profile call for the same reason.
    const [scaleId, setScaleId] = useState(null);

    // Matches the shell's phone breakpoint. Tracked live so a rotation or a resized window
    // swaps between the viewer and the actions instead of being decided once at mount.
    const [isPhone, setIsPhone] = useState(() => window.matchMedia("(max-width: 767px)").matches);
    useEffect(() => {
        const mq = window.matchMedia("(max-width: 767px)");
        const onChange = (e) => setIsPhone(e.matches);
        mq.addEventListener("change", onChange);
        return () => mq.removeEventListener("change", onChange);
    }, []);

    useEffect(() => {
        const formData = new FormData();
        formData.set("uid", window.localStorage.getItem("uid"));
        userBackend
            .getUserInfo(formData, window.localStorage.getItem("session_token"))
            .then((res) => {
                setTemplateKey(res.data.invoiceTemplate || "classic");
                setFontKey(res.data.documentFont || "open-sans");
                setScaleId(res.data.documentScale || "normal");
            })
            .catch(() => {
                setTemplateKey("classic");
                setFontKey("open-sans");
                setScaleId("normal");
            });
    }, []);

    if (!templateKey) return null;

    // Mobile browsers do not render a PDF inside an iframe, which is all PDFViewer is - the
    // frame showed a dead "Open" button instead of the invoice. Below the shell's phone
    // breakpoint, offer the actions that do work there rather than a preview that cannot.
    if (isPhone) {
        return (
            <div style={{ padding: 20 }}>
                <InvoiceActions invoice={invoice} templateKey={templateKey} fontKey={fontKey} scaleId={scaleId} compact />
            </div>
        );
    }

    const Template = getInvoiceTemplate(templateKey);
    return (
        <>
            <div style={{ padding: "10px 12px", borderBottom: "1px solid var(--border-default)" }}>
                <InvoiceActions invoice={invoice} templateKey={templateKey} fontKey={fontKey} scaleId={scaleId} />
            </div>
            <PDFViewer className="app">
                <Template invoice={invoice} fontKey={fontKey} scaleId={scaleId} />
            </PDFViewer>
        </>
    );
};

export default Main;
