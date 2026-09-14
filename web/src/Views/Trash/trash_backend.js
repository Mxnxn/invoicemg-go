import axios from "axios";

const HEADER = {
	headers: {
		"SESSION-TOKEN": window.localStorage.getItem("session_token"),
	},
};
class TrashBackend {
	getAllEntry(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/trash/get`,
					formData,
					HEADER
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
	setPassword(formData) {
		return new Promise(async (resolve, reject) => {
			try {
				const res = await axios.post(
					`${import.meta.env.VITE_API_URL}/trash/setPassword`,
					formData,
					HEADER
				);
				if (res.data.code !== 200) throw res.data;
				resolve(res.data);
			} catch (error) {
				reject(error);
			}
		});
	}
}

export let trashBackend = new TrashBackend();
