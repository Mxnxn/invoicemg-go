import React, { useEffect, useState } from "react";
import { Zap, Server, Send } from "react-feather";
import PasswordInput from "../../Common/PasswordInput";
import { whatsappBackend } from "../../Common/whatsapp_backend";
import TemplatePicker from "./TemplatePicker";
import NotifyPreferences from "./NotifyPreferences";
import ReminderPreferences from "./ReminderPreferences";
import { personBackend } from "../../Common/person_backend";
import { clientsBackend } from "../Client/client_backend";
import { notifySuccess, notifyError } from "../../global/toast";

// WhatsApp credentials for this Company - see routes/WhatsApp.js.
//
// Two ways to connect, one message format. Meta's own Cloud API, or a BSP that resells it
// (360dialog and similar) - those accept the identical request body, differing only in host
// and key header. Each has its own form, because the fields have nothing in common and a
// single merged form would ask for values that do not apply.
//
// Secrets are write-only: never echoed back, so those fields start blank and only report
// whether one is already on file. Leaving one blank keeps the stored value.
// What each side can be told about. All at module scope because they are constants: rebuilt
// on every render they would be a new `events` identity for ReminderPreferences each time,
// and its loader keys off exactly that.
//
// Suppliers only, and customers. Employees are not here: they do not receive WhatsApp about
// jobs, so a tab of switches for them would have been settings that control nothing.
//
// `fallback` is what an unanswered supplier cell inherits, and it must match DEFAULTS in the
// API's Helpers/NotifyPreference.js - the cell says what the default DOES, and a cell that
// says the wrong thing is worse than no cell. Customer cells carry no fallback: an unanswered
// customer is asked each time rather than inheriting anything, which is why that table runs
// with nullMeans="ask".
const SUPPLIER_EVENTS = [
	{ field: "notifyPoCreated", label: "Order sent", hint: "An approved purchase order is shared", fallback: true },
	{
		field: "notifyPoUpdated",
		label: "Order updated",
		hint: "A shared order is shared again after a change",
		fallback: true,
	},
	{ field: "notifyPoConfirmed", label: "Dispatch", hint: "The dispatch confirmation", fallback: true },
];

const CUSTOMER_EVENTS = [
	{ field: "notifyOnCreate", label: "Job created", hint: "A job-id is raised for them" },
	{ field: "notifyOnUpdate", label: "Job updated", hint: "A job-id they were told about changes" },
];

// The two adapters. They exist so ReminderPreferences knows about neither backend: each hands
// back rows already flattened to the shape the table reads, which is what lets one table serve
// a Person and a Client despite the two naming every field differently.
const loadSuppliers = () => personBackend.list("Supplier").then((res) => res.data || []);

const saveSupplier = (id, field, value) => personBackend.setNotifyPreference(id, field, value);

const loadCustomers = () =>
	clientsBackend.listNotifyPreferences(true).then((res) =>
		(res.data || []).map((c) => ({
			_id: c._id,
			name: c.clientName,
			firm: c.clientFirm,
			phone: c.clientPhone,
			notifyOnCreate: c.notifyOnCreate,
			notifyOnUpdate: c.notifyOnUpdate,
		}))
	);

// The client route takes a `kind` rather than a field name, and has done since long before
// this table existed - translated here rather than changing a route the post-create prompt
// also calls.
const saveCustomer = (id, field, value) => {
	const formData = new FormData();
	formData.set("client_id", id);
	formData.set("kind", field === "notifyOnUpdate" ? "updated" : "created");
	formData.set("value", value);
	return clientsBackend.setNotifyPreference(formData);
};

