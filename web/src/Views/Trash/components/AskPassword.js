import PasswordInput from "../../../Common/PasswordInput";
import React from "react";
import {
	ModalBody,
	Col,
	Modal,
	ModalHeader,
	Row,
	FormGroup,
	Input,
	ModalFooter,
	Button,
} from "reactstrap";

const AskPassword = ({
	onCancelHandler,
	modal,
	password,
	Done,
	error,
	setPassword,
}) => {
	return (
		<Modal
			isOpen={modal}
			toggle={onCancelHandler}
			style={{ zIndex: 10000000000 }}
		>
			<ModalHeader className="bg " toggle={onCancelHandler}>
				<span className="text fs-24 geb">Vault</span>
			</ModalHeader>
			<ModalBody className="bg">
				{error ? (
					<Row>
						<Col xl="12" sm="12">
							<div className="alert-danger alert">{error}</div>
						</Col>
					</Row>
				) : (
					""
				)}

				<Row>
					<Col xl="12">
						<FormGroup>
							<label className="form-control-label pp fs-12 ">Password</label>
							<PasswordInput
								inputClassName={
									error
										? "form-control-alternative nn is-danger"
										: "form-control-alternative nn"
								}
								placeholder="Enter"
								name="cPassword"
								value={password}
								onChange={(evt) =>
									setPassword({ ...password, value: evt.target.value })
								}
							/>
						</FormGroup>
					</Col>
				</Row>
			</ModalBody>
			<ModalFooter className="bg">
				<Button color="primary" className="btn fira btn-success" onClick={Done}>
					Open
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

export default AskPassword;
