import React, { useEffect, useState } from "react";
import { ModalBody, Col, Modal, ModalHeader, Row, FormGroup, Input, ModalFooter, Button } from "reactstrap";
import { getDateForEntry } from "../../../Common/DateAndTime/getDate";
import { invoiceBackend } from "../invoice_backend";
import DateField from "../../../Common/DateField";

const GenrateInvoiceNumber = ({ onCancelHandler, onGenerateInvoice, modal }) => {
      const [state, setState] = useState({
            date: getDateForEntry(new Date()),
            invoiceNumber: "",
      });

      useEffect(() => {
            if (modal) {
                  setState((s) => ({ ...s, invoiceNumber: "" }));
                  invoiceBackend
                        .nextInvoiceNumber()
                        .then((res) => setState((s) => ({ ...s, invoiceNumber: res.data.invoiceNumber })));
            }
      }, [modal]);

      return (
            <Modal isOpen={modal} toggle={onCancelHandler} style={{ zIndex: 10000000000 }}>
                  <ModalHeader className="bg " toggle={onCancelHandler}>
                        <span className="text fs-24 geb">Select Month</span>
                  </ModalHeader>
                  <ModalBody className="bg">
                        <Row>
                              <Col xl="12">
                                    <FormGroup>
                                          <label className="form-control-label" htmlFor="input-name">
                                                Invoice Number
                                          </label>
                                          {/* Read-only: the number comes from the server's running
                                              sequence (/invoice/next-number). Letting it be typed over
                                              invited duplicate or out-of-order invoice numbers, which
                                              is the one field on a tax invoice that has to be unique
                                              and sequential. */}
                                          <Input
                                                readOnly
                                                disabled
                                                className="form-control-alternative"
                                                placeholder="Generating…"
                                                type="text"
                                                value={state.invoiceNumber}
                                          />
                                          <small className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                                Assigned automatically.
                                          </small>
                                    </FormGroup>
                              </Col>
                              <Col xl="12">
                                    <FormGroup>
                                          <label className="form-control-label" htmlFor="input-name">
                                                Date
                                          </label>
                                          <DateField value={state.date} onChange={(evt) => {
                                                      setState({ ...state, date: evt.target.value });
                                                }} />
                                    </FormGroup>
                              </Col>
                        </Row>
                  </ModalBody>
                  <ModalFooter className="bg">
                        <Button
                              color="primary"
                              className="btn fira btn-success"
                              // Until the number arrives there's nothing to stamp on the invoice.
                              disabled={!state.invoiceNumber}
                              onClick={(evt) => {
                                    onGenerateInvoice(state.date, state.invoiceNumber);
                              }}
                        >
                              Add
                        </Button>{" "}
                        <Button color="secondary" className="btn fira btn-warning" onClick={onCancelHandler}>
                              Cancel
                        </Button>
                  </ModalFooter>
            </Modal>
      );
};

export default GenrateInvoiceNumber;
