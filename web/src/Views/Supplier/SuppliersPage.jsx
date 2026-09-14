import React, { useCallback, useEffect, useState } from "react";

import { Row, Col, Container } from "reactstrap";
import { Edit, Plus, Trash2 } from "react-feather";

import { personBackend } from "../../Common/person_backend";
import Loader from "../../global/Loader/Loader";
import DataTable from "../../Common/DataTable/DataTable";
import Pagination from "../../Shell/Pagination";
import usePagedRows from "../../Common/useRowsPerPage";
import RowActionMenu, { RowActionMenuItem } from "../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../Common/ConfirmDialog";
import { useUndoDelete } from "../../Common/undoDelete";
import SearchField from "../../Common/SearchField";
import QuickCreateSupplierModal from "../PurchaseInvoice/component/QuickCreateSupplierModal";

const SuppliersPage = () => {
	const [loading, setLoading] = useState(false);
	const [suppliers, setSuppliers] = useState([]);
	const [search, setSearch] = useState("");
	const [moreMenu, setMoreMenu] = useState(-1);
	const [deleteTarget, setDeleteTarget] = useState(null);
	const { scheduleDelete } = useUndoDelete();

	// null = creating, a supplier = editing. One dialog serves both, and it is the same dialog
	// the purchase-invoice screen opens - see QuickCreateSupplierModal.
	const [formOpen, setFormOpen] = useState(false);
	const [editing, setEditing] = useState(null);

	const getSuppliers = useCallback(async () => {
		try {
			setLoading(true);
			const res = await personBackend.list("Supplier");
			setSuppliers(res.data);
		} catch (error) {
			console.log(error);
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		getSuppliers();
	}, [getSuppliers]);

	const onCreateClick = () => {
		setEditing(null);
		setFormOpen(true);
	};

	const onEditClick = (supplier) => {
		setEditing(supplier);
		setFormOpen(true);
		setMoreMenu(-1);
	};

	// Through the undo bar rather than a typed confirmation: a supplier is a row you can
	// retype, and holding the request for five seconds makes an accidental delete free.
	const confirmDelete = () => {
		if (!deleteTarget) return;
		const id = deleteTarget;
		const removed = suppliers.find((el) => el._id === id);
		const index = suppliers.findIndex((el) => el._id === id);
		setSuppliers((prev) => prev.filter((el) => el._id !== id));
		setDeleteTarget(null);
		scheduleDelete({
			label: "supplier",
			commit: () => {
				const formData = new FormData();
				formData.set("person_id", id);
				return personBackend.delete(formData);
			},
			undo: () => setSuppliers((prev) => [...prev.slice(0, index), removed, ...prev.slice(index)]),
		});
	};

	const filtered = suppliers.filter((el) => {
		if (!search) return true;
		const needle = search.toUpperCase();
		return (
			el.name.toUpperCase().includes(needle) ||
			(el.firm || "").toUpperCase().includes(needle) ||
			(el.phone || "").toUpperCase().includes(needle) ||
			(el.gst || "").toUpperCase().includes(needle)
		);
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
					{/* One full-width table, the same shape as Configure > Customers and Products.
					    The editing form used to sit beside it in a 5/7 split, which cost the table
					    nearly half its width permanently for a form used a few seconds a month. */}
					<Col xl="12">
						<div className="shell-card">
							<div className="shell-card-header">
								<span className="text-heading-brand">Suppliers</span>
								<div className="d-flex align-items-center" style={{ gap: 10 }}>
									<SearchField
										placeholder="Search suppliers, firm, phone, GST"
										value={search}
										onChange={(e) => setSearch(e.target.value)}
									/>
									<button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={onCreateClick}>
										<Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
										Create Supplier
									</button>
								</div>
							</div>
							<DataTable loading={loading}>
								<thead>
									<tr>
										<th scope="col">Name</th>
										<th scope="col">Firm</th>
										<th scope="col">Phone</th>
										<th scope="col">GST</th>
										<th scope="col">Address</th>
										<th scope="col">Action</th>
									</tr>
								</thead>
								<tbody>
									{filtered.length === 0 ? (
										<tr>
											<td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
												{search ? "No suppliers match that search." : "No suppliers yet."}
											</td>
										</tr>
									) : (
										pageRows.map((el, index) => (
											<tr key={el._id}>
												<td>{el.name}</td>
												<td>{el.firm || dash}</td>
												<td className="cell-mono">{el.phone || dash}</td>
												<td className="cell-mono">{el.gst || dash}</td>
												<td>{el.address || dash}</td>
												<td>
													<RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
														<RowActionMenuItem icon={Edit} variant="warning" onClick={() => onEditClick(el)}>
															Edit
														</RowActionMenuItem>
														<RowActionMenuItem
															icon={Trash2}
															variant="danger"
															onClick={() => {
																setDeleteTarget(el._id);
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
									{perPage > 0 && total > perPage ? `${pageRows.length} of ${total} suppliers` : `${total} suppliers`}
								</span>
								<Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
							</div>
						</div>
					</Col>
				</Row>
			</Container>

			<QuickCreateSupplierModal
				isOpen={formOpen}
				toggle={() => setFormOpen(false)}
				supplier={editing}
				onCreated={(created) => setSuppliers((prev) => [created, ...prev])}
				onUpdated={(saved) => setSuppliers((prev) => prev.map((el) => (el._id === saved._id ? saved : el)))}
			/>

			<ConfirmDialog
				open={!!deleteTarget}
				message="Delete this supplier?"
				confirmLabel="Delete"
				position="bottom"
				danger
				onConfirm={confirmDelete}
				onCancel={() => setDeleteTarget(null)}
			/>
		</>
	);
};

export default SuppliersPage;
