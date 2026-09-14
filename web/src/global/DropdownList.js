import React from "react";

const DropdownList = (props) => {
    let clients = props.clients;
    let cid = props.cid;

    return (
        <select className={`form-control nn form-control-alternative `} value={cid} onChange={(e) => props.onChange(e.target.value)}>
            <option value="">Select One</option>

            {clients.map((client) => {
                return (
                    <option style={{ paddingLeft: "10px" }} value={client._id}>
                        {client.clientFirm}
                    </option>
                );
            })}
        </select>
    );
};

export default DropdownList;
