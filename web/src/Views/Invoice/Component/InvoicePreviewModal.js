import React, { Suspense, lazy } from "react";
import { Modal, ModalHeader, ModalBody, Button } from "reactstrap";

// @react-pdf/renderer's dependency chain includes a top-level `require()` that isn't
// valid in a browser ESM bundle - lazy-loading keeps it out of the main entry chunk so
// it only executes (and only risks breaking) when an invoice preview is actually opened.
const Main = lazy(() => import("../Template/Main"));

// onSave is only passed by callers driving the generate-then-save flow (Client page's
// Jobs tab) - pure preview callers (past-invoice viewers, the Invoices list, the demo
// preview) leave it out and just get the plain preview.
const InvoicePreviewModal = ({ isOpen, toggle, invoice, onSave }) => (
      <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ zIndex: 10000000000, maxWidth: "95vw", width: "95vw" }}>
            <ModalHeader toggle={toggle}>
                  Invoice Preview
                  {onSave && (
                        <Button
                              color="primary"
                              className="shell-btn shell-btn-primary"
                              style={{ marginLeft: 16 }}
                              onClick={onSave}
                        >
                              Save This Invoice
                        </Button>
                  )}
            </ModalHeader>
            <ModalBody style={{ height: "90vh", padding: 0 }}>
                  {isOpen && invoice && (
                        <Suspense fallback={null}>
                              <Main invoice={invoice} />
                        </Suspense>
                  )}
            </ModalBody>
      </Modal>
);

export default InvoicePreviewModal;
