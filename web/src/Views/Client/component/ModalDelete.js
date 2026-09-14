import React from "react";
import {
	ModalBody,
	Col,
	Modal,
	ModalHeader,
	Row,
	ModalFooter,
	Button,
} from "reactstrap";

const ModalDelete = ({ onCancelHandler, onSubmitHandler, modal }) => {
	return (
		<Modal
			isOpen={modal}
			toggle={onCancelHandler}
			style={{ zIndex: 10000000000 }}
		>
			<ModalHeader className="bg " toggle={onCancelHandler}>
				{/* Shell typography, not the legacy Argon utilities. `.geb` pins font-family
				    to GEB and `.fs-24` pins 24px, so this ignored BOTH Account settings >
				    Appearance > Font and > Font size - the text-* classes scale with each. */}
				<h3 className="text-heading-page">Delete Product</h3>
			</ModalHeader>
			<ModalBody className="bg">
				<Row>
					<Col>
						<h4 className="text-heading-brand">Are you sure?</h4>
					</Col>
				</Row>
			</ModalBody>
			<ModalFooter className="bg">
				<Button
					color="primary"
					className="btn fira btn-success"
					onClick={onSubmitHandler}
				>
					Delete
				</Button>{" "}
				<Button
					color="secondary"
					className="btn fira btn-warning"
					onClick={onCancelHandler}
				>
					Cancel
				</Button>
			</ModalFooter>
		</Modal>
	);
};

export default ModalDelete;
