import React, { Suspense, lazy } from "react";
import { Modal, ModalHeader, ModalBody } from "reactstrap";

// @react-pdf/renderer's dependency chain includes a top-level require() that isn't valid
// in a browser ESM bundle - lazy-load the whole PDFViewer+Document unit (same as
// InvoicePreviewModal.js's Main) so it only executes once a preview is actually opened.
const QuotationMain = lazy(() => import("./QuotationMain"));

const QuotationPreviewModal = ({ isOpen, toggle, quotation, user }) => (
    <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ zIndex: 10000000000, maxWidth: "95vw", width: "95vw" }}>
        <ModalHeader toggle={toggle}>Quotation Preview</ModalHeader>
        <ModalBody style={{ height: "90vh", padding: 0 }}>
            {isOpen && quotation && (
                <Suspense fallback={null}>
                    <QuotationMain quotation={quotation} user={user} />
                </Suspense>
            )}
        </ModalBody>
    </Modal>
);

export default QuotationPreviewModal;
