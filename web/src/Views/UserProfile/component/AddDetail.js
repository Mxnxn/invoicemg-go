import React from "react";
import {
	Modal,
	ModalHeader,
	ModalBody,
	Row,
	Col,
	FormGroup,
	Input,
	ModalFooter,
	Button,
} from "reactstrap";
const AddDetail = ({
	update,
	form,
	setForm,
	modalNew,
	status,
	onSubmitHandler,
	toggle,
	onCancelHandler,
}) => {
	return (
		<Modal size="lg" isOpen={modalNew} toggle={toggle}>
			<ModalHeader className="bg " toggle={toggle}>
				<h3 className="text-default">Update Entry</h3>
			</ModalHeader>
			<ModalBody className="bg">
				<Col className="order-xl-1" xl="12">
					{update ? (
						<div className="alert alert-info">
							Note: These will override your details and may affect your
							Invoice.
						</div>
					) : (
						<></>
					)}
					{status ? (
						<div className="alert alert-success">Successfully Saved.</div>
					) : (
						<></>
					)}
					<Row>
						<Col lg="6">
							<FormGroup>
								<label
									className="form-control-label nn-bb"
									htmlFor="input-username"
								>
									Name
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-username"
									placeholder="Name"
									type="text"
									value={form.name}
									onChange={(e) => setForm({ ...form, name: e.target.value })}
								/>
							</FormGroup>
						</Col>
						<Col lg="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-email">
									Email address
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-email"
									placeholder="xyz@mail.com"
									type="email"
									value={form.email}
									onChange={(e) => setForm({ ...form, email: e.target.value })}
								/>
							</FormGroup>
						</Col>
					</Row>
					<Row>
						<Col lg="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-firm-name">
									Firm Name
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-firm-name"
									placeholder="e.g Facebook Inc."
									type="text"
									value={form.firm}
									onChange={(e) => setForm({ ...form, firm: e.target.value })}
								/>
							</FormGroup>
						</Col>
						<Col lg="6">
							<FormGroup>
								<label
									className="form-control-label"
									htmlFor="input-phone-number"
								>
									Contact Number
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-last-number"
									placeholder="+12 1234567890"
									type="text"
									value={form.phone}
									onChange={(e) => setForm({ ...form, phone: e.target.value })}
								/>
							</FormGroup>
						</Col>
					</Row>
					<hr className="my-4" />
					<h6 className="heading-small text-muted mb-4">Contact information</h6>
					<Row>
						<Col md="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-address">
									Address
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-address"
									placeholder="Address"
									type="text"
									value={form.address}
									onChange={(e) =>
										setForm({ ...form, address: e.target.value })
									}
								/>
							</FormGroup>
						</Col>
						<Col md="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-gst">
									GST Number
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-gst"
									placeholder="24EYJPSBLABLA2Z2"
									type="text"
									value={form.gst}
									onChange={(e) => setForm({ ...form, gst: e.target.value })}
								/>
							</FormGroup>
						</Col>
					</Row>
					<Row>
						<Col md="12">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-address">
									Bank Name
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-address"
									placeholder="Kotak Mahendra"
									type="text"
									value={form.bank_name}
									onChange={(e) =>
										setForm({ ...form, bank_name: e.target.value })
									}
								/>
							</FormGroup>
						</Col>
					</Row>
					<Row>
						<Col md="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-address">
									IFSC
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-address"
									placeholder="KKBKXXXXXXXx"
									type="text"
									value={form.ifsc}
									onChange={(e) => setForm({ ...form, ifsc: e.target.value })}
								/>
							</FormGroup>
						</Col>
						<Col md="6">
							<FormGroup>
								<label className="form-control-label" htmlFor="input-gst">
									Account Number
								</label>
								<Input
									className="nn form-control-alternative"
									id="input-gst"
									placeholder="Account number"
									type="text"
									value={form.account_no}
									onChange={(e) =>
										setForm({ ...form, account_no: e.target.value })
									}
								/>
							</FormGroup>
						</Col>
					</Row>
				</Col>
			</ModalBody>
			<ModalFooter className="bg">
				<Button
					color="primary"
					className="btn btn-danger"
					onClick={(e) => onCancelHandler()}
				>
					Cancel
				</Button>
				<Button
					color="primary"
					className="btn btn-success"
					onClick={onSubmitHandler}
				>
					Done
				</Button>
			</ModalFooter>
		</Modal>
	);
};

export default AddDetail;
