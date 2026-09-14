import React, { useState, useEffect, useCallback } from "react";
import { Button, CardBody, FormGroup, Form, Input, Row, Col, Container } from "reactstrap";
import { sales } from "../../Sales/sales_backend";
import DropdownList from "../../../global/DropdownList";
import DataTable from "../../../Common/DataTable/DataTable";
import WastageForm from "./WastageForm";
import DateField from "../../../Common/DateField";

// Sales lookup moved here from the (removed) standalone Sales tab - same feature, now
// living beside Challan instead of its own nav item. Kept as one component (rather than
// split across the parent's Row/Col grid) so its state stays self-contained.
const ChallanIndex = () => {
    const [salesState, setSalesState] = useState({
        clients: [],
        fetched: {},
        fetchedData: false,
    });

    const [selected, setSelected] = useState({
        client_id: null,
        date: null,
        flag: "UNPAID",
    });

    const getClients = useCallback(async () => {
        try {
            const res = await sales.getAllClients();
            setSalesState((prev) => ({ ...prev, clients: res.data }));
        } catch (error) {
            console.log(error);
        }
    }, []);

    useEffect(() => {
        getClients();
    }, [getClients]);

    const onCustomerChange = (id) => {
        setSelected({ ...selected, client_id: id });
    };

    const onSubmitHandler = async () => {
        setSalesState((prev) => ({ ...prev, fetched: {}, fetchedData: false }));
        try {
            const formData = new FormData();
            formData.set("date", selected.date);
            formData.set("flag", selected.flag);
            if (selected.client_id) formData.set("cid", selected.client_id);
            const res = await sales.getData(formData);
            setSalesState((prev) => ({ ...prev, fetched: res.data, fetchedData: true }));
        } catch (error) {
            console.log(error);
        }
    };

    return (
        <>
            <Container fluid >
                {/* <Row>
                    <Col xl="4" className="mb-4 mb-xl-0">
                        <div className="shell-card">
                            <div className="shell-card-header">
                                <span className="text-heading-brand">Challan</span>
                            </div>
                            <CardBody>
                                <h6 className="heading-small text-muted mb-2 geb ">Challan information</h6>
                                <Form autoComplete="off">
                                    <Row>
                                        <Col lg="12">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Date</label>
                                                <Input className="form-control-alternative nn" name="date" placeholder="Product" type="date" />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <Row>
                                        <Col lg="12">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">From</label>
                                                <Input className="form-control-alternative nn" name="from" placeholder="Company" type="text" />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <Row>
                                        <Col lg="12">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Description</label>
                                                <Input className="form-control-alternative nn" name="description" placeholder="Rate" type="text" />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <Row>
                                        <Col lg="6">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Amount</label>
                                                <Input className="form-control-alternative nn" name="amount" placeholder="e.g 1000" type="number" />
                                            </FormGroup>
                                        </Col>
                                        <Col lg="6">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Quantity</label>
                                                <Input className="form-control-alternative nn" name="quantity" placeholder="1" type="number" />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <div className="text-left ">
                                        <Button className="shell-btn shell-btn-primary my-2">Add</Button>
                                    </div>
                                </Form>
                            </CardBody>
                            <div className="shell-card-footer">
                                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    Note: To copy all products from a client to new will replace existing one
                                </span>
                            </div>
                        </div>
                    </Col>

                    <Col xl="4" className="mb-4 mb-xl-0">
                        <div className="shell-card">
                            <div className="shell-card-header">
                                <span className="text-heading-brand">Sales</span>
                            </div>
                            <CardBody>
                                <h6 className="heading-small text-danger text-muted mb-2 geb ">Required</h6>
                                <Form autoComplete="off">
                                    <Row>
                                        <Col lg="12">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Date</label>
                                                <DateField name="material_name" value={selected.date} onChange={(evt) => {
                                                        setSelected({ ...selected, date: evt.target.value });
                                                    }} />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <Row>
                                        <Col lg="12">
                                            <div className="d-flex">
                                                <span
                                                    className={
                                                        selected.flag === "UNPAID" ? "text-primary fs-12 mt-1 mr-2" : "text-muted fs-12 mt-1 mr-2"
                                                    }
                                                >
                                                    Unpaid
                                                </span>
                                                <label className="custom-toggle">
                                                    <input
                                                        onClick={() => {
                                                            setSelected((prev) => ({ ...prev, flag: prev.flag === "UNPAID" ? "PAID" : "UNPAID" }));
                                                        }}
                                                        defaultChecked={selected.flag === "UNPAID" ? false : true}
                                                        type="checkbox"
                                                    />
                                                    <span className="custom-toggle-slider rounded-circle" />
                                                </label>
                                                <span
                                                    className={
                                                        selected.flag === "PAID" ? "text-primary fs-12 mt-1 ml-2" : "text-muted fs-12 mt-1 ml-2"
                                                    }
                                                >
                                                    Paid
                                                </span>
                                            </div>
                                        </Col>
                                    </Row>
                                    <hr />
                                    <h6 className="heading-small text-danger text-muted mb-2 geb ">Optional</h6>
                                    <Row>
                                        <Col lg="12">
                                            <FormGroup>
                                                <label className="form-control-label pp fs-12">Customers</label>
                                                <DropdownList
                                                    className="form-control-alternative"
                                                    clients={salesState.clients}
                                                    placeholder="Client-Names"
                                                    type="text"
                                                    cid={selected.client_id}
                                                    onChange={onCustomerChange}
                                                />
                                            </FormGroup>
                                        </Col>
                                    </Row>
                                    <div className="text-left ">
                                        <Button onClick={onSubmitHandler} className="shell-btn shell-btn-primary my-2">
                                            Get
                                        </Button>
                                    </div>
                                </Form>
                            </CardBody>
                            <div className="shell-card-footer">
                                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    Note: Selection of Client is optional. But, It'll fetch data for respective customer.
                                </span>
                            </div>
                        </div>
                    </Col>

                    <Col xl="4">
                        {salesState.fetchedData && (
                            <div className="shell-card">
                                <div className="shell-card-header">
                                    <span className="text-heading-brand">
                                        {salesState.fetched.customer ? `${salesState.fetched.customer.clientFirm}'s Sales` : "Sales"}
                                    </span>
                                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                        {salesState.fetched.requestedDate}
                                    </span>
                                </div>
                                <DataTable>
                                    <thead>
                                        <tr>
                                            <th scope="col">#</th>
                                            <th scope="col">Material</th>
                                            <th scope="col">Total Sq.ft</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {Object.keys(salesState.fetched.stats).map((el, index) => (
                                            <tr key={index}>
                                                <td className={el === "Total" ? "text-danger cell-mono" : "cell-mono"}>{index + 1}</td>
                                                <td className={el === "Total" ? "text-danger" : ""}>{el}</td>
                                                <td className={el === "Total" ? "text-danger cell-mono" : "cell-mono"}>
                                                    {salesState.fetched.stats[el]}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </DataTable>
                            </div>
                        )}
                    </Col>
                </Row> */}
                <WastageForm />
            </Container>
        </>
    );
};

export default ChallanIndex;
