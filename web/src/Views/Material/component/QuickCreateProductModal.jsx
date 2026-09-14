import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Row, Col, Input } from "reactstrap";
import { materialsBackend } from "../material_backend";
import { unitBackend } from "../../../Common/unit_backend";
import useRequiredFields from "../../../Common/useRequiredFields";
import DataTable from "../../../Common/DataTable/DataTable";
import { getDate } from "../../../Common/DateAndTime/getDate";

const initialForm = { material_name: "", material_rate: "", purchase_rate: "", hsn: "", tax: "18", unit: "" };

// The one product dialog: "add a product without leaving what you're doing" from the material
// picker's "+ Create new" row in CreateJobModal, AND the create/edit form for
// Configure > Products, which has no inline form of its own any more.
//
// Serving both is what keeps them honest. This dialog used to be create-only and carried no
// `unit` field at all, while Configure > Products had made unit required - so a product
// created from a job card was saved without one and then sat under "Not counted" on the
// inventory report forever. Two forms for one record is how that happens; there is now one.
//
// `product` null (or absent) means create; a product means edit.
const QuickCreateProductModal = ({ isOpen, toggle, initialName, product, onCreated, onUpdated }) => {
	const [form, setForm] = useState(initialForm);
	const [saving, setSaving] = useState(false);
	const [units, setUnits] = useState([]);

	const isEditing = !!product;

	const required = useRequiredFields(form, {
		material_name: "Product name",
		material_rate: "Sale rate",
		purchase_rate: "Purchase rate",
		hsn: "HSN code",
		// Required here rather than in the schema: every product that already exists has no
		// unit, and a required field on the model would make each of them unsaveable. The form
		// is where the gap gets closed - and until it is, the inventory report lists the
		// product under "Not counted" rather than guessing a unit for it.
		unit: "Unit",
	});

	// The same Model/Unit collection the Purchase Invoice rows use and UnitManager edits. The
	// API seeds the defaults the inventory rules understand on first read, so this is never
	// empty on a fresh company.
	useEffect(() => {
		if (!isOpen) return;
		unitBackend
			.list()
			.then((res) => setUnits(res.data || []))
			// The interceptor toasts the failure; an empty list leaves the select empty, which
			// the required-field guard already refuses to submit.
			.catch(() => {});
	}, [isOpen]);

	useEffect(() => {
		if (!isOpen) return;
		if (product) {
			setForm({
				material_name: product.material_name || "",
				material_rate: product.material_rate ?? "",
				purchase_rate: product.purchase_rate ?? "",
				hsn: product.hsn || "",
				tax: String(product.tax ?? 0),
				unit: product.unit || "",
			});
		} else {
			setForm({ ...initialForm, material_name: initialName || "" });
		}
	}, [isOpen, initialName, product]);

	const onChange = (e) => setForm({ ...form, [e.target.name]: e.target.value });

	const onSubmit = async () => {
		if (!required.isComplete) {
			required.showAll();
			return;
		}
		setSaving(true);
		try {
			const formData = new FormData();
			formData.set("material_name", form.material_name.trim());
			formData.set("material_rate", form.material_rate);
			formData.set("purchase_rate", form.purchase_rate);
			formData.set("hsn", form.hsn.trim());
			formData.set("tax", form.tax || 0);
			formData.set("unit", form.unit);

			const stoken = window.localStorage.getItem("session_token");
			if (isEditing) {
				formData.set("material_id", product.id || product._id);
				const res = await materialsBackend.editMaterial(formData, stoken);
				if (onUpdated) onUpdated(res.data);
			} else {
				const res = await materialsBackend.addNewMaterial(formData, stoken);
				// Hand the created product straight back so the caller can select it into the
				// row the user was already filling in.
				if (onCreated) onCreated(res.data);
			}
			toggle();
		} catch (error) {
			// errorInterceptor already raises the toast.
		} finally {
			setSaving(false);
		}
	};

	const field = (name, label, props = {}) => (
		<FormGroup>
			<label className="form-control-label pp fs-12" htmlFor={`product-${name}`}>
				{label}
				{required.errors[name] !== undefined && <span className="required-star">*</span>}
			</label>
			<Input
				id={`product-${name}`}
				className={`form-control-alternative nn${required.errorFor(name) ? " is-required-missing" : ""}`}
				name={name}
				value={form[name]}
				onChange={onChange}
				onBlur={() => required.markTouched(name)}
				{...props}
			/>
			{required.errorFor(name) && <span className="field-error">{required.errorFor(name)}</span>}
		</FormGroup>
	);

	const history = isEditing ? product.priceHistory || [] : [];

	return (
		<Modal isOpen={isOpen} toggle={toggle} style={{ zIndex: 10000000001 }} centered>
			<ModalHeader toggle={toggle}>
				<span className="confirm-modal-title">{isEditing ? "Edit product" : "New product"}</span>
			</ModalHeader>
			<ModalBody>
				<Row>
					<Col md="12">{field("material_name", "Product name", { placeholder: "e.g. Vinyl Print" })}</Col>
				</Row>
				<Row>
					<Col md="6">{field("material_rate", "Sale rate", { type: "number", placeholder: "0" })}</Col>
					<Col md="6">{field("purchase_rate", "Purchase rate", { type: "number", placeholder: "0" })}</Col>
				</Row>
				<Row>
					<Col md="6">{field("hsn", "HSN", { placeholder: "e.g. 4911" })}</Col>
					<Col md="6">
						<FormGroup>
							<label className="form-control-label pp fs-12" htmlFor="product-tax">
								GST %
							</label>
							<Input
								id="product-tax"
								className="form-control-alternative nn"
								type="select"
								name="tax"
								value={form.tax}
								onChange={onChange}
							>
								{[0, 5, 12, 18, 28].map((t) => (
									<option key={t} value={t}>
										{t}%
									</option>
								))}
							</Input>
						</FormGroup>
					</Col>
				</Row>
				<Row>
					<Col md="12">
						{field("unit", "Unit", {
							type: "select",
							children: (
								<>
									<option value="">Select a unit</option>
									{units.map((u) => (
										<option key={u._id} value={u.name}>
											{u.name}
										</option>
									))}
								</>
							),
						})}
						<span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
							How this product is counted in stock. Job dimensions are read in this unit.
						</span>
					</Col>
				</Row>

				{/* Only when editing, because a product being created has no history yet. */}
				{history.length > 0 && (
					<>
						<div className="text-body-small" style={{ color: "var(--text-tertiary)", marginTop: 14, marginBottom: 6 }}>
							Price history
						</div>
						<DataTable>
							<thead>
								<tr>
									<th scope="col">Date</th>
									<th scope="col">Rate</th>
									<th scope="col">Purchase rate</th>
								</tr>
							</thead>
							<tbody>
								{[...history]
									.sort((a, b) => new Date(b.changed_at) - new Date(a.changed_at))
									.map((el, idx) => (
										<tr key={idx}>
											<td className="cell-mono">{getDate(el.changed_at)}</td>
											<td className="cell-mono">{el.material_rate}</td>
											<td className="cell-mono">{el.purchase_rate}</td>
										</tr>
									))}
							</tbody>
						</DataTable>
					</>
				)}
			</ModalBody>
			<ModalFooter>
				<button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={saving}>
					Cancel
				</button>
				<button
					type="button"
					className="shell-btn shell-btn-primary"
					onClick={onSubmit}
					disabled={saving || !required.isComplete}
				>
					{saving ? "Saving…" : isEditing ? "Save changes" : "Add product"}
				</button>
			</ModalFooter>
		</Modal>
	);
};

export default QuickCreateProductModal;
