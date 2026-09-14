import React from "react";
import { ModalBody, Modal, ModalHeader, ModalFooter } from "reactstrap";
import { AlertCircle } from "react-feather";

// Confirmation before an invoice is written.
//
// Previously styled with legacy utility classes (`text`, `geb`, `fira`) and Bootstrap's
// btn-success / btn-warning, so the body text and buttons kept light-theme colours in dark
// mode and used a different typeface from the rest of the app. Everything here now comes
// from src/styles/tokens.css, which follows the viewer's theme.
const ConfirmModal = ({ onCancelHandler, onSubmitHandler, modal }) => {
	return (
		<Modal isOpen={modal} toggle={onCancelHandler} style={{ zIndex: 10000000000 }} centered>
			<ModalHeader className="bg" toggle={onCancelHandler}>
				<span className="confirm-modal-title">Save this invoice</span>
			</ModalHeader>
			<ModalBody className="bg">
				<div className="confirm-modal-body">
					<span className="confirm-modal-icon">
						<AlertCircle size={18} />
					</span>
					<div>
						<p className="confirm-modal-lead">Are you sure?</p>
						<p className="confirm-modal-sub">
							The invoice number is assigned when you save, so this cannot be undone by editing.
						</p>
					</div>
				</div>
			</ModalBody>
			<ModalFooter className="bg">
				<button type="button" className="shell-btn shell-btn-secondary" onClick={onCancelHandler}>
					Cancel
				</button>
				<button type="button" className="shell-btn shell-btn-primary" onClick={onSubmitHandler}>
					Save invoice
				</button>
			</ModalFooter>
		</Modal>
	);
};

export default ConfirmModal;
