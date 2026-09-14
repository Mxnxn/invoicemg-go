import React from "react";

// reactstrap components
import { Container } from "reactstrap";

class LiteHeader extends React.Component {
	constructor(props) {
		super();
		this.props = props;
	}
	render() {
		return (
			<>
				<div
					className={
						this.props.bg === "primary"
							? "header"
							: "header"
					}
				>
					<Container fluid>
						<div className="header-body"></div>
					</Container>
				</div>
			</>
		);
	}
}

export default LiteHeader;
