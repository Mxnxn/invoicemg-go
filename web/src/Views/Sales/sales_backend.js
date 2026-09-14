import axios from "axios";
const HEADER = {
	headers: {
		"SESSION-TOKEN": window.localStorage.getItem("session_token"),
	},
};
class Sales {
	getAllClients() {
		return new Promise(async (resolve, reject) => {
			try {
				// const res = await axios.get(import.meta.env.VITE_API_URL,{headers:{"SESSION-TOKEN":localStorage.get('session_token')}})
				const res = await axios.get(`${import.meta.env.VITE_API_URL}/stats/clients`, HEADER);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
	getData(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				// const res = await axios.get(import.meta.env.VITE_API_URL,{headers:{"SESSION-TOKEN":localStorage.get('session_token')}})
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/stats/get`, formData, {
					headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
				});
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}

export let sales = new Sales();
