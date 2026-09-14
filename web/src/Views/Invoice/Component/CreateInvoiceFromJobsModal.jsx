import React, { useEffect, useMemo, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup } from "reactstrap";
import { FileText, AlertTriangle } from "react-feather";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { invoiceBackend } from "../invoice_backend";
import { formatAmount } from "../../../Common/money";
import DateField from "../../../Common/DateField";

// Build an invoice by picking job-ids rather than individual entries.
//
// An Invoice is stored against Entries, but people think in job-ids - one job is one piece of
// work a customer handed over. This picks the jobs and resolves them to their entry ids
// behind the scenes, so the mental model matches the paperwork.
//
// Invoices are stored against Entries, but you should not have to convert a job by hand
// first. Any Done row that has not been converted is converted as part of creating the
// invoice, reusing /lifecycle/jobs/convert-to-entries rather than a parallel code path -
// that endpoint already refuses rows which are not Done or are already converted.
//
// /invoice/invoiceable-jobs reports invoicedRows and unconvertedRows per job, so a partially
// available job explains itself instead of quietly offering less than expected.
// initialClient/initialJobIds let the Jobs board hand over a selection it has already made,
// so opening from there lands on the right client with those job-ids ticked instead of
// asking the user to pick them a second time. Opened from the Invoices page both are absent
// and the modal starts empty, exactly as before.
const CreateInvoiceFromJobsModal = ({ isOpen, toggle, onCreated, initialClient = null, initialJobIds = [] }) => {
	const [clients, setClients] = useState([]);
	const [client, setClient] = useState({ id: "", name: "" });
	const [jobs, setJobs] = useState([]);
	const [selected, setSelected] = useState([]);
	const [invNo, setInvNo] = useState("");
	const [date, setDate] = useState(() => new Date().toISOString().slice(0, 10));
	const [loading, setLoading] = useState(false);
	const [saving, setSaving] = useState(false);

	useEffect(() => {
		if (!isOpen) return;
		setClient(initialClient || { id: "", name: "" });
		setJobs([]);
		setSelected(initialJobIds.map(String));
		setDate(new Date().toISOString().slice(0, 10));
		lifecycleBackend
			.lookupClients()
			.then((res) => setClients(res.data || []))
			.catch(() => setClients([]));
		invoiceBackend
			.nextInvoiceNumber()
			// The endpoint returns { invoiceNumber }, not a bare string - assigning the whole
			// object rendered "[object Object]" in the field.
			.then((res) => setInvNo(res.data?.invoiceNumber || ""))
			.catch(() => setInvNo(""));
	}, [isOpen]);

	useEffect(() => {
		if (!client.id) {
			setJobs([]);
			setSelected([]);
			return;
		}
		// A handed-over selection must survive this load; only clear it when the user picks a
		// different client themselves.
		setLoading(true);
		invoiceBackend
			.invoiceableJobs(client.id)
			.then((res) => {
				const list = res.data || [];
				setJobs(list);
				setSelected((prev) => prev.filter((id) => list.some((j) => String(j._id) === String(id))));
			})
			.catch(() => setJobs([]))
			.finally(() => setLoading(false));
	}, [client.id]);

	const clientOptions = clients.map((c) => ({
		id: c._id,
		name: c.clientFirm || c.clientName,
		search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
	}));

	const toggleJob = (id) =>
		setSelected((prev) => (prev.includes(id) ? prev.filter((j) => j !== id) : [...prev, id]));

	const chosen = useMemo(() => jobs.filter((j) => selected.includes(j._id)), [jobs, selected]);
	const entryIds = useMemo(() => chosen.flatMap((j) => j.entryIds), [chosen]);
	// Interstate job-ids among the selection. Worth saying out loud before the invoice is
	// raised: an IGST invoice files under a different head, and it is listed on its own tab.
	const igstJobs = useMemo(() => chosen.filter((j) => j.hasIgst), [chosen]);
	const total = useMemo(() => chosen.reduce((sum, j) => sum + Number(j.amount || 0), 0), [chosen]);
	// Done rows on the chosen jobs that still need converting - stated up front so the button
	// does not quietly do more than it says.
	const pendingConversion = useMemo(
		() => chosen.reduce((sum, j) => sum + Number(j.convertibleRows || 0), 0),
		[chosen]
	);

	const canSubmit = client.id && (entryIds.length > 0 || pendingConversion > 0) && invNo && date && !saving;

	const onSubmit = async () => {
		if (!canSubmit) return;
		setSaving(true);
		try {
			// Invoice straight from the job-ids. The server converts any Done, unconverted row
			// itself (Helpers/ConvertJobRows.js) and records which jobs the invoice covers, so
			// there is no convert-then-re-read dance here any more - and no window where the
			// rows were converted but the invoice failed to save.
			const res = await invoiceBackend.saveInvoice({
				job_ids: chosen.map((j) => j._id),
				client_id: client.id,
				date,
				invNo,
			});
			onCreated(res.data);
			toggle();
		} catch (error) {
			// errorInterceptor raises the toast.
		} finally {
			setSaving(false);
		}
	};

	return (
		<Modal isOpen={isOpen} toggle={toggle} size="lg" centered>
			<ModalHeader toggle={toggle}>
				<span className="confirm-modal-title">New invoice from job-ids</span>
			</ModalHeader>
			<ModalBody>
				<div className="d-flex" style={{ gap: 12, flexWrap: "wrap" }}>
					<FormGroup style={{ flex: "1 1 220px" }}>
						<label className="form-control-label pp fs-12">
							Client<span className="required-star">*</span>
						</label>
						<AssigneeDropdown
							value={client.name}
							placeholder="Select client"
							options={clientOptions}
							fullWidth
							menuWidth={320}
							onSelect={(id, name) => setClient({ id: id || "", name: name || "" })}
						/>
					</FormGroup>
					<FormGroup style={{ flex: "0 1 160px" }}>
						<label className="form-control-label pp fs-12">
							Invoice no.<span className="required-star">*</span>
						</label>
						<input className="dev-input" value={invNo} onChange={(e) => setInvNo(e.target.value)} />
					</FormGroup>
					<FormGroup style={{ flex: "0 1 160px" }}>
						<label className="form-control-label pp fs-12">
							Date<span className="required-star">*</span>
						</label>
						<DateField value={date} onChange={(e) => setDate(e.target.value)} />
					</FormGroup>
				</div>

				{!client.id ? (
					<p className="dev-muted">Pick a client to see their billable job-ids.</p>
				) : loading ? (
					<p className="dev-muted">Loading job-ids…</p>
				) : jobs.length === 0 ? (
					<div className="pending-jobs-strip">
						<AlertTriangle size={16} />
						<span>
							Nothing to invoice for this client. Job rows have to be converted to entries first, from the
							job-id detail view.
						</span>
					</div>
				) : (
					<>
					{igstJobs.length > 0 && (
						<div className="invoice-igst-note" role="status">
							<AlertTriangle size={14} />
							<span>
								{igstJobs.length === 1 ? "This job-id carries" : `${igstJobs.length} of these job-ids carry`} IGST
								— the invoice will be interstate and appears under the IGST tab.
							</span>
						</div>
					)}
					<div className="job-pick-list">
						{jobs.map((job) => {
							const isOn = selected.includes(job._id);
							// Defensive only: the server no longer sends a job-id with work still
							// in production, so this should never be true.
							const nothingToBill = job.entryIds.length === 0 && !job.convertibleRows;
							return (
								<button
									type="button"
									key={job._id}
									className={["job-pick", isOn ? "is-on" : "", nothingToBill ? "is-waiting" : ""]
										.filter(Boolean)
										.join(" ")}
									aria-pressed={isOn}
									disabled={nothingToBill}
									title={nothingToBill ? "Every remaining row is still in production." : undefined}
									onClick={() => toggleJob(job._id)}
								>
									<span className="job-pick-main">
										<span className="job-pick-id">
											<FileText size={14} /> {job.challanNumber}
										</span>
										<span className="dev-muted">
											{job.receivedDate} · {job.entryIds.length} of {job.rowCount} row
											{job.rowCount === 1 ? "" : "s"} billable
											{job.invoicedRows > 0 ? ` · ${job.invoicedRows} already invoiced` : ""}
											{job.unconvertedRows > 0 ? ` · ${job.unconvertedRows} not converted` : ""}
										</span>
									</span>
									<span className="job-pick-amount cell-mono">{formatAmount(job.amount)}</span>
								</button>
							);
						})}
					</div>
					</>
				)}
			</ModalBody>
			<ModalFooter style={{ justifyContent: "space-between" }}>
				<span className="dev-muted">
					{entryIds.length} entr{entryIds.length === 1 ? "y" : "ies"} selected · {formatAmount(total)}
					{pendingConversion > 0
						? ` · ${pendingConversion} row${pendingConversion === 1 ? "" : "s"} not yet billed`
						: ""}
				</span>
				<span style={{ display: "flex", gap: 10 }}>
					<button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={saving}>
						Cancel
					</button>
					<button type="button" className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={!canSubmit}>
						{saving ? "Creating…" : "Create invoice"}
					</button>
				</span>
			</ModalFooter>
		</Modal>
	);
};

export default CreateInvoiceFromJobsModal;