const WhatsAppManager = () => {
	const [config, setConfig] = useState(null);
	const [provider, setProvider] = useState("meta");
	// Which row is highlighted in the picker. Display only for now - the Done alert still
	// sends order_management_2 (Helpers/JobDoneAlert.js); wiring the choice through to the
	// send is a separate change, since each template needs its variables mapped to real data.
	const [selectedTemplate, setSelectedTemplate] = useState("order_management_2");
	const [saving, setSaving] = useState(false);
	// Setup first, so the page opens where it has always opened. The two preference tables are
	// peers of it rather than more cards below it: they are lists of people, not part of
	// connecting an account, and the page was already long enough to scroll past.
	const [tab, setTab] = useState("setup");

	// Which approved template each alert button sends. Names rather than ids, because that is
	// what the Cloud API takes and what Helpers/JobCreatedAlert.js and JobDoneAlert.js read.
	const [createdTemplate, setCreatedTemplate] = useState("");
	// Sent instead of the created template when a job-id that was already announced
	// changes - see DEFAULT_UPDATED_TEMPLATE in Helpers/JobCreatedAlert.js, which this
	// overrides per company.
	const [updatedTemplate, setUpdatedTemplate] = useState("");
	const [doneTemplate, setDoneTemplate] = useState("");
	// The purchase-order pair plus the dispatch confirmation. On Company since the purchase
	// order work but never settable until now, so every company has been sending the built-in
	// defaults whether or not it had them approved.
	const [poCreatedTemplate, setPoCreatedTemplate] = useState("");
	const [poUpdateTemplate, setPoUpdateTemplate] = useState("");
	const [poConfirmTemplate, setPoConfirmTemplate] = useState("");
	// Published by the picker beside these fields, so the mapping offers exactly the templates
	// that are actually approved instead of a free-text box that can be quietly wrong.
	const [approved, setApproved] = useState([]);
	// Whether raising a job-id offers to tell the customer straight away, and whether that
	// offer sends on its own. Off means the prompt still appears - it just waits to be asked.
	const [notifyOnCreate, setNotifyOnCreate] = useState(false);
	const [notifyOnUpdate, setNotifyOnUpdate] = useState(false);

	// Meta Cloud API
	const [phoneNumberId, setPhoneNumberId] = useState("");
	const [businessAccountId, setBusinessAccountId] = useState("");
	const [apiToken, setApiToken] = useState("");

	// BSP
	const [bspBaseUrl, setBspBaseUrl] = useState("");
	const [bspAuthHeader, setBspAuthHeader] = useState("");
	const [bspApiKey, setBspApiKey] = useState("");

	const load = () => {
		whatsappBackend.getConfig().then((res) => {
			setConfig(res.data);
			setProvider(res.data.provider || "meta");
			setPhoneNumberId(res.data.phoneNumberId || "");
			setBusinessAccountId(res.data.businessAccountId || "");
			setBspBaseUrl(res.data.bspBaseUrl || "");
			setBspAuthHeader(res.data.bspAuthHeader || "");
			setCreatedTemplate(res.data.createdTemplate || "");
			setUpdatedTemplate(res.data.updatedTemplate || "");
			setDoneTemplate(res.data.doneTemplate || "");
			setPoCreatedTemplate(res.data.poCreatedTemplate || "");
			setPoUpdateTemplate(res.data.poUpdateTemplate || "");
			setPoConfirmTemplate(res.data.poConfirmTemplate || "");
			setNotifyOnCreate(Boolean(res.data.notifyOnCreate));
			setNotifyOnUpdate(Boolean(res.data.notifyOnUpdate));
		});
	};

	useEffect(() => {
		load();
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const save = async (e) => {
		e.preventDefault();

		// Validate only the form being submitted - the other provider's blanks are irrelevant.
		if (provider === "meta") {
			if (!phoneNumberId.trim() || !businessAccountId.trim()) {
				return notifyError("Phone number ID and business account ID are both required.");
			}
			if (!config.hasApiToken && !apiToken.trim()) {
				return notifyError("An API token is required the first time you connect.");
			}
		} else {
			if (!bspBaseUrl.trim()) return notifyError("The provider's API base URL is required.");
			if (!phoneNumberId.trim()) return notifyError("Phone number ID is required.");
			if (!config.hasBspApiKey && !bspApiKey.trim()) {
				return notifyError("An API key is required the first time you connect.");
			}
		}

		setSaving(true);
		try {
			const formData = new FormData();
			formData.set("provider", provider);
			formData.set("phoneNumberId", phoneNumberId);
			formData.set("businessAccountId", businessAccountId);
			formData.set("bspBaseUrl", bspBaseUrl);
			formData.set("bspAuthHeader", bspAuthHeader);
			// Blank secrets mean "keep what is stored" - the API applies the same rule.
			if (apiToken) formData.set("apiToken", apiToken);
			if (bspApiKey) formData.set("bspApiKey", bspApiKey);
			// Not secrets: a blank one is a real choice ("clear it"), so these always travel.
			formData.set("notifyOnCreate", String(notifyOnCreate));
			formData.set("notifyOnUpdate", String(notifyOnUpdate));
			formData.set("createdTemplate", createdTemplate);
			formData.set("updatedTemplate", updatedTemplate);
			formData.set("doneTemplate", doneTemplate);
			formData.set("poCreatedTemplate", poCreatedTemplate);
			formData.set("poUpdateTemplate", poUpdateTemplate);
			formData.set("poConfirmTemplate", poConfirmTemplate);

			const res = await whatsappBackend.updateConfig(formData);
			setConfig(res.data);
			setApiToken("");
			setBspApiKey("");
		} finally {
			setSaving(false);
		}
	};

	if (!config) return null;

	const field = (label, value, onChange, props = {}) => (
		<div style={{ marginBottom: 14 }}>
			<label className="form-control-label pp fs-12" style={{ display: "block" }}>
				{label}
			</label>
			<input
				className="form-control form-control-alternative nn"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				autoComplete="off"
				{...props}
			/>
		</div>
	);

	// Which approved template each button should offer. A select rather than a text field:
	// the names come from the account itself, and a typo here fails at Meta rather than here.
	const templateSelect = (label, value, onChange, hint) => (
		<div style={{ marginBottom: 14 }}>
			<label className="form-control-label pp fs-12" style={{ display: "block" }}>
				{label}
			</label>
			<select className="form-control form-control-alternative nn" value={value} onChange={(e) => onChange(e.target.value)}>
				<option value="">Use the default</option>
				{/* A saved name that is no longer approved still has to be selectable, or
				    opening this form would silently re-map the alert to something else. */}
				{(approved.some((t) => t.name === value) || !value ? approved : [{ name: value, status: "UNKNOWN" }, ...approved]).map(
					(t) => (
						<option key={t.name} value={t.name}>
							{t.name}
							{t.status && t.status !== "APPROVED" ? ` (${t.status})` : ""}
						</option>
					)
				)}
			</select>
			<span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
				{hint}
			</span>
		</div>
	);

	return (
		<>
			<div className="shell-segmented" role="tablist" style={{ marginBottom: 16 }}>
				{[
					{ key: "setup", label: "Setup" },
					{ key: "customers", label: "Customers" },
					{ key: "suppliers", label: "Suppliers" },
				].map((t) => (
					<button
						key={t.key}
						type="button"
						role="tab"
						aria-selected={tab === t.key}
						className="shell-segmented-btn"
						style={tab === t.key ? { background: "var(--xan-blue-bg)", color: "var(--xan-blue)" } : undefined}
						onClick={() => setTab(t.key)}
					>
						{t.label}
					</button>
				))}
			</div>

			{tab === "customers" && (
				<ReminderPreferences
					title="Customers"
					noun="customer"
					events={CUSTOMER_EVENTS}
					load={loadCustomers}
					save={saveCustomer}
					// A customer with no answer is ASKED, by the card that appears after a
					// job-id is raised. That is a real third behaviour rather than a quiet no,
					// so the cells cycle through all three states instead of toggling two.
					nullMeans="ask"
					intro="Whether each customer is told on WhatsApp when a job-id is raised for them, or changes afterwards. Anyone left on “Asks each time” gets the offer after the job-id is raised, and nothing is sent until someone answers it."
					emptyHint="No customers yet."
				/>
			)}

			{tab === "suppliers" && (
				<ReminderPreferences
					title="Suppliers"
					noun="supplier"
					events={SUPPLIER_EVENTS}
					load={loadSuppliers}
					save={saveSupplier}
					intro="Which purchase order messages each supplier should get. A supplier turned off here is refused at the moment of sending, with the reason named."
					emptyHint="No suppliers yet — add them under Suppliers."
				/>
			)}

			{/* Hidden rather than unmounted, unlike the two tables above: this form holds
			    unsaved edits in component state, and unmounting it on a tab change would throw
			    away half-typed credentials with no warning. The tables hold nothing unsaved -
			    every press writes immediately - so unmounting them is free, and means each one
			    re-reads its people on return rather than showing a list that went stale. */}
			<div className="row" hidden={tab !== "setup"}>
			<div className="col-12 col-lg-6">
				<div className="shell-card">
					<div className="shell-card-header">
						<span className="text-heading-brand">WhatsApp</span>
					</div>

			<div style={{ padding: 20 }}>
				{/* Which connection this company uses. Switching does not discard the other
				    form's stored values - only the selected one is validated and used. */}
				<div className="segmented segmented--field" role="radiogroup" aria-label="Connection type">
					<button
						type="button"
						role="radio"
						aria-checked={provider === "meta"}
						className={["segmented-option", provider === "meta" ? "active" : ""].filter(Boolean).join(" ")}
						onClick={() => setProvider("meta")}
					>
						<Zap size={14} />
						<span>
							Meta Cloud API<small>direct from Meta</small>
						</span>
					</button>
					<button
						type="button"
						role="radio"
						aria-checked={provider === "bsp"}
						className={["segmented-option", provider === "bsp" ? "active" : ""].filter(Boolean).join(" ")}
						onClick={() => setProvider("bsp")}
					>
						<Server size={14} />
						<span>
							Via a BSP<small>360dialog and similar</small>
						</span>
					</button>
				</div>

				<form autoComplete="off" onSubmit={save}>
					{provider === "meta" ? (
						<div className="auth-panel" key="meta">
							<p className="text-body-small" style={{ color: "var(--text-tertiary)", marginBottom: 16 }}>
								Get these from the{" "}
								<a
									href="https://developers.facebook.com/apps"
									target="_blank"
									rel="noreferrer"
									style={{ color: "var(--xan-blue)" }}
								>
									Meta developer console
								</a>{" "}
								under WhatsApp → API Setup. Use a permanent system-user token, not the 24-hour test token.
							</p>

							{field("Phone Number ID", phoneNumberId, setPhoneNumberId, {
								placeholder: "e.g. 109876543210987",
								name: "whatsapp-phone-number-id",
							})}
							{field("Business Account ID", businessAccountId, setBusinessAccountId, {
								placeholder: "e.g. 123456789012345",
								name: "whatsapp-business-account-id",
							})}

							<div style={{ marginBottom: 6 }}>
								<label className="form-control-label pp fs-12" style={{ display: "block" }}>
									API Token
								</label>
								<PasswordInput
									inputClassName="form-control form-control-alternative nn"
									autoComplete="new-password"
									name="whatsapp-api-token"
									value={apiToken}
									onChange={(e) => setApiToken(e.target.value)}
									placeholder={
										config.hasApiToken ? "•••••••••••••• (saved - leave blank to keep)" : "Permanent access token"
									}
								/>
							</div>
						</div>
					) : (
						<div className="auth-panel" key="bsp">
							<p className="text-body-small" style={{ color: "var(--text-tertiary)", marginBottom: 16 }}>
								For providers that resell Meta's Cloud API and accept the same message format — 360dialog and
								similar. Your provider's dashboard gives you the API base URL and a key.
							</p>

							{field("API Base URL", bspBaseUrl, setBspBaseUrl, {
								placeholder: "e.g. https://waba.360dialog.io/v1",
								name: "whatsapp-bsp-base-url",
							})}
							{field("Auth Header Name", bspAuthHeader, setBspAuthHeader, {
								placeholder: "e.g. D360-API-KEY — leave blank for Bearer",
								name: "whatsapp-bsp-auth-header",
							})}

							<div style={{ marginBottom: 14 }}>
								<label className="form-control-label pp fs-12" style={{ display: "block" }}>
									API Key
								</label>
								<PasswordInput
									inputClassName="form-control form-control-alternative nn"
									autoComplete="new-password"
									name="whatsapp-bsp-api-key"
									value={bspApiKey}
									onChange={(e) => setBspApiKey(e.target.value)}
									placeholder={
										config.hasBspApiKey ? "•••••••••••••• (saved - leave blank to keep)" : "Provider API key"
									}
								/>
							</div>

							{field("Phone Number ID", phoneNumberId, setPhoneNumberId, {
								placeholder: "the number registered with your provider",
								name: "whatsapp-bsp-phone-number-id",
							})}
						</div>
					)}

					<div
						className="text-body-small"
						style={{ color: config.configured ? "var(--xan-emerald)" : "var(--text-tertiary)", margin: "10px 0" }}
					>
						{config.configured ? `✓ Connected via ${config.provider === "bsp" ? "BSP" : "Meta Cloud API"}` : "Not connected yet"}
					</div>

					<button type="submit" className="shell-btn shell-btn-primary" disabled={saving}>
						{saving ? "Saving…" : "Save"}
					</button>
				</form>

			</div>
				</div>

				{/* Stacked under the connection card rather than spanning the page: the
				    Templates card beside it is much taller, so a full-width table below both
				    left an empty column here. This is also where it belongs by subject - the
				    company-wide "notify on create" default is set in Templates, and these are
				    the per-customer exceptions to it. */}
				<NotifyPreferences
					notifyOnCreate={notifyOnCreate}
					setNotifyOnCreate={setNotifyOnCreate}
					notifyOnUpdate={notifyOnUpdate}
					setNotifyOnUpdate={setNotifyOnUpdate}
				/>
			</div>

			{/* Templates sit in their own card beside the connection, not under it: choosing
			    which template each button sends is a different job from connecting an account,
			    and the picker is long enough to bury the Save button when stacked. */}
			<div className="col-12 col-lg-6">
				<div className="shell-card">
					<div className="shell-card-header">
						<span className="text-heading-brand">Templates</span>
					</div>
					<div style={{ padding: 20 }}>
						{/* Only once the credentials are saved: the list is read from Meta with
						    that token, so before then it could only show an error. */}
						{!config.configured ? (
							<p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0 }}>
								Connect an account first — the template list is read live from it.
							</p>
						) : (
							<>
								{templateSelect(
									"Job created",
									createdTemplate,
									setCreatedTemplate,
									"Sent by the “Notify created” button when a job-id is raised."
								)}
								{templateSelect(
									"Job updated",
									updatedTemplate,
									setUpdatedTemplate,
									"Sent by the same button when a job-id that was already announced changes."
								)}
								{templateSelect(
									"Job done",
									doneTemplate,
									setDoneTemplate,
									"Sent by the “Notify Ready to Pick” button once every card is Done."
								)}
								{templateSelect(
									"Purchase order sent",
									poCreatedTemplate,
									setPoCreatedTemplate,
									"Sent when an approved purchase order is shared with its supplier for the first time."
								)}
								{templateSelect(
									"Purchase order updated",
									poUpdateTemplate,
									setPoUpdateTemplate,
									"Sent instead when an order the supplier already has is shared again after a change."
								)}
								{templateSelect(
									"Dispatch confirmation",
									poConfirmTemplate,
									setPoConfirmTemplate,
									"Sent by “Confirm” to tell the supplier to dispatch the order."
								)}
								<p className="text-body-small" style={{ color: "var(--text-tertiary)", marginBottom: 12 }}>
									Saved with the connection form — press Save there after changing these.
								</p>
								<h3 className="text-heading-brand" style={{ marginBottom: 4 }}>
									Approved templates
								</h3>
								<p className="text-body-small" style={{ color: "var(--text-tertiary)", marginTop: 0, marginBottom: 12 }}>
									Read live from your WhatsApp Business Account. A template must be APPROVED before it can be
									sent, and the variables it lists are what a send has to supply.
								</p>
								<TemplatePicker
									selected={selectedTemplate}
									onSelect={(t) => setSelectedTemplate(t.name)}
									onLoaded={setApproved}
								/>
							</>
						)}
					</div>
				</div>
			</div>
		</div>
		</>
	);
};

export default WhatsAppManager;
