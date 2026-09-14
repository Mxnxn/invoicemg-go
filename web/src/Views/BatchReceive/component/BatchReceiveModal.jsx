import React from "react";
import { Modal, ModalHeader, ModalBody } from "reactstrap";
import BatchReceiveFormCard from "./BatchReceiveFormCard";

// Same form as the global More > Batch Receive hub, just wrapped in a modal with the client
// already fixed - used from the Client page instead of the old simple amount/note/date form.
const BatchReceiveModal = ({ isOpen, toggle, fixedClient, onCreated }) => (
    <Modal isOpen={isOpen} toggle={toggle} size="lg">
        <ModalHeader toggle={toggle}>New Batch Receive</ModalHeader>
        <ModalBody>
            <BatchReceiveFormCard
                showHeader={false}
                fixedClient={fixedClient}
                onCreated={(receive) => {
                    onCreated(receive);
                    toggle();
                }}
            />
        </ModalBody>
    </Modal>
);

export default BatchReceiveModal;
