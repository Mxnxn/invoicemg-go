import PasswordInput from "../../../Common/PasswordInput";
import React, { useState, useEffect } from "react";
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
import { trashBackend } from "../trash_backend";

const SetPassword = ({ onCancelHandler, modal, onPasswordSet }) => {
	const initError = {
		password: false,
		cPassword: false,
	};

	const initFields = {
		password: "",
		cPassword: "",
	};

	useEffect(() => {
		setPasswords({
			input: initFields,
			error: initError,
		});
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const [passwords, setPasswords] = useState({
		input: initFields,
		error: initError,
	});

	const onChange = (evt) => {
		setPasswords({
			...passwords,
			error: initError,
			input: { ...passwords.input, [evt.target.name]: evt.target.value },
		});
	};

	const setPassword = async () => {
		if (
			!passwords.input.password ||
			passwords.input.password.length < 8 ||
			!passwords.input.cPassword
		) {
			return setPasswords({
				...passwords,
				error: { password: "Invalid Input.", cPassword: "Invalid Input." },
			});
		}
		if (passwords.input.password !== passwords.input.cPassword) {
			return setPasswords({
				...passwords,
				error: { ...passwords.error, cPassword: "Not matching." },
			});
		}
		try {
			const formData = new FormData();
			formData.set("uid", window.localStorage.getItem("uid"));
			formData.set("password", passwords.input.password);
			await trashBackend.setPassword(formData);
			onPasswordSet();
		} catch (error) {
			setPasswords((prev) => ({
				...prev,
				error: { ...prev.error, password: error.message || "Couldn't save the password." },
			}));
		}
	};

	return (
		<Modal
			isOpen={modal}
			toggle={onCancelHandler}
			style={{ zIndex: 10000000000 }}
		>
			<ModalHeader className="bg " toggle={onCancelHandler}>
				<h3 className="text fs-24 geb">Vault</h3>
			</ModalHeader>
			<ModalBody className="bg">
				{passwords.error.password || passwords.error.cPassword ? (
					<Row>
						<Col xl="12" sm="12">
							<div className="alert-danger alert">
								{passwords.error.password
									? passwords.error.password
									: passwords.error.cPassword}
							</div>
						</Col>
					</Row>
				) : (
					""
				)}

				<Row>
					<Col xl="6">
						<FormGroup>
							<label className="form-control-label  pp fs-12">Password</label>
							<PasswordInput
								inputClassName={
									passwords.error.password
										? "form-control-alternative nn is-danger"
										: "form-control-alternative nn"
								}
								placeholder="Enter"
																name="password"
								value={passwords.input.password}
								onChange={onChange}
							/>
						</FormGroup>
					</Col>
					<Col xl="6">
						<FormGroup>
							<label className="form-control-label pp fs-12 ">
								Confirm Password
							</label>
							<PasswordInput
								inputClassName={
									passwords.error.cPassword
										? "form-control-alternative nn is-danger"
										: "form-control-alternative nn"
								}
								placeholder="Re-Enter"
																name="cPassword"
								value={passwords.input.cPassword}
								onChange={onChange}
							/>
						</FormGroup>
					</Col>
				</Row>
			</ModalBody>
			<ModalFooter className="bg">
				<Button
					color="primary"
					className="btn fira btn-success"
					onClick={setPassword}
				>
					Add
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

export default SetPassword;
