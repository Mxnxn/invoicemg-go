import axios from "axios";
const HEADER = {
	headers: {
		"SESSION-TOKEN": window.localStorage.getItem("session_token"),
	},
};

class WastageBackend {
	getMaterials(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/wastage/materials`, formData, HEADER);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	addWastage(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/wastage/add`, formData, HEADER);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	getWastages(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/wastage/getall`, formData, HEADER);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}

	removeWastage(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(`${import.meta.env.VITE_API_URL}/wastage/remove`, formData, HEADER);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}

export let wastageBackend = new WastageBackend();
