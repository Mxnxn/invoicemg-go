import React, { useState, useEffect, useCallback } from "react";
import { Row, Col, Container } from "reactstrap";
import { Edit, Plus, Trash2, Truck } from "react-feather";
import { Link } from "react-router-dom";
import { clientsBackend } from "../client_backend";
import DataTable from "../../../Common/DataTable/DataTable";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDeleteModal from "../../../Common/ConfirmDeleteModal";
import QuickCreateClientModal from "./QuickCreateClientModal";
import { personBackend } from "../../../Common/person_backend";
import { notifySuccess } from "../../../global/toast";
import SearchField from "../../../Common/SearchField";

const AddClient = () => {
	const uid = window.localStorage.getItem("uid");
	const [clients, setClients] = useState([]);
	const [search, setSearch] = useState("");
	const [moreMenu, setMoreMenu] = useState(-1);
	// The record being deleted, not just its id - the confirmation shows the name back and
	// asks for it to be typed, so it needs the whole row.
	const [deleteTarget, setDeleteTarget] = useState(null);
	// null = creating, a client = editing. One dialog serves both.
	const [formOpen, setFormOpen] = useState(false);
	const [editing, setEditing] = useState(null);

	const getClients = useCallback(async () => {
		try {
			const formData = new FormData();
			formData.set("uid", uid);
			const res = await clientsBackend.getOnlyClients(formData);
			setClients(res.data);
		} catch (error) {
			console.log(error);
		}
	}, [uid]);

	useEffect(() => {
		getClients();
	}, [getClients]);

	const onCreateClick = () => {
		setEditing(null);
		setFormOpen(true);
	};

	const onEditClick = (client) => {
		setEditing(client);
		setFormOpen(true);
		setMoreMenu(-1);
	};

	// Deleted straight away rather than through the undo bar. The undo bar holds the request
	// for five seconds and cancels it if you change your mind, which is the right trade for a
	// row you can retype - but this one takes the customer's entries with it, and the
	// confirmation already made you type the name. Two deliberate steps, then it happens.
	const confirmDelete = async () => {
		if (!deleteTarget) return;
		const id = deleteTarget._id;
		const formData = new FormData();
		formData.set("client_id", id);
		await clientsBackend.deleteClient(formData);
		setClients((prev) => prev.filter((el) => el._id !== id));
	};

	// Copies the client's details into a new, independent Supplier record - Client and
	// Supplier stay separate documents, this just saves re-typing the shared fields.
	const onAddAsSupplier = async (client) => {
		try {
			const formData = new FormData();
			formData.set("name", client.clientName);
			formData.set("type", "Supplier");
			formData.set("firm", client.clientFirm || "");
			formData.set("phone", client.clientPhone || "");
			formData.set("address", client.clientAddress || "");
			formData.set("gst", client.clientGST || "");
			await personBackend.create(formData);
			notifySuccess(`${client.clientName} added as a supplier.`);
		} catch (error) {
			console.log(error);
		} finally {
			setMoreMenu(-1);
		}
	};

	const filtered = clients.filter((el) => {
		if (!search) return true;
		const needle = search.toUpperCase();
		return (
			(el.clientName || "").toUpperCase().includes(needle) ||
			(el.clientFirm || "").toUpperCase().includes(needle) ||
			(el.clientPhone || "").toUpperCase().includes(needle) ||
			(el.clientGST || "").toUpperCase().includes(needle)
		);
	});

	// Honours Account settings > Appearance > Rows per page, like every other list.
	const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered);

	return (
		<>
			<Container fluid>
				<Row className="mt-2">
					{/* One full-width table. The editing form used to sit beside it in a 5/7 split,
					    which cost the table nearly half its width permanently so that a form could be
					    on screen for the few seconds a month anyone edits a customer - and the table
					    is the thing people come here to read. The form is a dialog now, and it is the
					    same dialog the job-id and quotation screens open. */}
					<Col xl="12">
						<div className="shell-card">
							<div className="shell-card-header">
								<span className="text-heading-brand">Customers</span>
								<div className="d-flex align-items-center" style={{ gap: 10 }}>
									<SearchField placeholder="Search name, firm, phone, GST" value={search} onChange={(e) => setSearch(e.target.value)} />
									<button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={onCreateClick}>
										<Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
										Create Customer
									</button>
								</div>
							</div>
							<DataTable>
								<thead>
									<tr>
										<th scope="col">Client</th>
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
												{search ? "No customers match that search." : "No customers yet."}
											</td>
										</tr>
									) : (
										pageRows.map((el, index) => (
											<tr key={el._id}>
												<td>
													<Link to={`/customer/${el._id}`}>{el.clientName}</Link>
												</td>
												<td>{el.clientFirm}</td>
												<td className="cell-mono">{el.clientPhone}</td>
												<td className="cell-mono">{el.clientGST || <span style={{ color: "var(--text-tertiary)" }}>&mdash;</span>}</td>
												<td>{el.clientAddress || <span style={{ color: "var(--text-tertiary)" }}>&mdash;</span>}</td>
												<td>
													<RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
														<RowActionMenuItem icon={Edit} variant="warning" onClick={() => onEditClick(el)}>
															Edit
														</RowActionMenuItem>
														<RowActionMenuItem icon={Truck} onClick={() => onAddAsSupplier(el)}>
															Add as Supplier
														</RowActionMenuItem>
														<RowActionMenuItem
															icon={Trash2}
															variant="danger"
															onClick={() => {
																setDeleteTarget(el);
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
									{perPage > 0 && total > perPage ? `${pageRows.length} of ${total} customers` : `${total} customers`}
								</span>
								<Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
							</div>
						</div>
					</Col>
				</Row>
			</Container>

			<QuickCreateClientModal
				isOpen={formOpen}
				toggle={() => setFormOpen(false)}
				client={editing}
				onCreated={(created) => setClients((prev) => [created, ...prev])}
				onUpdated={(saved) => setClients((prev) => prev.map((el) => (el._id === saved._id ? saved : el)))}
			/>

			{/* Typed confirmation, not a one-click dialog: deleting a customer takes their
			    entries with them, and there is no restore endpoint to undo it with. */}
			<ConfirmDeleteModal
				isOpen={!!deleteTarget}
				toggle={() => setDeleteTarget(null)}
				kind="customer"
				name={deleteTarget?.clientName || ""}
				warning="Every entry recorded against this customer is deleted with them. This cannot be undone."
				onConfirm={confirmDelete}
			/>
		</>
	);
};

export default AddClient;
