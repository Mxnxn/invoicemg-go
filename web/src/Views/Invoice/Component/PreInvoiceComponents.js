import React from "react";
import { Row, Container, Col } from "reactstrap";
import Footer from "../../../Common/Footers/AdminFooter";
import ClientInvoicesPanel from "./ClientInvoicesPanel";

// The full-page view of one customer's invoices, at /invoice/:cid.
//
// The page chrome is all that lives here; the table itself is ClientInvoicesPanel, which was
// split out when the invoice list also showed this view. That list now filters in place
// instead (the Client picker above the table) and the modal has gone, leaving this as the only
// caller - but the split stays, because the panel is where the tests are.
//
// The route stays because it is a real address: the customer list links to it and it survives
// a refresh.
const PreInvoiceComponents = () => {
    // From the path rather than useParams: this component is cloned with props by
    // ProtectiveRoute rather than rendered as a route element, so the router's param hooks
    // see nothing here.
    const cid = window.location.pathname.split("/")[2];

    return (
        // data-shell, like AdminLayout. The Appearance preferences - font family, text scales,
        // accent, scoped resets - are applied under [data-shell] in tokens.css, so a page routed
        // outside ProtectiveRoute falls back to Argon's own body font.
        <div data-shell style={{ minHeight: "100vh" }}>
            <Container fluid style={{ paddingTop: "20px" }}>
                <Row>
                    <Col>
                        <div className="shell-card">
                            <div style={{ padding: 20 }}>
                                <ClientInvoicesPanel cid={cid} />
                            </div>
                        </div>
                    </Col>
                </Row>
            </Container>
            <Container fluid>
                <Footer darkModeFlag={"false"} />
            </Container>
        </div>
    );
};

export default PreInvoiceComponents;
