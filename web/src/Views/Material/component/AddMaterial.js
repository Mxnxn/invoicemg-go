import React, { useCallback, useEffect, useState } from "react";

import { Row, Col, Container } from "reactstrap";
import { Edit, Plus, Trash2 } from "react-feather";

import { materialsBackend } from "../material_backend";
import MaterialHelper from "../material_helper";
import Loader from "../../../global/Loader/Loader";
import DataTable from "../../../Common/DataTable/DataTable";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import UnitManager from "../../../Common/UnitManager";
import { useUndoDelete } from "../../../Common/undoDelete";
import SearchField from "../../../Common/SearchField";
import QuickCreateProductModal from "./QuickCreateProductModal";

const materialHelper = new MaterialHelper();

const AddMaterial = () => {
	const [loading, setLoading] = useState(false);
	const [products, setProducts] = useState([]);
	const [search, setSearch] = useState("");
	const [moreMenu, setMoreMenu] = useState(-1);
	const [deleteTarget, setDeleteTarget] = useState(null);
	const { scheduleDelete } = useUndoDelete();

	// null = creating, a product = editing. One dialog serves both, and it is the same dialog
	// the job card opens - see QuickCreateProductModal.
	const [formOpen, setFormOpen] = useState(false);
	const [editing, setEditing] = useState(null);

	const getProducts = useCallback(async () => {
		try {
			setLoading(true);
			const res = await materialsBackend.getAllMaterials(new FormData(), window.localStorage.getItem("session_token"));
			setProducts(res.data.map((el) => materialHelper.toMaterialEntry(el._id, el)));
		} catch (error) {
			console.log(error);
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		getProducts();
	}, [getProducts]);

	const onCreateClick = () => {
		setEditing(null);
		setFormOpen(true);
	};

	const onEditClick = (product) => {
		setEditing(product);
		setFormOpen(true);
		setMoreMenu(-1);
	};

	// Through the undo bar rather than a typed confirmation. A product is a row you can
	// retype, and holding the request for five seconds makes an accidental delete free -
	// which is the right trade here. A customer is the opposite case: deleting one takes
	// their entries with it and there is no restore, so that one asks for the name.
	const confirmDelete = () => {
		if (!deleteTarget) return;
		const id = deleteTarget;
		const removed = products.find((el) => el.id === id);
		const index = products.findIndex((el) => el.id === id);
		setProducts((prev) => prev.filter((el) => el.id !== id));
		setDeleteTarget(null);
		scheduleDelete({
			label: "product",
			commit: () => {
				const formData = new FormData();
				formData.set("material_id", id);
				return materialsBackend.deleteMaterial(formData, window.localStorage.getItem("session_token"));
			},
			undo: () => setProducts((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
		});
	};

	const filtered = products.filter((el) => {
		if (!search) return true;
		const needle = search.toUpperCase();
		return el.material_name.toUpperCase().includes(needle) || (el.hsn || "").toUpperCase().includes(needle);
	});

	// Honours Account settings > Appearance > Rows per page, like every other list.
	const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

	const dash = <span style={{ color: "var(--text-tertiary)" }}>&mdash;</span>;

	return loading ? (
		<Loader />
	) : (
		<>
			<Container fluid>
				<Row className="mt-2">
					{/* One full-width table, the same shape as Configure > Customers. The editing
					    form used to sit beside it in a 5/7 split, which cost the table nearly half
					    its width permanently so that a form could be on screen for the few seconds
					    a month anyone edits a product - and the table is what people come here to
					    read. The form is a dialog now, and it is the same dialog the job-id screen
					    opens. */}
					<Col xl="12">
						<div className="shell-card">
							<div className="shell-card-header">
								<span className="text-heading-brand">Products</span>
								<div className="d-flex align-items-center" style={{ gap: 10 }}>
									<SearchField placeholder="Search products or HSN" value={search} onChange={(e) => setSearch(e.target.value)} />
									<button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={onCreateClick}>
										<Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
										Create Product
									</button>
								</div>
							</div>
							<DataTable loading={loading}>
								<thead>
									<tr>
										<th scope="col">Name</th>
										<th scope="col">Rate</th>
										<th scope="col">Purchase rate</th>
										<th scope="col">HSN</th>
										<th scope="col">Unit</th>
										<th scope="col">Tax %</th>
										<th scope="col">Action</th>
									</tr>
								</thead>
								<tbody>
									{filtered.length === 0 ? (
										<tr>
											<td colSpan={7} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
												{search ? "No products match that search." : "No products yet."}
											</td>
										</tr>
									) : (
										pageRows.map((el, index) => (
											<tr key={el.id}>
												<td>{el.material_name}</td>
												<td className="cell-mono">{el.material_rate}</td>
												<td className="cell-mono">{el.purchase_rate}</td>
												<td className="cell-mono">{el.hsn || dash}</td>
												{/* Shown because it is required and decides how the product
												    depletes on the inventory report - a product missing one
												    was invisible here while quietly counting for nothing. */}
												<td className="cell-mono">{el.unit || dash}</td>
												<td className="cell-mono">{el.tax || 0}%</td>
												<td>
													<RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
														<RowActionMenuItem icon={Edit} variant="warning" onClick={() => onEditClick(el)}>
															Edit
														</RowActionMenuItem>
														<RowActionMenuItem
															icon={Trash2}
															variant="danger"
															onClick={() => {
																setDeleteTarget(el.id);
																setMoreMenu(-1);
															}}
														>
															Delete
														</RowActionMenuItem>
													</RowActionMenu>
												</td>
											</tr>
										))
									)}
								</tbody>
							</DataTable>
							<div className="table-foot">
								<span className="text-body-small table-foot-count">
									{perPage > 0 && total > perPage ? `${pageRows.length} of ${total} products` : `${total} products`}
								</span>
								<Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
							</div>
						</div>
					</Col>
				</Row>
				<Row>
					<Col xl="12">
						<UnitManager />
					</Col>
				</Row>
			</Container>

			<QuickCreateProductModal
				isOpen={formOpen}
				toggle={() => setFormOpen(false)}
				product={editing}
				onCreated={(created) => setProducts((prev) => [materialHelper.toMaterialEntry(created._id, created), ...prev])}
				onUpdated={(saved) => {
					const updated = materialHelper.toMaterialEntry(saved._id, saved);
					setProducts((prev) => prev.map((el) => (el.id === updated.id ? updated : el)));
				}}
			/>

			<ConfirmDialog
				open={!!deleteTarget}
				message="Delete this product?"
				confirmLabel="Delete"
				position="bottom"
				danger
				onConfirm={confirmDelete}
				onCancel={() => setDeleteTarget(null)}
			/>
		</>
	);
};

export default AddMaterial;
