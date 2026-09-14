import React, { useState, useEffect, useCallback } from "react";
import { Button, CardBody, FormGroup, Form, Input, Row, Col } from "reactstrap";
import { wastageBackend } from "../wastage_backend";
import DataTable from "../../../Common/DataTable/DataTable";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import DateField from "../../../Common/DateField";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { useUndoDelete } from "../../../Common/undoDelete";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { Trash2 } from "react-feather";

const today = () => new Date().toISOString().slice(0, 10);

const emptyEntry = {
	date: today(),
	material_name: "",
	rate: 0,
	length: 0,
	height: 0,
	purchase_rate: 0,
	cost_total: 0,
	total: 0,
};

// Wastage: narrows down false numbers on the operations side of analytics by
// recording material actually wasted (rate x length x height), separate from
// billed entries.
const WastageForm = () => {
	const uid = window.localStorage.getItem("uid");
	const [materials, setMaterials] = useState([]);
	const [entries, setEntries] = useState([]);
	const [entry, setEntry] = useState(emptyEntry);
	const [deleteTarget, setDeleteTarget] = useState(null);
	const { scheduleDelete } = useUndoDelete();

	// Removed from the list straight away and committed after the undo window, the same as
	// every other delete in the app. Nothing references a wastage row, so there is no
	// allocation to reverse - putting it back is just putting the row back on screen.
	const confirmDelete = () => {
		if (!deleteTarget) return;
		const row = deleteTarget;
		const index = entries.findIndex((el) => el._id === row._id);
		setEntries((prev) => prev.filter((el) => el._id !== row._id));
		setDeleteTarget(null);
		scheduleDelete({
			label: "wastage record",
			commit: () => {
				const formData = new FormData();
				formData.set("wastage_id", row._id);
				return wastageBackend.removeWastage(formData);
			},
			undo: () => setEntries((prev) => [...prev.slice(0, index), row, ...prev.slice(index)]),
		});
	};

	const loadMaterials = useCallback(async () => {
		try {
			const formData = new FormData();
			formData.set("uid", uid);
			const res = await wastageBackend.getMaterials(formData);
			setMaterials(res.data);
		} catch (error) {
			console.log(error);
		}
	}, [uid]);

	const loadWastages = useCallback(async () => {
		try {
			const formData = new FormData();
			formData.set("uid", uid);
			const res = await wastageBackend.getWastages(formData);
			setEntries(res.data);
		} catch (error) {
			console.log(error);
		}
	}, [uid]);

	useEffect(() => {
		loadMaterials();
		loadWastages();
	}, [loadMaterials, loadWastages]);

	const recompute = (next) => ({
		...next,
		total: next.rate * next.length * next.height,
		cost_total: (Number(next.purchase_rate) || 0) * next.length * next.height,
	});

	const onChange = (e) => {
		const { name, value } = e.target;
		if (name === "material_name") {
			const match = materials.find((el) => el.material_name === value);
			setEntry(
				recompute({
					...entry,
					material_name: value,
					rate: match ? match.material_rate : 0,
					// Prefilled from the product so the cost basis is right by default; still editable.
					purchase_rate: match ? match.purchase_rate || 0 : 0,
				})
			);
			return;
		}
		if (name === "rate" || name === "length" || name === "height") {
			setEntry(recompute({ ...entry, [name]: Number(value) }));
			return;
		}
		setEntry({ ...entry, [name]: value });
	};

	const onSubmit = async (e) => {
		e.preventDefault();
		if (!entry.material_name || !entry.length || !entry.height) return;
		try {
			const formData = new FormData();
			formData.set("uid", uid);
			formData.set("material_name", entry.material_name);
			formData.set("rate", entry.rate);
			formData.set("purchase_rate", entry.purchase_rate);
			formData.set("cost_total", entry.cost_total);
			formData.set("length", entry.length);
			formData.set("height", entry.height);
			formData.set("total", entry.total);
			formData.set("date", entry.date);
			await wastageBackend.addWastage(formData);
			setEntry(emptyEntry);
			loadWastages();
		} catch (error) {
			console.log(error);
		}
	};

	return (
		<Row className="mt-4">
			<Col xl="4" className="mb-4 mb-xl-0">
				<div className="shell-card">
					<div className="shell-card-header">
						<span className="text-heading-brand">Wastage</span>
					</div>
					<CardBody>
						<h6 className="heading-small text-muted mb-2 geb">Wastage information</h6>
						<Form autoComplete="off" onSubmit={onSubmit}>
							<Row>
								<Col lg="6">
									<FormGroup>
										<label className="form-control-label pp fs-12">Date</label>
										<DateField name="date" value={entry.date} onChange={onChange} />
									</FormGroup>
								</Col>
								<Col lg="6">
									<FormGroup>
										<label className="form-control-label pp fs-12">Material</label>
										{/* Searchable, like every other product/party picker in the app -
										    a plain <select> meant scrolling a list that grows with
										    every material ever recorded. Clearing it (the x) puts the
										    form back to no material, which is what "None" did. */}
										<AssigneeDropdown
											value={entry.material_name}
											placeholder="Select a material"
											options={materials.map((el) => ({ id: el.material_name, name: el.material_name }))}
											fullWidth
											onSelect={(id) => onChange({ target: { name: "material_name", value: id || "" } })}
										/>
									</FormGroup>
								</Col>
							</Row>
							<Row>
								<Col lg="4">
									<FormGroup>
										<label className="form-control-label pp fs-12">Rate</label>
										<Input
											className="form-control-alternative nn"
											name="rate"
											placeholder="0"
											type="number"
											value={entry.rate}
											onChange={onChange}
										/>
									</FormGroup>
								</Col>
								<Col lg="4">
									<FormGroup>
										<label className="form-control-label pp fs-12">Purchase Rate</label>
										<Input
											className="form-control-alternative nn"
											name="purchase_rate"
											placeholder="0"
											type="number"
											value={entry.purchase_rate}
											onChange={onChange}
										/>
									</FormGroup>
								</Col>
								<Col lg="4">
									<FormGroup>
										<label className="form-control-label pp fs-12">Length</label>
										<Input
											className="form-control-alternative nn"
											name="length"
											placeholder="0"
											type="number"
											value={entry.length}
											onChange={onChange}
										/>
									</FormGroup>
								</Col>
								<Col lg="4">
									<FormGroup>
										<label className="form-control-label pp fs-12">Height</label>
										<Input
											className="form-control-alternative nn"
											name="height"
											placeholder="0"
											type="number"
											value={entry.height}
											onChange={onChange}
										/>
									</FormGroup>
								</Col>
							</Row>
							<Row>
								<Col lg="6">
									<FormGroup>
										<label className="form-control-label pp fs-12">Total (selling value)</label>
										<Input className="form-control-alternative nn" type="number" value={entry.total} disabled />
									</FormGroup>
								</Col>
								<Col lg="6">
									<FormGroup>
										{/* What the wasted material actually cost - this is the figure the
											Analytics wastage metric deducts from profit. */}
										<label className="form-control-label pp fs-12">Cost of Wastage</label>
										<Input className="form-control-alternative nn" type="number" value={entry.cost_total} disabled />
									</FormGroup>
								</Col>
							</Row>
							<div className="text-left">
								<Button type="submit" className="shell-btn shell-btn-primary my-2">
									Add
								</Button>
							</div>
						</Form>
					</CardBody>
					<div className="shell-card-footer">
						<span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
							Total = Rate x Length x Height. Materials are deduped case-insensitively across
							clients, using the average rate.
						</span>
					</div>
				</div>
			</Col>

			<Col xl="8">
				<div className="shell-card">
					<div className="shell-card-header">
						<span className="text-heading-brand">Wastage log</span>
					</div>
					<DataTable>
						<thead>
							<tr>
								<th scope="col">#</th>
								<th scope="col">Date</th>
								<th scope="col">Material</th>
								<th scope="col">Rate</th>
								<th scope="col">Length</th>
								<th scope="col">Height</th>
								<th scope="col">Total</th>
								<th scope="col">Action</th>
							</tr>
						</thead>
						<tbody>
							{entries.map((el, index) => (
								<tr key={el._id}>
									<td className="cell-mono">{index + 1}</td>
									<td>{el.date}</td>
									<td>{el.material_name}</td>
									<td className="cell-mono">{RoundOff(el.rate)}</td>
									<td className="cell-mono">{el.length}</td>
									<td className="cell-mono">{el.height}</td>
									<td className="cell-mono">{RoundOff(el.total)}</td>
									<td>
										<button
											type="button"
											className="shell-icon-btn"
											aria-label={`Delete wastage from ${el.date}`}
											title="Delete"
											onClick={() => setDeleteTarget(el)}
										>
											<Trash2 size={14} />
										</button>
									</td>
								</tr>
							))}
						</tbody>
					</DataTable>
				</div>
			</Col>

			<ConfirmDialog
				open={!!deleteTarget}
				message="Delete this wastage record?"
				confirmLabel="Delete"
				position="bottom"
				danger
				onConfirm={confirmDelete}
				onCancel={() => setDeleteTarget(null)}
			/>
		</Row>
	);
};

export default WastageForm;
