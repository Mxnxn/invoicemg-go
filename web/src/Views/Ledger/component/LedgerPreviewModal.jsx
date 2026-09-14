import React, { Suspense, lazy } from "react";
import { Modal, ModalHeader, ModalBody } from "reactstrap";

// @react-pdf/renderer's dependency chain includes a top-level require() that isn't valid
// in a browser ESM bundle - lazy-load the whole PDFViewer+Document unit (same as
// InvoicePreviewModal.js/QuotationPreviewModal.jsx) so it only executes once opened.
const LedgerMain = lazy(() => import("./LedgerMain"));

const LedgerPreviewModal = ({ isOpen, toggle, ledger, user, from, to }) => (
    <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ zIndex: 10000000000, maxWidth: "95vw", width: "95vw" }}>
        <ModalHeader toggle={toggle}>Ledger Preview</ModalHeader>
        <ModalBody style={{ height: "90vh", padding: 0 }}>
            {isOpen && ledger && (
                <Suspense fallback={null}>
                    <LedgerMain ledger={ledger} user={user} from={from} to={to} />
                </Suspense>
            )}
        </ModalBody>
    </Modal>
);

export default LedgerPreviewModal;
