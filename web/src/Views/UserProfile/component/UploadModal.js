import React from "react";
import { Modal, ModalHeader, ModalBody, Row, Col, ModalFooter, Button } from "reactstrap";
const UploadModal = ({ modal, error, toggle, uploadImage, inputRef, userPhoto, progressVar, handlePhoto, Progress }) => {
      return (
            <Modal isOpen={modal} toggle={toggle}>
                  <ModalHeader className="bg " toggle={toggle}>
                        <h3 className="text-default">Update Entry</h3>
                  </ModalHeader>
                  <ModalBody className="bg">
                        <Row>
                              <Col xl="12">
                                    {error && (
                                          <div className="alert alert-danger">
                                                Please select correct file with jpg,jpeg or png format.
                                          </div>
                                    )}

                                    {userPhoto ? (
                                          <p tag="h6 mt-5">
                                                Filename :<span className="geb fs-18 ml-1">{userPhoto.name}</span>{" "}
                                          </p>
                                    ) : (
                                          <></>
                                    )}

                                    <Button color="warning" onClick={(e) => inputRef.current.click()}>
                                          Choose picture
                                    </Button>

                                    {progressVar > 0 ? <Progress value={progressVar} /> : <></>}
                                    <input type="file" ref={inputRef} onChange={handlePhoto} style={{ display: "none" }} />
                              </Col>
                              <Col xl="6"></Col>
                        </Row>
                  </ModalBody>
                  <ModalFooter className="bg">
                        <Button color="primary" className="btn btn-success" onClick={uploadImage}>
                              Add
                        </Button>
                  </ModalFooter>
            </Modal>
      );
};

export default UploadModal;
